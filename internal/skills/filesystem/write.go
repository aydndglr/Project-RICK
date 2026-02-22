package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
)

type WriteTool struct{}

func (t *WriteTool) Name() string { return "fs_write" }
func (t *WriteTool) Description() string {
	return "Dosyaya veri yazar. 'mode' parametresi ile üzerine yazabilir (overwrite), sonuna ekleyebilir (append) veya belirli bir satıra ekleme yapabilirsin (insert)."
}

func (t *WriteTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":    map[string]interface{}{"type": "string", "description": "İşlem yapılacak dosya yolu."},
			"content": map[string]interface{}{"type": "string", "description": "Yazılacak içerik."},
			"mode":    map[string]interface{}{"type": "string", "description": "Kayıt modu: 'overwrite' (üzerine yaz - varsayılan), 'append' (sonuna ekle), 'insert' (belirli satıra ekle).", "enum": []string{"overwrite", "append", "insert"}},
			"line":    map[string]interface{}{"type": "integer", "description": "Sadece 'insert' modunda geçerlidir. İçeriğin ekleneceği satır numarası."},
		},
		"required": []string{"path", "content"},
	}
}

func (t *WriteTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// 🛡️ GÜVENLİ TÜR DÖNÜŞÜMÜ (PANIC KORUMASI)
	pathRaw, ok := args["path"]
	if !ok || pathRaw == nil {
		return "", fmt.Errorf("HATA: 'path' parametresi eksik. Nereye yazacağımı belirtmelisin")
	}
	pathStr, ok := pathRaw.(string)
	if !ok {
		return "", fmt.Errorf("HATA: 'path' parametresi metin (string) formatında olmalı")
	}
	path := ResolvePath(pathStr)

	contentRaw, ok := args["content"]
	if !ok || contentRaw == nil {
		return "", fmt.Errorf("HATA: 'content' parametresi eksik. Dosyaya ne yazacağımı belirtmelisin")
	}
	contentStr, ok := contentRaw.(string)
	if !ok {
		return "", fmt.Errorf("HATA: 'content' parametresi metin (string) formatında olmalı")
	}
	content := contentStr

	// Varsayılan mod overwrite
	mode := "overwrite"
	if m, ok := args["mode"].(string); ok && m != "" {
		mode = m
	}

	line := 1
	if l, ok := args["line"].(float64); ok {
		line = int(l)
	}

	// Klasör hiyerarşisini garantiye al
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}

	// Dosya hiç yoksa insert veya append yapamayız, mecburen overwrite moduna dönüyoruz
	if _, err := os.Stat(path); os.IsNotExist(err) {
		mode = "overwrite"
	}

	switch mode {
	case "append":
		// Dosyayı sadece yazma ve ekleme modunda aç
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return "", err
		}
		defer f.Close()
		if _, err := f.WriteString("\n" + content); err != nil {
			return "", err
		}
		logger.Success("💾 Dosya Sonuna Eklendi: %s", path)
		return fmt.Sprintf("✅ İşlem Başarılı: %s dosyasına içerik eklendi (append).", path), nil

	case "insert":
		// Dosyayı oku ve satırlara böl
		fileBytes, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		
		lines := strings.Split(string(fileBytes), "\n")
		
		// Satır numarası mantık sınırları dışındaysa düzelt (Guardrails)
		if line < 1 {
			line = 1
		}
		if line > len(lines) {
			line = len(lines) + 1
		}

		// Yeni içeriği araya yerleştirme (Go Slice Magic)
		var newLines []string
		newLines = append(newLines, lines[:line-1]...) // Eklenecek yere kadar olan mevcut kısım
		newLines = append(newLines, content)           // Rick'in enjekte ettiği kod/metin
		newLines = append(newLines, lines[line-1:]...) // Dosyanın geri kalanı

		finalContent := strings.Join(newLines, "\n")
		
		if err := os.WriteFile(path, []byte(finalContent), 0644); err != nil {
			return "", err
		}
		logger.Success("💾 Dosyaya Satır Eklendi (Satır: %d): %s", line, path)
		return fmt.Sprintf("✅ İşlem Başarılı: %s dosyasının %d. satırına içerik yerleştirildi (insert).", path, line), nil

	default: // "overwrite"
		// Mevcut sistemin çalıştığı gibi her şeyi ezer 
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return "", err
		}
		logger.Success("💾 Dosya Üzerine Yazıldı: %s", path)
		return fmt.Sprintf("✅ İşlem Başarılı: %s dosyası baştan yaratıldı (overwrite).", path), nil
	}
}