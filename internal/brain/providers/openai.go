package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aydndglr/rick-agent-v3/internal/core/kernel"
)

// OpenAIProvider: OpenAI uyumlu tüm API'ler (GPT-4, DeepSeek, LocalAI) için istemci.
type OpenAIProvider struct {
	BaseURL string
	APIKey  string
	Model   string
	Client  *http.Client
}

func NewOpenAI(url, key, model string) *OpenAIProvider {
	return &OpenAIProvider{
		BaseURL: strings.TrimSuffix(url, "/"),
		APIKey:  key,
		Model:   model,
		Client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (o *OpenAIProvider) Chat(ctx context.Context, history []kernel.Message, tools []kernel.Tool) (*kernel.BrainResponse, error) {
	type msg struct {
		Role    string      `json:"role"`
		Content interface{} `json:"content"` // String veya Multi-modal dizi
	}
	type payload struct {
		Model    string `json:"model"`
		Messages []msg  `json:"messages"`
	}

	var messages []msg
	for _, h := range history {
		var content interface{}

		// Görsel varsa Multi-modal yapı kur
		if len(h.Images) > 0 {
			var parts []interface{}
			parts = append(parts, map[string]string{"type": "text", "text": h.Content})

			for _, img := range h.Images {
				imgUrl := img
				if !strings.HasPrefix(img, "data:image") {
					imgUrl = "data:image/jpeg;base64," + img
				}
				parts = append(parts, map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]string{"url": imgUrl},
				})
			}
			content = parts
		} else {
			// Sadece metin
			content = h.Content
		}

		messages = append(messages, msg{Role: h.Role, Content: content})
	}

	reqBody := payload{Model: o.Model, Messages: messages}
	jsonData, _ := json.Marshal(reqBody)

	req, _ := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/v1/chat/completions", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	if o.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.APIKey)
	}

	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI API hatası (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("boş cevap döndü")
	}

	return &kernel.BrainResponse{
		Content: result.Choices[0].Message.Content,
		Usage:   map[string]int{"total_tokens": result.Usage.TotalTokens},
	}, nil
}

func (o *OpenAIProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, fmt.Errorf("OpenAI embed henüz aktif değil")
}