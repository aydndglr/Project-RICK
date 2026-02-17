package commands

import (
	"context"
	"fmt"
	"strings"
)

// --- PRESENT ANSWER (Final Sunum Aracı) ---

type PresentAnswerCommand struct{}

func (c *PresentAnswerCommand) Name() string { return "present_answer" }

func (c *PresentAnswerCommand) Description() string {
	return "Görevi tamamladığında ve kullanıcıya nihai cevabı vermek istediğinde BU ARACI KULLANMAK ZORUNDASIN."
}

// Parameters: Rick'e bu aracın şemasını bildirir.
func (c *PresentAnswerCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"answer": map[string]interface{}{
				"type":        "string",
				"description": "Kullanıcıya gösterilecek detaylı ve nihai yanıt.",
			},
		},
		"required": []string{"answer"},
	}
}

func (c *PresentAnswerCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// GÜNCELLEME: Şema "answer" zorunlu kılıyor ama yine de esnek olalım (Fallback Logic).
	
	// 1. Öncelikli olarak 'answer' ara
	if val, ok := args["answer"].(string); ok && val != "" {
		return val, nil
	}

	// 2. Yoksa alternatiflere bak (message, content, text, result)
	alternatives := []string{"message", "content", "text", "result", "output"}
	for _, key := range alternatives {
		if val, ok := args[key].(string); ok && val != "" {
			return val, nil // Hata vermek yerine kabul et
		}
	}
	
	// 3. Eğer liste gönderdiyse (bazen yapıyor)
	if resList, ok := args["results"].([]interface{}); ok {
		var sb strings.Builder
		for _, item := range resList {
			sb.WriteString(fmt.Sprintf("- %v\n", item))
		}
		return sb.String(), nil
	}

	// Hiçbiri yoksa
	return "", fmt.Errorf("HATA: Cevap metnini bulamadım. Lütfen 'answer' parametresini kullan.")
}