package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type PythonRunCommand struct {
	BaseDir string
}

func (c *PythonRunCommand) Name() string { return "python_run" }

func (c *PythonRunCommand) Description() string {
	return "Verilen Python kodunu sistemde çalıştırır ve çıktısını döner. Karmaşık hesaplamalar, veri çekme veya script işlemleri için bunu kullan. Parametre: 'code'."
}

func (c *PythonRunCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	code, ok := args["code"].(string)
	if !ok || code == "" {
		return "", fmt.Errorf("eksik parametre: code")
	}

	// 1. Geçici bir .py dosyası oluştur
	tmpFile := filepath.Join(c.BaseDir, "temp_script.py")
	err := os.WriteFile(tmpFile, []byte(code), 0644)
	if err != nil {
		return "", fmt.Errorf("script dosyası oluşturulamadı: %v", err)
	}
	defer os.Remove(tmpFile) // İşlem bitince dosyayı temizle

	// 2. Python komutunu belirle (python veya python3)
	pythonCmd := "python"
	if runtime.GOOS != "windows" {
		pythonCmd = "python3"
	}

	// 3. Kodu çalıştır
	cmd := exec.CommandContext(ctx, pythonCmd, tmpFile)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Sprintf("❌ Python Hatası:\n%s\nHata Detayı: %v", string(output), err), nil
	}

	return string(output), nil
}