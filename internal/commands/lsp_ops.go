package commands

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type CodeAnalyzeCommand struct {
	BaseDir string
}

func (c *CodeAnalyzeCommand) Name() string { return "code_analyze" }

func (c *CodeAnalyzeCommand) Description() string {
	return "Kodu derlemeden statik analiz (LSP/Lint) yapar; yazım hatalarını ve potansiyel bugları raporlar."
}

// Parameters: Rick'e bu aracın şemasını bildirir.
func (c *CodeAnalyzeCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Statik analiz yapılacak dosya veya klasörün yolu (Örn: 'cmd/main.go' veya 'internal/').",
			},
		},
		"required": []string{"path"},
	}
}

func (c *CodeAnalyzeCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("hata: 'path' parametresi eksik veya geçersiz")
	}

	fullPath := filepath.Join(c.BaseDir, path)
	ext := filepath.Ext(fullPath)

	var cmd *exec.Cmd
	var toolName string

	switch ext {
	case ".go":
		toolName = "go vet"
		dir := filepath.Dir(fullPath)
		if strings.HasSuffix(fullPath, "/...") {
			dir = fullPath 
		}
		cmd = exec.Command("go", "vet", dir)

	case ".py":
		toolName = "python syntax check"
		cmd = exec.Command("python", "-m", "py_compile", fullPath)

	case ".js", ".ts":
		toolName = "node check"
		cmd = exec.Command("node", "--check", fullPath)

	default:
		return "", fmt.Errorf("bu dosya türü için analizci tanımlı değil: %s", ext)
	}

	output, err := cmd.CombinedOutput()
	outStr := string(output)

	if err != nil {
		return fmt.Sprintf("❌ %s HATA BULDU:\n%s\n(Lütfen bu hataları düzelt)", toolName, outStr), nil
	}

	if outStr == "" {
		return fmt.Sprintf("✅ %s Analizi: Kod temiz görünüyor.", toolName), nil
	}

	return fmt.Sprintf("⚠️ %s Uyarıları:\n%s", toolName, outStr), nil
}