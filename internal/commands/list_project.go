package commands

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

type ListProjectCommand struct {
	BaseDir string
}

func (c *ListProjectCommand) Name() string { return "list_project" }

func (c *ListProjectCommand) Description() string {
	return "Proje dizin yapısını ve dosyaları ağaç şeklinde listeler."
}

// Parameters: Bu komut argüman almaz, o yüzden boş şema dönüyoruz.
func (c *ListProjectCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (c *ListProjectCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	var tree strings.Builder
	tree.WriteString(fmt.Sprintf("📂 Proje Kök Dizini: %s\n", c.BaseDir))

	// Dosya sistemini tara
	err := filepath.WalkDir(c.BaseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Gizli dosyaları ve büyük klasörleri atla (Rick zaman kaybetmez!)
		if c.shouldIgnore(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Yolu temizle (BaseDir kısmını çıkar)
		relPath, _ := filepath.Rel(c.BaseDir, path)
		if relPath == "." {
			return nil
		}

		// Derinliğe göre boşluk (indent) ekle
		depth := strings.Count(relPath, string(filepath.Separator))
		indent := strings.Repeat("  ", depth)

		icon := "📄"
		if d.IsDir() {
			icon = "📁"
		}

		tree.WriteString(fmt.Sprintf("%s%s %s\n", indent, icon, d.Name()))
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("proje taranırken hata oluştu: %v", err)
	}

	return tree.String(), nil
}

// shouldIgnore: Rick'in kafasını karıştıracak gereksiz klasörleri filtreler
func (c *ListProjectCommand) shouldIgnore(name string) bool {
	ignoreList := []string{
		".git",
		".vscode",
		"node_modules",
		"vendor",
		"bin",
		"rick_whatsapp.db", // Kendi DB'sini okumasın
		"__pycache__",      // Python cache
	}

	for _, ignore := range ignoreList {
		if name == ignore {
			return true
		}
	}

	// Gizli dosyalar (nokta ile başlayanlar)
	return strings.HasPrefix(name, ".") && name != "."
}