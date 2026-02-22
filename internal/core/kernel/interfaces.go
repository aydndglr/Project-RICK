package kernel

import (
	"context"
)

// Tool: Rick'in kullanabileceği her yetenek bu arayüzü uygulamalıdır.
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]interface{} // JSON Schema
	Execute(ctx context.Context, args map[string]interface{}) (string, error)
}

// ToolCall: LLM'in araç çağırma isteği
type ToolCall struct {
	ID        string                 `json:"id"`
	Function  string                 `json:"function"`  // LLM genelde 'function' veya 'name' gönderir
	Arguments map[string]interface{} `json:"arguments"`
}

// BrainResponse: Beyinden gelen ham cevap
type BrainResponse struct {
	Content   string
	ToolCalls []ToolCall
	Usage     map[string]int // Token kullanımı
}

// Brain: Zeka sağlayıcısı (Ollama, OpenAI, Gemini vb.)
type Brain interface {
	// Chat: Sohbet geçmişi ve araçlarla birlikte modele gider
	Chat(ctx context.Context, history []Message, tools []Tool) (*BrainResponse, error)
	// Embed: Metni vektöre çevirir (Hafıza için)
	Embed(ctx context.Context, text string) ([]float32, error)
}

// Message: Sohbet geçmişi birimi
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	Images     []string   `json:"images,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}


// Memory: Uzun süreli hafıza
type Memory interface {
	Add(ctx context.Context, content string, metadata map[string]interface{}) error
	Search(ctx context.Context, query string, limit int) ([]string, error)
}

// Agent: Rick'in kendisi
type Agent interface {
	// YENİ: Görselleri alabilmesi için images parametresi eklendi
	Run(ctx context.Context, input string, images []string) (string, error)
	RegisterTool(t Tool)
}