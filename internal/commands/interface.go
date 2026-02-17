package commands

import (
	"context"
)

// Command: Her Rick komutunun uygulaması gereken interface
type Command interface {
	Name() string                                      // Komutun benzersiz adı (örn: create_file)
	Description() string                               // Modelin ne zaman kullanacağını anlaması için açıklama
	
	// Parameters: Aracın kabul ettiği JSON şeması (OpenAI/Ollama standardı).
	// Bu fonksiyon, modelin hangi parametreleri (string, int, required vb.) göndermesi gerektiğini söyler.
	Parameters() map[string]interface{} 
	
	Execute(ctx context.Context, args map[string]interface{}) (string, error) // Gerçek Go kodu
}