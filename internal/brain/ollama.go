package brain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type OllamaProvider struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

func NewOllamaProvider(url, model string) *OllamaProvider {
	return &OllamaProvider{
		BaseURL: url,
		Model:   model,
		Client:  &http.Client{Timeout: 300 * time.Second},
	}
}

// ollamaRequest: /api/chat endpoint'i için istek yapısı
type ollamaRequest struct {
	Model    string           `json:"model"`
	Messages []ollamaMessage  `json:"messages"`
	Stream   bool             `json:"stream"`
	Tools    []ToolDefinition `json:"tools,omitempty"` 
	Options  map[string]interface{} `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	Function struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	} `json:"function"`
}

type ollamaResponse struct {
	Model           string        `json:"model"`
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	PromptEvalCount int           `json:"prompt_eval_count"`
	EvalCount       int           `json:"eval_count"`
}

// Chat: Native Tool desteği olan veya olmayan modeller için hibrit çözüm sunar.
func (o *OllamaProvider) Chat(ctx context.Context, history []Message, tools []ToolDefinition, opts *GenerationOptions) (*BrainResponse, error) {
	var messages []ollamaMessage
	for _, msg := range history {
		messages = append(messages, ollamaMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	reqBody := ollamaRequest{
		Model:    o.Model,
		Messages: messages,
		Stream:   false,
		Options: map[string]interface{}{
			"temperature": 0.2,
		},
	}

	// KRİTİK: Eva veya GGUF modelleri için native tools parametresini hiç gönderme.
	isLegacyModel := strings.Contains(strings.ToLower(o.Model), "eva") || strings.Contains(strings.ToLower(o.Model), "gguf")
	if !isLegacyModel && len(tools) > 0 {
		reqBody.Tools = tools
	}

	if opts != nil {
		if opts.Temperature > 0 {
			reqBody.Options["temperature"] = opts.Temperature
		}
		if opts.MaxTokens > 0 {
			reqBody.Options["num_predict"] = opts.MaxTokens
		}
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("istek paketlenemedi: %v", err)
	}

	req, _ := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/api/chat", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		
		// Auto-Fallback: Eğer native tools yüzünden 400 verirse, tools'suz tekrar dene.
		if resp.StatusCode == http.StatusBadRequest && !isLegacyModel {
			return o.Chat(ctx, history, nil, opts)
		}
		return nil, fmt.Errorf("ollama api hatası (%d): %v", resp.StatusCode, errResp["error"])
	}

	var result ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	finalResp := &BrainResponse{
		Message: Message{
			Role:    RoleAssistant,
			Content: result.Message.Content,
		},
		Usage: TokenUsage{
			PromptTokens:     result.PromptEvalCount,
			CompletionTokens: result.EvalCount,
			TotalTokens:      result.PromptEvalCount + result.EvalCount,
		},
	}

	// 1. Durum: Model Native Tool çağırdıysa
	if len(result.Message.ToolCalls) > 0 {
		finalResp.FinishReason = "tool_calls"
		for _, tc := range result.Message.ToolCalls {
			argBytes, _ := json.Marshal(tc.Function.Arguments)
			finalResp.Message.ToolCalls = append(finalResp.Message.ToolCalls, ToolCall{
				ToolName:  tc.Function.Name,
				Arguments: string(argBytes),
				ID:        fmt.Sprintf("call_%d", time.Now().UnixNano()),
			})
		}
		return finalResp, nil
	}

	// 2. Durum: Eva gibi modeller JSON'u Content içine bastıysa (Manuel Ayıklama)
	if manualCalls := o.manualExtractToolCalls(result.Message.Content); len(manualCalls) > 0 {
		finalResp.Message.ToolCalls = manualCalls
		finalResp.FinishReason = "tool_calls"
	} else {
		finalResp.FinishReason = "stop"
	}

	return finalResp, nil
}

// manualExtractToolCalls: Metin içerisinden JSON formatındaki tool çağrılarını ayıklar.
func (o *OllamaProvider) manualExtractToolCalls(content string) []ToolCall {
	cleanJSON := cleanJSONString(content)
	if cleanJSON == "" {
		return nil
	}

	// Modelin üretebileceği muhtemel JSON yapılarını dene
	var raw struct {
		ToolCalls []struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		} `json:"tool_calls"`
	}

	if err := json.Unmarshal([]byte(cleanJSON), &raw); err == nil && len(raw.ToolCalls) > 0 {
		var calls []ToolCall
		for _, rc := range raw.ToolCalls {
			argBytes, _ := json.Marshal(rc.Arguments)
			calls = append(calls, ToolCall{
				ToolName:  rc.Name,
				Arguments: string(argBytes),
				ID:        fmt.Sprintf("manual_%d", time.Now().UnixNano()),
			})
		}
		return calls
	}
	return nil
}

func cleanJSONString(input string) string {
	input = strings.TrimSpace(input)
	// Markdown bloklarını temizle
	if strings.Contains(input, "```json") {
		parts := strings.Split(input, "```json")
		if len(parts) > 1 {
			input = parts[1]
		}
		parts = strings.Split(input, "```")
		if len(parts) > 0 {
			input = parts[0]
		}
	} else if strings.Contains(input, "```") {
		parts := strings.Split(input, "```")
		if len(parts) > 1 {
			input = parts[1]
		}
	}
	return strings.TrimSpace(input)
}

func (o *OllamaProvider) StreamChat(ctx context.Context, history []Message, tools []ToolDefinition, opts *GenerationOptions, onToken func(string)) (*BrainResponse, error) {
	return o.Chat(ctx, history, tools, opts)
}

func (o *OllamaProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]interface{}{"model": o.Model, "prompt": text}
	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/api/embeddings", bytes.NewBuffer(jsonData))
	resp, err := o.Client.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	var result struct { Embedding []float32 `json:"embedding"` }
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Embedding, nil
}

func (o *OllamaProvider) CountTokens(text string) int {
	return len(text) / 4
}

func (o *OllamaProvider) HealthCheck() bool {
	resp, err := o.Client.Get(o.BaseURL)
	return err == nil && resp.StatusCode == http.StatusOK
}