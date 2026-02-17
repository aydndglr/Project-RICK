package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aydndglr/rick-agent/internal/brain"
	"github.com/aydndglr/rick-agent/internal/config"
	"github.com/aydndglr/rick-agent/internal/memory"
	"github.com/aydndglr/rick-agent/internal/tools"
	"github.com/aydndglr/rick-agent/pkg/logger"
)

type AgentState string

const (
	StateThinking  AgentState = "THINKING"
	StateExecuting AgentState = "EXECUTING"
	StateFinished  AgentState = "FINISHED"
)

type Agent struct {
	Config   *config.Config
	Brain    brain.LLMProvider
	Registry *tools.Registry
	Memory   memory.VectorStore
	State    AgentState
	MaxSteps int
	History  []brain.Message
}

// NewAgent: Ajanı başlatır ve İLK HAFIZAYI (System Prompt) yükler.
func NewAgent(cfg *config.Config, b brain.LLMProvider, reg *tools.Registry, mem memory.VectorStore) *Agent {
	agent := &Agent{
		Config:   cfg,
		Brain:    b,
		Registry: reg,
		Memory:   mem,
		State:    StateThinking,
		MaxSteps: 20, // Karmaşık görevler için maksimum döngü sayısı
		History:  []brain.Message{},
	}

	// 🧠 BAŞLANGIÇ: System Prompt'u yükle
	agent.resetMemoryWithSystemPrompt()

	return agent
}

// resetMemoryWithSystemPrompt: Hafızayı temizler ve Rick'in kişiliğini yükler.
//func (a *Agent) resetMemoryWithSystemPrompt() {
//	toolboxDesc := a.Registry.GetToolboxDescription()
//	systemPrompt := brain.GetSystemPrompt(toolboxDesc)
//	
//	a.History = []brain.Message{
//		{Role: brain.RoleSystem, Content: systemPrompt},
//	}
//}

// refreshSystemPrompt: Mevcut hafızadaki sistem mesajını (History[0]) güncel araç listesiyle tazeler.
func (a *Agent) refreshSystemPrompt() {
	toolboxDesc := a.Registry.GetToolboxDescription()
	systemPrompt := brain.GetSystemPrompt(toolboxDesc)
	
	if len(a.History) > 0 && a.History[0].Role == brain.RoleSystem {
		a.History[0].Content = systemPrompt
	} else {
		// Eğer History boşsa veya ilk mesaj sistem değilse (güvenlik için)
		a.History = append([]brain.Message{{Role: brain.RoleSystem, Content: systemPrompt}}, a.History...)
	}
}

// resetMemoryWithSystemPrompt: (Mevcut olanı bununla değiştirebilirsin, daha temiz olur)
func (a *Agent) resetMemoryWithSystemPrompt() {
	a.History = []brain.Message{} // Önce temizle
	a.refreshSystemPrompt()      // Sonra güncel haliyle yükle
}

// Run: (Legacy) Tek seferlik çıktı döner. Artık RunStream kullanılması önerilir.
func (a *Agent) Run(ctx context.Context, input string) (string, error) {
	// Stream kanalını dinleyip son cevabı birleştirip döner (Geriye uyumluluk)
	streamChan := a.RunStream(ctx, input)
	var finalOutput string
	for msg := range streamChan {
		finalOutput = msg // Son mesajı al
	}
	return finalOutput, nil
}

