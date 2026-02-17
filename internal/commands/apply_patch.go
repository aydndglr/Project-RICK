package commands

import (
	"context"
	"fmt"
	"path/filepath"

	// YENİ IMPORT: Tools yerine Patcher kullanıyoruz
	"github.com/aydndglr/rick-agent/internal/patcher"
)

type ApplyPatchCommand struct {
	BaseDir string
}

func (c *ApplyPatchCommand) Name() string { return "apply_patch" }

func (c *ApplyPatchCommand) Description() string {
	return "Dosyada metin tabanlı değişiklik yapar. Belirli bir kod bloğunu bulur ve yeni blokla değiştirir."
}

// Parameters: Rick'e bu aracın şemasını bildirir.
func (c *ApplyPatchCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Değişiklik yapılacak dosyanın yolu.",
			},
			"search": map[string]interface{}{
				"type":        "string",
				"description": "Dosya içinde aranacak olan mevcut kod bloğu (Search Block).",
			},
			"replace": map[string]interface{}{
				"type":        "string",
				"description": "Mevcut bloğun yerine koyulacak yeni kod bloğu (Replace Block).",
			},
		},
		"required": []string{"path", "search", "replace"},
	}
}

func (c *ApplyPatchCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, okP := args["path"].(string)
	searchBlock, okS := args["search"].(string)
	replaceBlock, okR := args["replace"].(string)

	if !okP || !okS || !okR {
		return "", fmt.Errorf("eksik parametre: 'path', 'search' ve 'replace' zorunludur")
	}

	fullPath := filepath.Join(c.BaseDir, path)

	// YENİ KULLANIM: patcher.NewSmartPatcher [cite: 213]
	p := patcher.NewSmartPatcher(c.BaseDir)
	
	err := p.Apply(fullPath, searchBlock, replaceBlock) 
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("✅ Patch başarıyla uygulandı: %s", path), nil
}