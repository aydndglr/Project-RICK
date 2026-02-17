package commands

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

type RunCommand struct{}

func (c *RunCommand) Name() string { return "run_command" }

func (c *RunCommand) Description() string {
	return "Terminalde sistem komutu çalıştırır. Windows için PowerShell, Linux/Mac için Bash kullanılır. Kullanım: { \"command\": \"go test ./...\" }"
}

func (c *RunCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// 1. Komutu Al
	cmdStr, ok := args["command"].(string)
	if !ok || cmdStr == "" {
		return "", fmt.Errorf("eksik parametre: 'command' zorunludur")
	}

	// 2. İşletim Sistemine Göre Hazırla
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows'ta PowerShell kullanımı
		cmd = exec.CommandContext(ctx, "powershell", "-Command", cmdStr)
	} else {
		// Linux/Mac'te Bash kullanımı
		cmd = exec.CommandContext(ctx, "bash", "-c", cmdStr)
	}

	// 3. Çıktıları Yakala
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 4. Çalıştır
	err := cmd.Run()

	output := strings.TrimSpace(stdout.String())
	errOutput := strings.TrimSpace(stderr.String())

	// 5. Sonucu Raporla
	if err != nil {
		return "", fmt.Errorf("komut hatası (Exit Code: %v):\n%s", err, errOutput)
	}

	if output == "" && errOutput == "" {
		return "✅ Komut başarıyla çalıştı ancak bir çıktı üretmedi.", nil
	}

	if errOutput != "" {
		return fmt.Sprintf("⚠️ Komut çalıştı ama stderr mesajı var:\n%s\n\nÇıktı:\n%s", errOutput, output), nil
	}

	return fmt.Sprintf("💻 Terminal Çıktısı:\n%s", output), nil
}