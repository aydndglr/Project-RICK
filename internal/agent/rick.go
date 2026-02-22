package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/aydndglr/rick-agent-v3/internal/core/config"
	"github.com/aydndglr/rick-agent-v3/internal/core/kernel"
	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
	"github.com/aydndglr/rick-agent-v3/internal/skills"
)

// Session: Rick'in aynı anda çalıştırdığı her bir görevin izole beyni
type Session struct {
	ID        string
	History   []kernel.Message
	CreatedAt time.Time
	Cancel    context.CancelFunc // 🚀 GÖREVİ ÖLDÜRME SİNYALİ
	mu        sync.Mutex
}

type Rick struct {
	Config   *config.Config
	Brain    kernel.Brain
	Skills   *skills.Manager
	Memory   kernel.Memory
	MaxSteps int
	
	Sessions map[string]*Session
	sessMu   sync.RWMutex
}

// =====================================================================
// 🚀 YENİ ARAÇ: RICK KONTROL (Kendi Klonlarını Yönetmesi İçin)
// =====================================================================
type RickControlTool struct {
	rick *Rick
}

func (t *RickControlTool) Name() string { return "rick_control" }
func (t *RickControlTool) Description() string { 
	return "Rick'in arka planda çalışan aktif görevlerini (oturumlarını) yönetmesini sağlar. Hatalı, donmuş veya iptal edilmesi istenen bir 'TSK-...' görevini durdurmak (cancel) veya aktif listeyi görmek (list) için kullan." 
}
func (t *RickControlTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{"type": "string", "enum": []string{"list", "cancel"}},
			"session_id": map[string]interface{}{"type": "string", "description": "İptal edilecek görevin ID'si (Örn: TSK-1A2B). 'list' işlemi için boş bırakılabilir."},
		},
		"required": []string{"action"},
	}
}
func (t *RickControlTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)
	
	if action == "list" {
		t.rick.sessMu.RLock()
		defer t.rick.sessMu.RUnlock()
		if len(t.rick.Sessions) <= 1 {
			return "Şu an benden başka çalışan aktif bir görev (klon) yok.", nil 
		}
		var res strings.Builder
		res.WriteString("📋 Aktif Görevler:\n")
		for id, sess := range t.rick.Sessions {
			res.WriteString(fmt.Sprintf("- %s (Başlama: %s)\n", id, sess.CreatedAt.Format("15:04:05")))
		}
		return res.String(), nil
	}
	
	if action == "cancel" {
		sessID, _ := args["session_id"].(string)
		if sessID == "" { return "HATA: İptal edilecek session_id belirtilmedi.", nil }
		
		t.rick.sessMu.RLock()
		sess, exists := t.rick.Sessions[sessID]
		t.rick.sessMu.RUnlock()
		
		if !exists { 
			return fmt.Sprintf("HATA: '%s' ID'li görev bulunamadı. Zaten bitmiş veya iptal edilmiş olabilir.", sessID), nil 
		}
		if sess.Cancel != nil {
			sess.Cancel() // 🚀 Hedef göreve durma sinyalini yolla!
			return fmt.Sprintf("✅ BAŞARILI: [%s] görevine ölüm sinyali (Cancel) gönderildi. Görev durduruluyor.", sessID), nil
		}
	}
	return "Geçersiz eylem.", nil
}

func NewRick(cfg *config.Config, brain kernel.Brain, skillMgr *skills.Manager, mem kernel.Memory) *Rick {
	r := &Rick{
		Config:   cfg,
		Brain:    brain,
		Skills:   skillMgr,
		Memory:   mem,
		MaxSteps: 15,
		Sessions: make(map[string]*Session),
	}
	
	// 🚀 Rick'in kendi kendini öldürebilmesi için aracı beynine kaydediyoruz
	r.RegisterTool(&RickControlTool{rick: r})
	return r
}

func (a *Rick) RegisterTool(t kernel.Tool) {
	a.Skills.Register(t)
}

func (a *Rick) createSession(cancel context.CancelFunc) *Session {
	a.sessMu.Lock()
	defer a.sessMu.Unlock()

	sessID := fmt.Sprintf("TSK-%X", time.Now().UnixNano()%0xFFFFF)
	
	sess := &Session{
		ID:        sessID,
		History:   []kernel.Message{},
		CreatedAt: time.Now(),
		Cancel:    cancel, // Sinyal kablosunu oturuma bağla
	}
	
	a.Sessions[sessID] = sess
	return sess
}

