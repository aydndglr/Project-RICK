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

type GeminiProvider struct {
	BaseURL string
	APIKey  string
	Model   string
	Client  *http.Client
}

func NewGemini(url, key, model string) *GeminiProvider {
	if url == "" {
		url = "https://generativelanguage.googleapis.com"
	}
	return &GeminiProvider{
		BaseURL: strings.TrimSuffix(url, "/"),
		APIKey:  key,
		Model:   model,
		Client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (g *GeminiProvider) Chat(ctx context.Context, history []kernel.Message, tools []kernel.Tool) (*kernel.BrainResponse, error) {
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", g.BaseURL, g.Model, g.APIKey)

	// --- GEMINI API YAPILARI ---
	type inlineData struct {
		MimeType string `json:"mimeType"`
		Data     string `json:"data"`
	}
	type functionCall struct {
		Name string                 `json:"name"`
		Args map[string]interface{} `json:"args"`
	}
	type functionResponse struct {
		Name     string                 `json:"name"`
		Response map[string]interface{} `json:"response"`
	}
	type part struct {
		Text             string            `json:"text,omitempty"`
		InlineData       *inlineData       `json:"inlineData,omitempty"`
		FunctionCall     *functionCall     `json:"functionCall,omitempty"`
		FunctionResponse *functionResponse `json:"functionResponse,omitempty"`
	}
	type content struct {
		Role  string `json:"role"`
		Parts []part `json:"parts"`
	}
	type systemInstruction struct {
		Parts []part `json:"parts"`
	}
	type functionDeclaration struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Parameters  map[string]interface{} `json:"parameters"`
	}
	type toolWrapper struct {
		FunctionDeclarations []functionDeclaration `json:"functionDeclarations"`
	}
	type geminiReq struct {
		SystemInstruction *systemInstruction `json:"systemInstruction,omitempty"`
		Contents          []content          `json:"contents"`
		Tools             []toolWrapper      `json:"tools,omitempty"`
	}

	reqBody := geminiReq{}

	// 1. ARAÇLARI (TOOLS) YÜKLE
	if len(tools) > 0 {
		var funcs []functionDeclaration
		for _, t := range tools {
			funcs = append(funcs, functionDeclaration{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Parameters(),
			})
		}
		reqBody.Tools = append(reqBody.Tools, toolWrapper{FunctionDeclarations: funcs})
	}

	// 2. MESAJ GEÇMİŞİNİ İŞLE (Sıra Hatalarını Çözen Algoritma)
	var contents []content
	for _, h := range history {
		// Sistem mesajını özel alana al
		if h.Role == "system" {
			reqBody.SystemInstruction = &systemInstruction{
				Parts: []part{{Text: h.Content}},
			}
			continue
		}

		role := h.Role
		if role == "assistant" {
			role = "model"
		} else if role == "tool" {
			role = "function" // Gemini, tool cevaplarını 'function' rolüyle bekler
		} else {
			role = "user"
		}

		var parts []part

		// Tool Çıktısı mı?
		if h.Role == "tool" {
			parts = append(parts, part{
				FunctionResponse: &functionResponse{
					Name:     h.Name,
					Response: map[string]interface{}{"result": h.Content},
				},
			})
		} else if len(h.ToolCalls) > 0 { // Asistan Tool mu Çağırdı?
			for _, tc := range h.ToolCalls {
				parts = append(parts, part{
					FunctionCall: &functionCall{
						Name: tc.Function,
						Args: tc.Arguments,
					},
				})
			}
		} else if h.Content != "" { // Normal Metin
			parts = append(parts, part{Text: h.Content})
		}

		// Görseller
		for _, img := range h.Images {
			mimeType := "image/jpeg"
			b64Data := img
			if strings.HasPrefix(img, "data:") {
				partsSplit := strings.SplitN(img, ";base64,", 2)
				if len(partsSplit) == 2 {
					mimeType = strings.TrimPrefix(partsSplit[0], "data:")
					b64Data = partsSplit[1]
				}
			}
			parts = append(parts, part{
				InlineData: &inlineData{MimeType: mimeType, Data: b64Data},
			})
		}

		// 🚀 KRİTİK: Peş peşe aynı rolden mesaj gelirse API çökmesin diye birleştir!
		if len(contents) > 0 && contents[len(contents)-1].Role == role {
			contents[len(contents)-1].Parts = append(contents[len(contents)-1].Parts, parts...)
		} else {
			contents = append(contents, content{Role: role, Parts: parts})
		}
	}
	reqBody.Contents = contents

	// 3. İSTEK GÖNDER
	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini API hatası (%d): %s", resp.StatusCode, string(b))
	}

	// 4. CEVABI AYRIKLA
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []part `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini boş cevap döndü")
	}

	brainResp := &kernel.BrainResponse{}

	for _, p := range result.Candidates[0].Content.Parts {
		if p.Text != "" {
			brainResp.Content += p.Text
		}
		if p.FunctionCall != nil {
			brainResp.ToolCalls = append(brainResp.ToolCalls, kernel.ToolCall{
				Function:  p.FunctionCall.Name,
				Arguments: p.FunctionCall.Args,
			})
		}
	}

	return brainResp, nil
}

// Hafıza (Vector Store) çökmesin diye güvenli bir Dummy (Boş) Embed fonksiyonu
func (g *GeminiProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	// API'yi yormamak için Rick'in hafızasına geçici boş vektör dönüyoruz.
	return make([]float32, 1536), nil
}