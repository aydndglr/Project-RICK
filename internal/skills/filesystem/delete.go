package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
)

type DeleteTool struct{}

func (t *DeleteTool) Name() string { return "fs_delete" }
func (t *DeleteTool) Description() string {
	return "Dosya veya klasörü siler. 'permanent:false' (varsayılan) ile çöp kutusuna taşır, 'permanent:true' ile kalıcı olarak yok eder."
}

func (t *DeleteTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":      map[string]interface{}{"type": "string", "description": "Silinecek dosya veya klasör yolu."},
			"permanent": map[string]interface{}{"type": "boolean", "description": "True ise geri dönüşümsüz siler. False ise '.rick_trash' klasörüne taşır (Varsayılan: false)."},
		},
		"required": []string{"path"},
	}
}

func (t *DeleteTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path := ResolvePath(args["path"].(string)) 
	permanent, _ := args["permanent"].(bool)

	// 🛡️ GÜVENLİK FİLTRESİ (GUARDRAILS)
	// Rick'in kendi beynini veya kritik dosyaları silmesini engelleyelim.
	protectedPaths := []string{".git", "go.mod", "go.sum", "internal", "cmd", "config/config.yaml"}
	for _, protected := range protectedPaths {
		if strings.Contains(path, protected) || path == "." || path == "/" {
			return "", fmt.Errorf("🛑 GÜVENLİK İHLALİ: '%s' yolu sistem için kritiktir ve silinemez!", path)
		}
	}

	// Dosya var mı kontrol et
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("HATA: Silinmek istenen yol bulunamadı: %s", path)
	}

	if !permanent {
		// ♻️ ÇÖP KUTUSU SİSTEMİ (SOFT DELETE)
		trashDir := ".rick_trash"
		os.MkdirAll(trashDir, 0755)
		
		newName := fmt.Sprintf("%d_%s", time.Now().Unix(), filepath.Base(path))
		trashPath := filepath.Join(trashDir, newName)

		if err := os.Rename(path, trashPath); err != nil {
			return "", fmt.Errorf("çöp kutusuna taşıma başarısız: %v", err)
		}
		
		logger.Warn("♻️ Çöp Kutusu'na Taşındı: %s -> %s", path, trashPath)
		return fmt.Sprintf("✅ '%s' başarıyla çöp kutusuna (.rick_trash) taşındı. Pişman olursan oradan alabilirsin.", path), nil
	}

	// 🔥 KALICI SİLME (HARD DELETE)
	if err := os.RemoveAll(path); err != nil {
		return "", fmt.Errorf("kalıcı silme başarısız: %v", err)
	}
	
	logger.Warn("🗑️ KALICI OLARAK SİLİNDİ: %s", path) 
	
	typeStr := "Dosya"
	if info.IsDir() { typeStr = "Klasör ve içeriği" }
	
	return fmt.Sprintf("🗑️ %s başarıyla ve KALICI olarak silindi: %s", typeStr, path), nil
}