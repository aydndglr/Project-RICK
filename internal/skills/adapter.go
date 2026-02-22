package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// PythonTool: Dinamik Python dosyalarını sarmalayan yapı.
type PythonTool struct {
	name        string
	description string
	scriptPath  string
	interpreter string // python veya venv/bin/python
}

// NewPythonTool: Yeni bir Python aracı oluşturur.
func NewPythonTool(name, desc, path, pythonPath string) *PythonTool {
	return &PythonTool{
		name:        name,
		description: desc,
		scriptPath:  path,
		interpreter: pythonPath,
	}
}

func (p *PythonTool) Name() string {
	return p.name
}

func (p *PythonTool) Description() string {
	return p.description
}

// Parameters: Dinamik araçlar için esnek parametre yapısı.
// Rick'in yazdığı scriptler genellikle JSON string veya argv bekler.
func (p *PythonTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			// Genellikle araçlar key-value argümanlarla çalışır
			// Rick'e esneklik tanımak için "kwargs" gibi davranıyoruz
			"args": map[string]interface{}{
				"type":        "object",
				"description": "Script'e gönderilecek parametreler (JSON formatında).",
				// DÜZELTME: Gemini'nin hata verdiği 'additionalProperties' satırı kaldırıldı.
				// Ollama ve OpenAI bu alan olmadan da standart 'object' tipini sorunsuz işler.
			},
		},
	}
}

// Execute: Python kodunu çalıştırır ve çıktısını yakalar.
func (p *PythonTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// 1. Argümanları JSON'a çevir (Python tarafında json.loads ile okunacak)
	jsonArgs, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("argüman paketleme hatası: %v", err)
	}

	// 2. Komutu hazırla
	// python script.py '{"key": "value"}'
	cmd := exec.CommandContext(ctx, p.interpreter, p.scriptPath, string(jsonArgs))
	
	// Çevresel değişkenleri ayarla (Gerekirse API keyleri buraya eklenir)
	cmd.Env = os.Environ()

	// 3. Çalıştır
	output, err := cmd.CombinedOutput()
	result := string(output)

	if err != nil {
		// Hata durumunda stderr çıktısını da dönelim ki Rick hatayı görüp düzeltsin
		return fmt.Sprintf("❌ Script Hatası (%s): %v\nÇıktı:\n%s", p.name, err, result), nil
	}

	// Başarılı çıktı
	return strings.TrimSpace(result), nil
}