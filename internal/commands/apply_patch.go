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
	return "Dosyada metin tabanlı değişiklik yapar. 'search' bloğunu bulur ve 'replace' bloğuyla değiştirir."
}

func (c *ApplyPatchCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, okP := args["path"].(string)
	searchBlock, okS := args["search"].(string)
	replaceBlock, okR := args["replace"].(string)

	if !okP || !okS || !okR {
		return "", fmt.Errorf("eksik parametre: 'path', 'search' ve 'replace' zorunludur")
	}

	fullPath := filepath.Join(c.BaseDir, path)

	// YENİ KULLANIM: patcher.NewSmartPatcher
	p := patcher.NewSmartPatcher(c.BaseDir)
	
	err := p.Apply(fullPath, searchBlock, replaceBlock)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("✅ Patch başarıyla uygulandı: %s", path), nil
}