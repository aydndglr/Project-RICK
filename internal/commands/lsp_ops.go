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
	return "Kodu derlemeden statik analiz (LSP/Lint) yapar. Parametre: path (dosya veya klasör)."
}

func (c *CodeAnalyzeCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("eksik parametre: path")
	}

	fullPath := filepath.Join(c.BaseDir, path)
	ext := filepath.Ext(fullPath)

	var cmd *exec.Cmd
	var toolName string

	switch ext {
	case ".go":
		// Go için 'go vet' (Standart araç, ekstra kuruluma gerek yok)
		toolName = "go vet"
		// go vet paket bazlı çalışır, klasöre işaret edelim
		dir := filepath.Dir(fullPath)
		if strings.HasSuffix(fullPath, "/...") {
			dir = fullPath // Recursive ise
		}
		cmd = exec.Command("go", "vet", dir)

	case ".py":
		// Python için syntax kontrolü (compile check)
		toolName = "python syntax check"
		cmd = exec.Command("python", "-m", "py_compile", fullPath)

	case ".js", ".ts":
		// Node varsa syntax check
		toolName = "node check"
		cmd = exec.Command("node", "--check", fullPath)

	default:
		return "", fmt.Errorf("bu dosya türü için analizci tanımlı değil: %s", ext)
	}

	output, err := cmd.CombinedOutput()
	outStr := string(output)

	if err != nil {
		// Hata varsa Rick'e detaylı rapor ver
		return fmt.Sprintf("❌ %s HATA BULDU:\n%s\n(Lütfen bu hataları düzelt)", toolName, outStr), nil
	}

	if outStr == "" {
		return fmt.Sprintf("✅ %s Analizi: Kod temiz görünüyor.", toolName), nil
	}

	return fmt.Sprintf("⚠️ %s Uyarıları:\n%s", toolName, outStr), nil
}