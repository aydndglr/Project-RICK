package system

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
	"time"

	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
)

// --- TOOL 1: EXECUTE COMMAND (Shell) ---
type ExecTool struct{}

func (t *ExecTool) Name() string { return "sys_exec" }
func (t *ExecTool) Description() string {
	return "Sistem terminalinde komut çalıştırır. Uzun sürecek (örn: sunucu başlatma) işlemler için 'start_task' kullan, bu komut işini bitirip geri dönmek zorundadır."
}
func (t *ExecTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command":  map[string]interface{}{"type": "string", "description": "Terminal komutu (örn: 'ipconfig', 'ls -la')."},
			"work_dir": map[string]interface{}{"type": "string", "description": "Komutun çalıştırılacağı dizin (Opsiyonel. Boş bırakılırsa uygulamanın dizininde çalışır)."},
			"timeout":  map[string]interface{}{"type": "integer", "description": "Saniye cinsinden zaman aşımı (Opsiyonel, varsayılan 60)."},
		},
		"required": []string{"command"},
	}
}
func (t *ExecTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	cmdStr, _ := args["command"].(string)

	// Zaman Aşımı (Timeout) Ayarı
	timeoutSec := 60 // Varsayılan 60 saniye
	if val, ok := args["timeout"].(float64); ok {
		timeoutSec = int(val)
	}
	
	// Rick için Guardrail: Maksimum 5 dakika bekleyebilir, sonrası start_task'a girmeli.
	if timeoutSec > 300 {
		timeoutSec = 300
	}

	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(execCtx, "powershell", "-Command", cmdStr)
	} else {
		cmd = exec.CommandContext(execCtx, "bash", "-c", cmdStr)
	}

	// Çalışma Dizini (Work Dir) Ayarı
	if workDir, ok := args["work_dir"].(string); ok && workDir != "" {
		cmd.Dir = workDir
		logger.Action("💻 Terminal (%s): %s", workDir, cmdStr)
	} else {
		logger.Action("💻 Terminal: %s", cmdStr)
	}

	output, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))
	
	// Zaman aşımı yakalama
	if execCtx.Err() == context.DeadlineExceeded {
		return fmt.Sprintf("🛑 UYARI: Komut %d saniye içinde tamamlanamadı ve zorla durduruldu (Timeout). \nEğer bu uzun sürecek bir işlemse 'start_task' aracını kullanmalısın!\nKısmi Çıktı:\n%s", timeoutSec, result), nil
	}

	if err != nil {
		return fmt.Sprintf("⚠️ Komut Hatası (Exit Code): %v\nÇıktı:\n%s", err, result), nil
	}
	
	if result == "" {
		return "✅ Komut çalıştı (Çıktı yok).", nil
	}
	
	// Çıktı çok uzunsa Rick'in jetonlarını tüketmemesi için kırpma
	if len(result) > 4000 {
		result = result[:4000] + "\n\n...[SİSTEM UYARISI: ÇIKTI ÇOK UZUN OLDUĞU İÇİN KESİLDİ. Daha fazlası için çıktıyı bir dosyaya yazdırıp fs_read ile okuyabilirsin]..."
	}
	
	return result, nil
}

// --- TOOL 2: SYSTEM INFO ---
type InfoTool struct{}

func (t *InfoTool) Name() string { return "sys_info" }
func (t *InfoTool) Description() string { return "İşletim sistemi, donanım, mevcut kullanıcı ve ortam değişkenleri hakkında detaylı bilgi verir." }
func (t *InfoTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object", "properties": map[string]interface{}{},
	}
}
func (t *InfoTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	pwd, _ := os.Getwd()
	hostname, _ := os.Hostname()
	
	userInfo := "Bilinmiyor"
	if u, err := user.Current(); err == nil {
		userInfo = fmt.Sprintf("%s (UID: %s, Home: %s)", u.Username, u.Uid, u.HomeDir)
	}

	// Rick'in ihtiyaç duyabileceği kritik ortam değişkenleri
	envVars := []string{"PATH", "GOPATH", "USERPROFILE", "HOME"}
	var envStr strings.Builder
	for _, e := range envVars {
		if val := os.Getenv(e); val != "" {
			envStr.WriteString(fmt.Sprintf("%s=%s\n", e, val))
		}
	}

	return fmt.Sprintf(
		"🖥️ SİSTEM BİLGİSİ (TELEMETRİ):\n" +
		"---------------------------------\n" +
		"OS / Mimari    : %s / %s\n" +
		"CPU Çekirdek   : %d\n" +
		"Go Sürümü      : %s\n" +
		"Hostname       : %s\n" +
		"Kullanıcı      : %s\n" +
		"Çalışma Dizini : %s\n" +
		"---------------------------------\n" +
		"KRİTİK ORTAM DEĞİŞKENLERİ:\n%s",
		runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), runtime.Version(),
		hostname, userInfo, pwd, envStr.String(),
	), nil
}