package commands

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Ortak Git Çalıştırıcı
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir // Komutu proje dizininde çalıştır
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// --- 1. GIT STATUS ---

type GitStatusCommand struct {
	BaseDir string
}

func (c *GitStatusCommand) Name() string { return "git_status" }

func (c *GitStatusCommand) Description() string {
	return "Git deposundaki güncel değişiklikleri ve dosya durumlarını listeler."
}

// Parameters: Bu komut argüman almaz.
func (c *GitStatusCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (c *GitStatusCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	out, err := runGit(c.BaseDir, "status")
	if err != nil {
		return "", fmt.Errorf("git hatası: %v", err)
	}
	return fmt.Sprintf("🌲 Git Durumu:\n%s", out), nil
}

// --- 2. GIT COMMIT ---

type GitCommitCommand struct {
	BaseDir string
}

func (c *GitCommitCommand) Name() string { return "git_commit" }

func (c *GitCommitCommand) Description() string {
	return "Tüm değişiklikleri sahneye ekler (stage) ve bir mesajla commit eder."
}

// Parameters: Rick'e bu aracın şemasını bildirir.
func (c *GitCommitCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Commit mesajı (örn: 'feat: yeni araç eklendi' veya 'fix: hata düzeltildi').",
			},
		},
		"required": []string{"message"},
	}
}

func (c *GitCommitCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	msg, ok := args["message"].(string)
	if !ok || msg == "" {
		return "", fmt.Errorf("hata: 'message' parametresi zorunludur")
	}

	// 1. Hepsini ekle
	if _, err := runGit(c.BaseDir, "add", "."); err != nil {
		return "", fmt.Errorf("git add hatası: %v", err)
	}

	// 2. Commit at
	out, err := runGit(c.BaseDir, "commit", "-m", msg)
	if err != nil {
		return fmt.Sprintf("⚠️ Commit yapılamadı (Belki değişiklik yoktur?):\n%s", out), nil
	}

	return fmt.Sprintf("✅ Commit Başarılı:\n%s", out), nil
}

// --- 3. GIT HISTORY ---

type GitHistoryCommand struct {
	BaseDir string
}

func (c *GitHistoryCommand) Name() string { return "git_history" }

func (c *GitHistoryCommand) Description() string {
	return "Projedeki son commit geçmişini (log) özetler."
}

// Parameters: Bu komut argüman almaz.
func (c *GitHistoryCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (c *GitHistoryCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Son 5 commiti, tek satır halinde göster
	out, err := runGit(c.BaseDir, "log", "-n", "5", "--oneline")
	if err != nil {
		return "", fmt.Errorf("git log hatası: %v", err)
	}
	return fmt.Sprintf("📜 Son Commitler:\n%s", out), nil
}