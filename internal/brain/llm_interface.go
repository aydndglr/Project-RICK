package brain

import (
	"context"
)

// MessageRole: Mesajı kimin gönderdiğini belirten enum (system, user, assistant, tool)
type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

// Message: LLM ile yapılan sohbetin en küçük yapı taşı.
// Artık string birleştirme yerine bu yapıları liste olarak tutacağız.
type Message struct {
	Role       MessageRole `json:"role"`
	Content    string      `json:"content"`
	Name       string      `json:"name,omitempty"`       // Tool çıktısı ise hangi tool olduğu
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"` // Assistant tool çağırdıysa buraya eklenir
	ToolCallID string      `json:"tool_call_id,omitempty"` // Tool yanıtı ise hangi çağrıya ait olduğu
}

// ToolDefinition: Modelin anlayacağı araç açıklaması
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}

// ToolCall: Modelden gelen araç çalıştırma isteği
type ToolCall struct {
	ID        string `json:"id"`
	ToolName  string `json:"function"` // OpenAI/Ollama genelde "function" veya "name" kullanır
	Arguments string `json:"arguments"` // JSON string
}

// TokenUsage: Harcanan kaynak bilgisi (Observability için kritik)
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// BrainResponse: Modelden dönen zenginleştirilmiş yanıt
type BrainResponse struct {
	Message      Message    `json:"message"` // İçeriği ve Rolü taşır
	Usage        TokenUsage `json:"usage"`   // Token kullanımı
	FinishReason string     `json:"finish_reason"` // "stop", "length", "tool_calls"
}

// GenerationOptions: İstek bazlı ayarlar (Temperature, MaxTokens vb.)
type GenerationOptions struct {
	Temperature float64
	MaxTokens   int
	StopWords   []string
	JsonMode    bool // Structured Output (JSON) zorlama
}

// LLMProvider: Rick Agent V2'nin (Enterprise) zeka arayüzü
type LLMProvider interface {
	// Chat: Sohbet geçmişi (history) ve araçlarla birlikte modele gider.
	Chat(ctx context.Context, history []Message, tools []ToolDefinition, opts *GenerationOptions) (*BrainResponse, error)

	// StreamChat: Yanıtı parça parça (token) almamızı sağlar.
	StreamChat(ctx context.Context, history []Message, tools []ToolDefinition, opts *GenerationOptions, onToken func(string)) (*BrainResponse, error)

	// Embed: Metni vektöre çevirir (RAG / Hafıza için gerekli).
	Embed(ctx context.Context, text string) ([]float32, error)

	// CountTokens: Verilen metnin kaç token tuttuğunu hesaplar (Context yönetimi için).
	CountTokens(text string) int
}