func (a *Rick) Run(ctx context.Context, input string, images []string) (string, error) {
	// 🚀 Göreve özel iptal edilebilir (cancellable) context oluştur
	sessCtx, cancel := context.WithCancel(ctx)
	sess := a.createSession(cancel)
	defer cancel() // Fonksiyon bitince belleği sızdırmamak için kabloyu kopar
	
	logger.Info("👤 User [%s]: %s (Görsel: %d)", sess.ID, input, len(images))

	sess.mu.Lock()
	sess.History = append(sess.History, kernel.Message{
		Role:    "user",
		Content: input,
		Images:  images,
	})
	sess.mu.Unlock()

	a.refreshSystemPrompt(sess)

	for i := 0; i < a.MaxSteps; i++ {
		// 🛑 İPTAL KONTROLÜ: Döngü başında görevin dışarıdan vurulup vurulmadığına bak
		select {
		case <-sessCtx.Done():
			a.sessMu.Lock()
			delete(a.Sessions, sess.ID)
			a.sessMu.Unlock()
			logger.Warn("🛑 [%s] Görev dışarıdan bir klon tarafından vuruldu (İptal).", sess.ID)
			return fmt.Sprintf("🛑 [%s] İşlem iptal edildi / durduruldu.", sess.ID), nil
		default:
		}

		a.manageContextWindow(sess)

		tools := a.Skills.ListTools()
		
		sess.mu.Lock()
		currentHistory := make([]kernel.Message, len(sess.History))
		copy(currentHistory, sess.History)
		sess.mu.Unlock()

		// Beyne düşünmesi için sinyal kablosunu (sessCtx) ver
		resp, err := a.Brain.Chat(sessCtx, currentHistory, tools)
		if err != nil {
			if sessCtx.Err() != nil {
				return fmt.Sprintf("🛑 [%s] Beyin düşünürken işlem yarıda kesildi.", sess.ID), nil
			}
			return "", err
		}

		if len(resp.ToolCalls) == 0 {
			jsonStr := a.extractJSON(resp.Content)
			if jsonStr != "" {
				var rawCall map[string]interface{}
				if err := json.Unmarshal([]byte(jsonStr), &rawCall); err == nil {
					funcName, ok := rawCall["function"].(string)
					if !ok {
						funcName, _ = rawCall["name"].(string)
					}

					if funcName != "" {
						args, _ := rawCall["arguments"].(map[string]interface{})
						if args == nil {
							args, _ = rawCall["parameters"].(map[string]interface{})
						}
						if args == nil {
							args = make(map[string]interface{})
						}

						resp.ToolCalls = append(resp.ToolCalls, kernel.ToolCall{
							ID:        fmt.Sprintf("call_%d", i),
							Function:  funcName,
							Arguments: args,
						})
						resp.Content = "" 
					}
				}
			}
		}

		msg := kernel.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		}
		
		sess.mu.Lock()
		sess.History = append(sess.History, msg)
		sess.mu.Unlock()

		if len(resp.ToolCalls) == 0 {
			if resp.Content != "" {
				logger.Success("🤖 Rick [%s]: İşlem Tamamlandı.", sess.ID)
				
				go a.Memory.Add(context.Background(), fmt.Sprintf("User: %s | Rick: %s", input, resp.Content), nil)
				
				a.sessMu.Lock()
				delete(a.Sessions, sess.ID)
				a.sessMu.Unlock()
				
				return fmt.Sprintf("🎯 [%s]\n%s", sess.ID, resp.Content), nil
			}
			
			sess.mu.Lock()
			sess.History = append(sess.History, kernel.Message{Role: "user", Content: "Devam et."})
			sess.mu.Unlock()
			continue
		}

		for _, call := range resp.ToolCalls {
			if call.Function == "" { continue }

			logger.Action("🛠️ [%s] Çalıştırılıyor: %s", sess.ID, call.Function)
			
			// 🛡️ Araç çalışırken de iptal kablosunu (sessCtx) içeri yolluyoruz
			toolOutput, err := a.executeToolSafe(sessCtx, call)

			// Araç çalışırken iptal sinyali gelmişse, sonucu boşver ve çık.
			if sessCtx.Err() != nil {
				return fmt.Sprintf("🛑 [%s] İşlem araç çalıştırılırken iptal edildi.", sess.ID), nil
			}

			if err != nil {
				logger.Warn("⚠️ [%s] Araç Hatası: %v", sess.ID, err)
				toolOutput = fmt.Sprintf("❌ ÇALIŞTIRMA HATASI: %v\nLütfen hatayı analiz et ve gerekiyorsa düzelt.", err)
			}

			sess.mu.Lock()
			sess.History = append(sess.History, kernel.Message{
				Role:       "tool",
				Content:    toolOutput,
				Name:       call.Function,
				ToolCallID: call.ID,
			})
			sess.mu.Unlock()
		}
	}
	
	a.sessMu.Lock()
	delete(a.Sessions, sess.ID)
	a.sessMu.Unlock()

	return fmt.Sprintf("🛑 [%s] Döngü sınırı aşıldı patron. İşlem çok uzadı.", sess.ID), nil
}

func (a *Rick) extractJSON(content string) string {
	re := regexp.MustCompile("(?s)```(?:json)?\n?(.*?)\n?```")
	match := re.FindStringSubmatch(content)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start != -1 && end != -1 && end > start {
		return content[start : end+1]
	}
	return ""
}

func (a *Rick) executeToolSafe(ctx context.Context, call kernel.ToolCall) (string, error) {
	tool, err := a.Skills.GetTool(call.Function)
	if err != nil {
		return "", fmt.Errorf("'%s' adında bir araç sistemde kayıtlı değil", call.Function)
	}
	return tool.Execute(ctx, call.Arguments)
}

func (a *Rick) refreshSystemPrompt(sess *Session) {
	osContext := fmt.Sprintf("%s (OS: %s, ARCH: %s)", a.Config.App.WorkDir, runtime.GOOS, runtime.GOARCH)
	sysMsg := BuildSystemPrompt(a.Config.App.ActivePrompt, osContext, a.Config.Security.Level, a.Skills.ListTools())

	sess.mu.Lock()
	defer sess.mu.Unlock()

	if len(sess.History) == 0 {
		sess.History = append([]kernel.Message{sysMsg}, sess.History...)
	} else if sess.History[0].Role == "system" {
		sess.History[0] = sysMsg
	} else {
		sess.History = append([]kernel.Message{sysMsg}, sess.History...)
	}
}

func (a *Rick) manageContextWindow(sess *Session) {
	maxContextSize := 20

	sess.mu.Lock()
	defer sess.mu.Unlock()

	if len(sess.History) > maxContextSize {
		systemMsg := sess.History[0]
		recentMsgs := sess.History[len(sess.History)-(maxContextSize-1):]

		newHistory := []kernel.Message{systemMsg}
		newHistory = append(newHistory, recentMsgs...)

		sess.History = newHistory
		logger.Warn("🧹 [%s] Hafıza optimize edildi (Sliding Window).", sess.ID)
	}
}