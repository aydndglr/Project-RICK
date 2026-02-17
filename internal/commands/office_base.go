package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ensurePythonLibs: Gerekli Python kütüphanelerini kontrol eder, yoksa yükler.
func ensurePythonLibs(ctx context.Context, libs ...string) error {
	cmd := exec.CommandContext(ctx, "pip", "list")
	output, err := cmd.CombinedOutput()
	installedPackages := string(output)

	if err != nil {
		return fmt.Errorf("pip komutu çalıştırılamadı, Python yüklü mü? Hata: %v", err)
	}

	for _, lib := range libs {
		if !strings.Contains(strings.ToLower(installedPackages), strings.ToLower(lib)) {
			fmt.Printf("📦 Rick: '%s' eksik, otomatik yükleniyor...\n", lib)
			installCmd := exec.CommandContext(ctx, "pip", "install", lib)
			if out, err := installCmd.CombinedOutput(); err != nil {
				return fmt.Errorf("%s yüklenirken hata oluştu: %s", lib, string(out))
			}
			fmt.Printf("✅ Rick: '%s' başarıyla yüklendi.\n", lib)
		}
	}
	return nil
}

// runEmbeddedPython: Go içinden geçici Python scripti çalıştırır.
func runEmbeddedPython(ctx context.Context, scriptContent string, baseDir string) (string, error) {
	tmpPath := filepath.Join(baseDir, fmt.Sprintf("rick_script_%d.py", os.Getpid()))
	if err := os.WriteFile(tmpPath, []byte(scriptContent), 0644); err != nil {
		return "", err
	}
	defer os.Remove(tmpPath)

	cmd := exec.CommandContext(ctx, "python", tmpPath)
	cmd.Dir = baseDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("script hatası: %v\nÇıktı: %s", err, string(output))
	}
	return string(output), nil
}

func stripXML(input string) string {
	var sb strings.Builder
	inTag := false
	for _, r := range input {
		if r == '<' { inTag = true; continue }
		if r == '>' { inTag = false; sb.WriteString(" "); continue }
		if !inTag { sb.WriteRune(r) }
	}
	return strings.TrimSpace(sb.String())
}