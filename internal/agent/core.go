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
		MaxSteps: 20, // Karmaşık görevler için adım sayısı
		History:  []brain.Message{},
	}

	// 🧠 BAŞLANGIÇ: System Prompt'u bir kere yükle ve hafızada tut.
	// Böylece her mesajda tekrar tekrar yüklemeyiz.
	agent.resetMemoryWithSystemPrompt()

	return agent
}

// resetMemoryWithSystemPrompt: Hafızayı temizler ve Rick'in kişiliğini yükler.
func (a *Agent) resetMemoryWithSystemPrompt() {
	toolboxDesc := a.Registry.GetToolboxDescription()
	systemPrompt := brain.GetSystemPrompt(toolboxDesc)

	// Proje yapısını dinamik olarak her seferinde tazelemek istersek buraya ekleyebiliriz
	// Şimdilik temel prompt yeterli.
	
	a.History = []brain.Message{
		{Role: brain.RoleSystem, Content: systemPrompt},
	}
}

// Run: Artık hafızayı SİLMİYOR, üzerine ekliyor (Continuous Session).
func (a *Agent) Run(ctx context.Context, input string) (string, error) {
	logger.Info("🚀 Girdi: %s", input)

	// --- 1. HAFIZA YÖNETİMİ (Context Window) ---
	// Eğer hafıza çok şiştiyse eski mesajları buda ama System Prompt'u koru.
	a.manageContextWindow()

	// --- 2. RAG: UZUN SÜRELİ BELLEK SORGUSU ---
	// Sadece görev odaklı sorgularda hafızaya bakmak daha verimlidir.
	if a.Memory != nil && len(input) > 10 {
		embedding, err := a.Brain.Embed(ctx, input)
		if err == nil {
			matches, err := a.Memory.Search(ctx, embedding, 2) // Sadece en alakalı 2 tanesi
			if err == nil && len(matches) > 0 {
				contextMsg := "🧠 (Hatırlatma - Long Term Memory):\n"
				for _, m := range matches {
					contextMsg += fmt.Sprintf("- %s\n", m.Content)
				}
				// Hafızadan gelen bilgiyi System mesajı olarak değil, 
				// Görünmez bir 'System Note' olarak ekleyelim ki akışı bozmasın.
				a.History = append(a.History, brain.Message{Role: brain.RoleSystem, Content: contextMsg})
			}
		}
	}

	// --- 3. KULLANICI MESAJINI EKLE ---
	a.History = append(a.History, brain.Message{Role: brain.RoleUser, Content: input})

	// --- 4. RE-ACT DÖNGÜSÜ ---
	for i := 0; i < a.MaxSteps; i++ {
		a.State = StateThinking
		
		// Modelin düşünmesini sağla
		opts := &brain.GenerationOptions{Temperature: 0.1} // Düşük sıcaklık = Daha itaatkar
		toolDefs := a.Registry.GetToolDefinitions()

		logger.Debug("🔄 Adım %d/%d (Thinking...)", i+1, a.MaxSteps)
		
		resp, err := a.Brain.Chat(ctx, a.History, toolDefs, opts)
		if err != nil {
			return "", fmt.Errorf("beyin hatası: %v", err)
		}

		// Asistanın cevabını hafızaya ekle (Bağlam oluşuyor)
		a.History = append(a.History, resp.Message)

		// --- DURUM A: SOHBET VEYA DİREKT CEVAP ---
		// Eğer model hiç araç çağırmadıysa ve bir şeyler söylediyse, bu bir sohbettir.
		if len(resp.Message.ToolCalls) == 0 {
			if resp.Message.Content != "" {
				logger.Success("🗣️ Rick: %s", resp.Message.Content)
				return resp.Message.Content, nil
			}
			// Boş döndüyse (Bazen olabiliyor), uyar ve devam et
			a.History = append(a.History, brain.Message{Role: brain.RoleUser, Content: "Devam et..."})
			continue
		}

		// --- DURUM B: ARAÇ KULLANIMI (GÖREV MODU) ---
		a.State = StateExecuting
		
		for _, call := range resp.Message.ToolCalls {
			// Özel Araçlar: Sohbet ve Bitiş
			if call.ToolName == "conversational_reply" || call.ToolName == "present_answer" {
				finalResponse := a.handleFinalTool(call)
				
				// Başarılı bir görev sonucunu uzun süreli belleğe kaydet
				if call.ToolName == "present_answer" && a.Memory != nil {
					go a.saveToMemory(input, finalResponse)
				}
				
				logger.Success("🏁 Tamamlandı: %s", finalResponse)
				return finalResponse, nil
			}

			// Standart Araçları Çalıştır
			output := a.executeToolSafe(ctx, call)
			
			// Sonucu hafızaya ekle
			a.addToolResult(call.ID, call.ToolName, output)
		}
		
		// Döngü başa döner, Rick araç çıktılarına göre yeni karar verir.
	}

	return "Maksimum adım sayısına ulaşıldı, süreç durduruldu.", nil
}

// executeToolSafe: Hataya dayanıklı araç çalıştırıcı
func (a *Agent) executeToolSafe(ctx context.Context, call brain.ToolCall) string {
	logger.Action("🛠️ Çalışıyor: %s", call.ToolName)

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return fmt.Sprintf("Argüman hatası: %v", err)
	}

	output, err := a.Registry.Dispatch(ctx, call.ToolName, args)
	if err != nil {
		logger.Error("❌ Hata: %s -> %v", call.ToolName, err)
		return fmt.Sprintf("HATA: %v", err)
	}

	// Çıktı çok uzunsa konsolu kirletmesin diye kırpıp logla
	logOutput := output
	if len(logOutput) > 200 { logOutput = logOutput[:200] + "..." }
	logger.Success("📉 Sonuç: %s", logOutput)

	return output
}

// handleFinalTool: present_answer veya conversational_reply argümanlarını esnek okur
func (a *Agent) handleFinalTool(call brain.ToolCall) string {
	var args map[string]interface{}
	_ = json.Unmarshal([]byte(call.Arguments), &args)

	// Olası anahtarları kontrol et (Model bazen karıştırabilir)
	keys := []string{"answer", "reply", "content", "message", "result", "text"}
	for _, k := range keys {
		if val, ok := args[k].(string); ok && val != "" {
			return val
		}
	}
	
	// Liste döndüyse birleştir
	if list, ok := args["results"].([]interface{}); ok {
		return fmt.Sprintf("%v", list)
	}

	return "İşlem tamamlandı (Model boş yanıt döndü)."
}

func (a *Agent) addToolResult(toolCallID, toolName, content string) {
	// Hafıza yönetimi için çok büyük çıktıları kırp (Token limitini koru)
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
	// Eşik değer: 20 Mesaj
	if len(a.History) > 20 {
		// System Prompt (Index 0) her zaman korunmalı!
		systemMsg := a.History[0]
		
		// Son 10 mesajı al (Yakın geçmiş)
		recentMsgs := a.History[len(a.History)-10:]
		
		// Yeni hafızayı oluştur
		newHistory := []brain.Message{systemMsg}
		newHistory = append(newHistory, recentMsgs...)
		
		a.History = newHistory
		logger.Warn("🧹 Hafıza optimize edildi (Eski konuşmalar sıkıştırıldı).")
	}
}

// saveToMemory: Hafızaya asenkron kayıt yapar
func (a *Agent) saveToMemory(task, result string) {
	// Basit görevleri kaydetmeye gerek yok
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