// RunStream: (v4.0) Cevapları parça parça (chunk) olarak kanal üzerinden gönderir.
// Bu sayede WhatsApp gibi arayüzler "işlem devam ediyor..." mesajlarını anlık gösterebilir.
func (a *Agent) RunStream(ctx context.Context, input string) <-chan string {
	outChan := make(chan string)

	go func() {
		defer close(outChan) // İşlem bitince kanalı kapat

		logger.Info("🚀 Girdi (Stream): %s", input)

		a.refreshSystemPrompt()
		// 1. HAFIZA YÖNETİMİ
		a.manageContextWindow()

		// 2. RAG: UZUN SÜRELİ BELLEK SORGUSU
		if a.Memory != nil && len(input) > 10 {
			embedding, err := a.Brain.Embed(ctx, input)
			if err == nil {
				matches, err := a.Memory.Search(ctx, embedding, 2)
				if err == nil && len(matches) > 0 {
					contextMsg := "🧠 (Hatırlatma - Long Term Memory):\n"
					for _, m := range matches {
						contextMsg += fmt.Sprintf("- %s\n", m.Content)
					}
					a.History = append(a.History, brain.Message{Role: brain.RoleSystem, Content: contextMsg})
				}
			}
		}

		// 3. KULLANICI MESAJINI EKLE
		a.History = append(a.History, brain.Message{Role: brain.RoleUser, Content: input})

		// 4. RE-ACT DÖNGÜSÜ
		for i := 0; i < a.MaxSteps; i++ {
			a.State = StateThinking
			
			// Modelin düşünmesini sağla
			opts := &brain.GenerationOptions{Temperature: 0.1}
			toolDefs := a.Registry.GetToolDefinitions()

			logger.Debug("🔄 Adım %d/%d (Thinking...)", i+1, a.MaxSteps)
			
			// Beyne sor
			resp, err := a.Brain.Chat(ctx, a.History, toolDefs, opts)
			if err != nil {
				outChan <- fmt.Sprintf("💥 Beyin hatası: %v", err)
				return
			}

			// Asistanın cevabını hafızaya ekle
			a.History = append(a.History, resp.Message)

			// --- DURUM A: SOHBET (Araç yok) ---
			if len(resp.Message.ToolCalls) == 0 {
				if resp.Message.Content != "" {
					logger.Success("🗣️ Rick: %s", resp.Message.Content)
					outChan <- resp.Message.Content // Kullanıcıya ilet
					return
				}
				// Boş döndüyse (Bazen olabiliyor), devam et
				a.History = append(a.History, brain.Message{Role: brain.RoleUser, Content: "Devam et..."})
				continue
			}

			// --- DURUM B: ARAÇ KULLANIMI (GÖREV MODU) ---
			a.State = StateExecuting
			
			for _, call := range resp.Message.ToolCalls {
				// Özel Araçlar: Sohbet ve Bitiş
				if call.ToolName == "conversational_reply" || call.ToolName == "present_answer" {
					finalResponse := a.handleFinalTool(call)
					
					// Hafızaya kaydet
					if call.ToolName == "present_answer" && a.Memory != nil {
						go a.saveToMemory(input, finalResponse)
					}
					
					logger.Success("🏁 Tamamlandı: %s", finalResponse)
					outChan <- finalResponse
					return
				}

				// ARA BİLDİRİM: Kullanıcıya ne yaptığımızı söyleyelim
				// "Arka planda virüs taraması başlatıyorum..." gibi.
				logger.Action("🛠️ Çalışıyor: %s", call.ToolName)
				// NOT: Her araç için mesaj atmak kullanıcıyı boğabilir. 
				// Sadece uzun süren "start_task" gibi işlemlerde bilgi vermek daha iyi olabilir.
				// Ancak şimdilik Rick'in canlı olduğunu hissettirmek için kısa log gönderiyoruz.
				// outChan <- fmt.Sprintf("⚙️ İşlem yapılıyor: %s...", call.ToolName)

				// Standart Araçları Çalıştır
				output := a.executeToolSafe(ctx, call)
				
				// Sonucu hafızaya ekle
				a.addToolResult(call.ID, call.ToolName, output)

				// Eğer bu bir 'start_task' ise, kullanıcıya ID bilgisini hemen dönelim
				if call.ToolName == "start_task" {
					outChan <- output // "Görev başlatıldı ID: task_123" bilgisini hemen gönder
					// Döngüyü kırma, belki başka bir şey daha söyleyecektir.
				}
			}
			
			// Döngü başa döner, Rick araç çıktılarına göre yeni karar verir.
		}

		outChan <- "⚠️ Maksimum adım sayısına ulaşıldı, süreç durduruldu."
	}()

	return outChan
}

// executeToolSafe: Hataya dayanıklı araç çalıştırıcı
func (a *Agent) executeToolSafe(ctx context.Context, call brain.ToolCall) string {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return fmt.Sprintf("Argüman hatası: %v", err)
	}

	output, err := a.Registry.Dispatch(ctx, call.ToolName, args)
	if err != nil {
		logger.Error("❌ Hata: %s -> %v", call.ToolName, err)
		return fmt.Sprintf("HATA: %v", err)
	}

	// Logları temiz tut
	logOutput := output
	if len(logOutput) > 200 { logOutput = logOutput[:200] + "..." }
	logger.Success("📉 Sonuç: %s", logOutput)

	return output
}

// handleFinalTool: present_answer veya conversational_reply argümanlarını esnek okur
func (a *Agent) handleFinalTool(call brain.ToolCall) string {
	var args map[string]interface{}
	_ = json.Unmarshal([]byte(call.Arguments), &args)

	keys := []string{"answer", "reply", "content", "message", "result", "text"}
	for _, k := range keys {
		if val, ok := args[k].(string); ok && val != "" {
			return val
		}
	}
	
	if list, ok := args["results"].([]interface{}); ok {
		return fmt.Sprintf("%v", list)
	}

	return "İşlem tamamlandı."
}

func (a *Agent) addToolResult(toolCallID, toolName, content string) {
	if len(content) > 3000 {
		content = content[:3000] + "\n...[Çıktı Kırpıldı]..."
	}

	a.History = append(a.History, brain.Message{
		Role:       brain.RoleTool,
		Content:    content,
		Name:       toolName,
		ToolCallID: toolCallID,
	})
}

// manageContextWindow: Hafızayı kayan pencere (Sliding Window) yöntemiyle yönetir.
func (a *Agent) manageContextWindow() {
	if len(a.History) > 20 {
		systemMsg := a.History[0]
		recentMsgs := a.History[len(a.History)-10:]
		
		newHistory := []brain.Message{systemMsg}
		newHistory = append(newHistory, recentMsgs...)
		
		a.History = newHistory
		logger.Warn("🧹 Hafıza optimize edildi.")
	}
}

// saveToMemory: Hafızaya asenkron kayıt yapar
func (a *Agent) saveToMemory(task, result string) {
	if len(result) < 10 { return }

	saveCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	content := fmt.Sprintf("Görev: %s | Sonuç: %s", task, result)
	vec, err := a.Brain.Embed(saveCtx, content)
	if err == nil {
		doc := memory.Document{
			Content:   content,
			Embedding: vec,
			Metadata: map[string]interface{}{
				"timestamp": time.Now().Unix(),
				"type":      "archive",
			},
		}
		_ = a.Memory.Add(saveCtx, doc)
	}
}