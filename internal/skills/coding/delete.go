package coding

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
)

type ToolDeleter struct {
	WorkspaceDir string
}

func NewDeleter(workspaceDir string) *ToolDeleter {
	return &ToolDeleter{WorkspaceDir: workspaceDir}
}

func (d *ToolDeleter) Name() string { return "delete_python_tool" }

func (d *ToolDeleter) Description() string {
	return "Gereksiz, hatalı veya artık kullanılmayan bir Python aracını sistemden TAMAMEN VE KALICI OLARAK siler."
}

func (d *ToolDeleter) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"filename": map[string]interface{}{"type": "string", "description": "Silinecek dosya adı (Örn: script.py)."},
		},
		"required": []string{"filename"},
	}
}

func (d *ToolDeleter) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// 1. Esnek Parametre Yakalama
	filename, ok := args["filename"].(string)
	if !ok || strings.TrimSpace(filename) == "" {
		filename, _ = args["name"].(string)
	}

	filename = strings.TrimSpace(filename)
	if filename == "" {
		return "", fmt.Errorf("HATA: 'filename' parametresi eksik! Neyi sileceğimi belirtmelisin.")
	}

	if !strings.HasSuffix(filename, ".py") { filename += ".py" }

	// 🛡️ GÜVENLİK ZIRHI: Dizin Atlama (Path Traversal) Saldırılarını Önle
	// Kullanıcı veya LLM "../../gizli_dosya.py" gönderse bile sadece "gizli_dosya.py" kısmını alır.
	filename = filepath.Base(filename)
	
	fullPath := filepath.Join(d.WorkspaceDir, filename)

	// 2. Silme İşlemi
	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("HATA: '%s' adında bir dosya çalışma alanında bulunamadı. Zaten silinmiş veya ismi yanlış olabilir.", filename)
		}
		return "", fmt.Errorf("Dosya silinemedi: %v", err)
	}

	logger.Action("🗑️ Araç Silindi: %s", filename)
	
	// Registry'den kaldır
	removeFromRegistryFile(d.WorkspaceDir, filename)

	return fmt.Sprintf("✅ BAŞARILI: '%s' sistemden ve kayıtlardan tamamen silindi.", filename), nil
}