package commands

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// --- 1. OPEN APP COMMAND ---

type OpenAppCommand struct{}

func (c *OpenAppCommand) Name() string { return "open_app" }

func (c *OpenAppCommand) Description() string {
	return "Bilgisayardaki bir uygulamayı başlatır."
}

func (c *OpenAppCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"app_name": map[string]interface{}{
				"type":        "string",
				"description": "Başlatılacak uygulamanın adı (örn: 'notepad', 'calc', 'chrome').",
			},
		},
		"required": []string{"app_name"},
	}
}

func (c *OpenAppCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	appName, ok := args["app_name"].(string)
	if !ok || appName == "" {
		return "", fmt.Errorf("eksik parametre: app_name")
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows'ta 'start' komutu ile açmak daha güvenilirdir
		cmd = exec.Command("cmd", "/C", "start", appName)
	} else if runtime.GOOS == "darwin" { // Mac
		cmd = exec.Command("open", "-a", appName)
	} else { // Linux
		cmd = exec.Command("xdg-open", appName)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("uygulama açılamadı: %v", err)
	}

	return fmt.Sprintf("✅ Uygulama başlatıldı: %s", appName), nil
}

// --- 2. KILL PROCESS COMMAND ---

type KillProcessCommand struct{}

func (c *KillProcessCommand) Name() string { return "kill_process" }

func (c *KillProcessCommand) Description() string {
	return "Çalışan bir uygulamayı isminden sonlandırır."
}

func (c *KillProcessCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"process_name": map[string]interface{}{
				"type":        "string",
				"description": "Sonlandırılacak işlemin adı (örn: 'chrome.exe').",
			},
		},
		"required": []string{"process_name"},
	}
}

func (c *KillProcessCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	procName, ok := args["process_name"].(string)
	if !ok || procName == "" {
		return "", fmt.Errorf("eksik parametre: process_name")
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// /F: Force, /IM: Image Name
		cmd = exec.Command("taskkill", "/F", "/IM", procName)
	} else {
		cmd = exec.Command("pkill", "-f", procName)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("⚠️ İşlem sonlandırılamadı (Belki zaten kapalıdır?):\n%s", string(output)), nil
	}

	return fmt.Sprintf("🛑 İşlem sonlandırıldı: %s", procName), nil
}

// --- 3. VIRUS SCAN COMMAND (Windows Defender) ---

type VirusScanCommand struct {
	BaseDir string
}

func (c *VirusScanCommand) Name() string { return "virus_scan" }

func (c *VirusScanCommand) Description() string {
	return "Belirtilen dosya veya klasörü Windows Defender ile tarar."
}

func (c *VirusScanCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Taranacak dosya veya klasörün tam yolu.",
			},
		},
		"required": []string{"path"},
	}
}

func (c *VirusScanCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if runtime.GOOS != "windows" {
		return "⚠️ Bu özellik sadece Windows'ta çalışır.", nil
	}

	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("eksik parametre: path")
	}

	// Defender CLI aracı genelde bu yoldadır
	defenderPath := "C:\\Program Files\\Windows Defender\\MpCmdRun.exe"
	
	// -Scan -ScanType 3 -File <path> (3 = Custom Scan)
	cmd := exec.Command(defenderPath, "-Scan", "-ScanType", "3", "-File", path)
	
	output, err := cmd.CombinedOutput()
	result := string(output)

	if err != nil {
		// Defender tehdit bulduğunda bazen exit code 2 döner
		if strings.Contains(result, "threat") || strings.Contains(result, "found") {
			return fmt.Sprintf("🚨 TEHDİT ALGILANDI!\n%s", result), nil
		}
		return fmt.Sprintf("⚠️ Tarama hatası veya temiz:\n%s", result), nil
	}

	return fmt.Sprintf("🛡️ Tarama Tamamlandı. Temiz görünüyor.\nRapor:\n%s", result), nil
}