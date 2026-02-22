package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aydndglr/rick-agent-v3/internal/core/kernel"
)

type OllamaProvider struct {
	BaseURL     string
	Model       string
	Temperature float64      // 🚀 YENİ: Config'den gelecek sıcaklık
	NumCtx      int          // 🚀 YENİ: Config'den gelecek token limiti
	Client      *http.Client
}

// 🚀 DÜZELTME: Fonksiyona temp ve numCtx parametrelerini ekledik
func NewOllama(url, model string, temp float64, numCtx int) *OllamaProvider {
	// Eğer yaml'da unutulmuşsa diye güvenli bir varsayılan atayalım
	if numCtx == 0 {
		numCtx = 8192
	}
	
	return &OllamaProvider{
		BaseURL:     url,
		Model:       model,
		Temperature: temp,
		NumCtx:      numCtx,
		Client:      &http.Client{Timeout: 300 * time.Second},
	}
}

// -- Ollama Spesifik Araç Şeması --
type ollamaTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Parameters  map[string]interface{} `json:"parameters"`
	} `json:"function"`
}

// -- İstek Yapıları --
type ollamaRequest struct {
	Model    string                 `json:"model"`
	Messages []ollamaMessage        `json:"messages"`
	Stream   bool                   `json:"stream"`
	Tools    []ollamaTool           `json:"tools,omitempty"` 
	Options  map[string]interface{} `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Images    []string         `json:"images,omitempty"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	Function struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	} `json:"function"`
}

type ollamaResponse struct {
	Message   ollamaMessage `json:"message"`
	EvalCount int           `json:"eval_count"`
}

// Chat: LLM ile konuşur
func (o *OllamaProvider) Chat(ctx context.Context, history []kernel.Message, tools []kernel.Tool) (*kernel.BrainResponse, error) {
	// 1. Mesajları dönüştür
	var messages []ollamaMessage
	for _, msg := range history {
		om := ollamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
			Images:  msg.Images,
		}
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				om.ToolCalls = append(om.ToolCalls, ollamaToolCall{
					Function: struct {
						Name      string                 `json:"name"`
						Arguments map[string]interface{} `json:"arguments"`
					}{Name: tc.Function, Arguments: tc.Arguments},
				})
			}
		}
		messages = append(messages, om)
	}

	// 2. İsteği hazırla
	reqBody := ollamaRequest{
		Model:    o.Model,
		Messages: messages,
		Stream:   false,
		Options: map[string]interface{}{
			"temperature": o.Temperature, // 🚀 YENİ: Config'den dinamik alıyoruz
			"num_ctx":     o.NumCtx,      // 🚀 YENİ: Config'den hafıza limitini enjekte ettik!
		},
	}

	for _, t := range tools {
		ot := ollamaTool{Type: "function"}
		ot.Function.Name = t.Name()
		ot.Function.Description = t.Description()
		ot.Function.Parameters = t.Parameters()
		reqBody.Tools = append(reqBody.Tools, ot)
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/api/chat", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	// 3. Gönder
	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama hatası: %d", resp.StatusCode)
	}

	// 4. Cevabı işle
	var result ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	brainResp := &kernel.BrainResponse{
		Content: result.Message.Content,
		Usage:   map[string]int{"completion_tokens": result.EvalCount},
	}

	for _, tc := range result.Message.ToolCalls {
		brainResp.ToolCalls = append(brainResp.ToolCalls, kernel.ToolCall{
			Function:  tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	return brainResp, nil
}

// Embed: Metni vektöre çevirir
func (o *OllamaProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"model":  o.Model,
		"prompt": text,
	}
	jsonData, _ := json.Marshal(reqBody)
	
	req, _ := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/api/embeddings", bytes.NewBuffer(jsonData))
	resp, err := o.Client.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()

	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Embedding, nil
}