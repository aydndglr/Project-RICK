package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type ReadFileCommand struct {
	BaseDir string
}

func (c *ReadFileCommand) Name() string { return "read_file" }

func (c *ReadFileCommand) Description() string {
	return "Belirtilen dosyanın içeriğini okur. Kullanım: { \"path\": \"dosya_yolu.go\" }"
}

func (c *ReadFileCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// 1. Parametre Kontrolü
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("hata: 'path' parametresi eksik veya geçersiz")
	}

	// 2. Güvenli Yol Oluşturma
	// Rick'in sadece çalışma dizininde kalmasını sağlıyoruz
	fullPath := filepath.Join(c.BaseDir, path)

	// 3. Dosyayı Oku
	content, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("dosya bulunamadı: %s", path)
		}
		return "", fmt.Errorf("dosya okunamadı: %v", err)
	}

	// 4. Sonucu Dön
	return fmt.Sprintf("📄 Dosya: %s\n\n%s", path, string(content)), nil
}