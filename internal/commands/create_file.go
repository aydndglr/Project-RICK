package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type CreateFileCommand struct {
	BaseDir string
}

func (c *CreateFileCommand) Name() string { return "create_file" }
func (c *CreateFileCommand) Description() string {
	return "Yeni bir dosya oluşturur ve içeriğini yazar. Tam yol (Absolute Path) verebilirsin. Kullanım: { \"path\": \"C:\\Users\\X\\Desktop\\deneme.txt\", \"content\": \"merhaba\" }"
}

func (c *CreateFileCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, okP := args["path"].(string)
	content, okC := args["content"].(string)
	if !okP || !okC {
		return "", fmt.Errorf("eksik parametre: path ve content zorunludur")
	}

	var fullPath string

	// --- KRİTİK DEĞİŞİKLİK BURADA ---
	// Eğer yol "Absolute" ise (Örn: C:\Users\...) direkt kullan.
	// Değilse, proje klasörüne (BaseDir) ekle.
	if filepath.IsAbs(path) {
		fullPath = path
	} else {
		fullPath = filepath.Join(c.BaseDir, path)
	}
	// -------------------------------

	// Klasör yoksa oluştur (Özyinelemeli)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("klasör oluşturulamadı: %v", err)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("dosya yazılamadı: %v", err)
	}

	return fmt.Sprintf("✅ Dosya başarıyla oluşturuldu: %s", fullPath), nil
}