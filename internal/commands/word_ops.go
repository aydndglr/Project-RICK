package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nguyenthenguyen/docx"
)

// --- 1. WORD READ ---
type WordReadCommand struct{ BaseDir string }

func (c *WordReadCommand) Name() string { return "word_read" }
func (c *WordReadCommand) Description() string { return "Word (.docx) dosyasının metin içeriğini okur." }

func (c *WordReadCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Okunacak Word dosyasının yolu.",
			},
		},
		"required": []string{"path"},
	}
}

func (c *WordReadCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok { return "", fmt.Errorf("eksik parametre: path") }
	
	fullPath := filepath.Join(c.BaseDir, path)
	r, err := docx.ReadDocxFile(fullPath)
	if err != nil { return "", fmt.Errorf("dosya açılamadı: %v", err) }
	defer r.Close()

	content := r.Editable().GetContent()
	return fmt.Sprintf("📝 WORD İÇERİĞİ:\n%s", stripXML(content)), nil
}

// --- 2. WORD CREATE ---
type WordCreateCommand struct{ BaseDir string }

func (c *WordCreateCommand) Name() string { return "word_create" }
func (c *WordCreateCommand) Description() string { return "Profesyonel bir Word dokümanı oluşturur." }

func (c *WordCreateCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":    map[string]interface{}{"type": "string", "description": "Kaydedilecek dosya yolu."},
			"title":   map[string]interface{}{"type": "string", "description": "Doküman başlığı."},
			"content": map[string]interface{}{
				"type": "array", 
				"items": map[string]interface{}{"type": "string"},
				"description": "Paragraflardan oluşan liste.",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (c *WordCreateCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensurePythonLibs(ctx, "python-docx"); err != nil { return "", err }

	path, _ := args["path"].(string)
	title, _ := args["title"].(string)
	contentRaw, _ := args["content"].([]interface{})

	if path == "" { return "", fmt.Errorf("eksik parametre: path") }

	var contentStr strings.Builder
	contentStr.WriteString("[")
	for _, item := range contentRaw {
		contentStr.WriteString(fmt.Sprintf("r'''%v''', ", item))
	}
	contentStr.WriteString("]")

	script := fmt.Sprintf(`
from docx import Document
import sys
import io

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
try:
    doc = Document()
    if r'%s': doc.add_heading(r'%s', 0)
    paragraphs = %s
    for p in paragraphs:
        doc.add_paragraph(p)
    doc.save(r'%s')
    print("✅ Word dosyası oluşturuldu.")
except Exception as e:
    print(f"HATA: {e}")
`, title, title, contentStr.String(), filepath.Join(c.BaseDir, path))

	return runEmbeddedPython(ctx, script, c.BaseDir)
}