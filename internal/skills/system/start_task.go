package system

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
)

type StartTaskTool struct{}

func (t *StartTaskTool) Name() string { return "start_task" }
func (t *StartTaskTool) Description() string {
	return "Arka planda uzun süren bir terminal komutu başlatır. Logları doğrudan diske yazar, RAM tüketmez."
}

func (t *StartTaskTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command":  map[string]interface{}{"type": "string", "description": "Çalıştırılacak komut (örn: 'go build', 'npm install')."},
			"work_dir": map[string]interface{}{"type": "string", "description": "Komutun çalıştırılacağı klasör (Opsiyonel)."},
		},
		"required": []string{"command"},
	}
}

func (t *StartTaskTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	cmdStr, _ := args["command"].(string)
	workDir, _ := args["work_dir"].(string)
	
	if cmdStr == "" {
		return "", fmt.Errorf("eksik parametre: command")
	}

	tm := GetTaskManager()
	// Benzersiz ID üretimi
	taskID := fmt.Sprintf("task_%d", time.Now().UnixNano())

	// 1. Context ve Komut Hazırlığı
	// Background context kullanıyoruz çünkü araç bittikten sonra da süreç yaşamalı.
	cmdCtx, cancel := context.WithCancel(context.Background())
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(cmdCtx, "powershell", "-Command", cmdStr)
	} else {
		cmd = exec.CommandContext(cmdCtx, "bash", "-c", cmdStr)
	}

	if workDir != "" {
		cmd.Dir = workDir
	}

	// 2. 📂 LOG DOSYASINI AÇ (Pipeline Kurulumu)
	logPath := filepath.Join(tm.LogDir, taskID+".log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		cancel()
		return "", fmt.Errorf("log dosyası hazırlanamadı: %v", err)
	}

	// Rick'in en büyük silahı: stdout ve stderr'i doğrudan dosyaya bağla!
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// 3. SÜRECİ BAŞLAT
	if err := cmd.Start(); err != nil {
		logFile.Close()
		cancel()
		return "", fmt.Errorf("görev başlatılamadı: %v", err)
	}

	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}

	// 4. Registry'e ekle (Persistency)
	tm.AddTask(taskID, cmdStr, pid, cancel)
	logger.Action("🚀 Arka Plan Görevi Yayında [%s]: %s", taskID, cmdStr)

	// 5. Arka Planda Takip ve Dosya Kapatma
	go func() {
		// Görev bittiğinde işletim sistemi seviyesindeki dosya handle'ını kapat
		defer logFile.Close() 
		
		err := cmd.Wait()
		
		status := StatusCompleted
		timestamp := time.Now().Format("15:04:05")

		if cmdCtx.Err() == context.Canceled {
			status = StatusKilled
			fmt.Fprintf(logFile, "\n[SİSTEM - %s]: Görev Rick tarafından durduruldu (SIGKILL).\n", timestamp)
		} else if err != nil {
			status = StatusFailed
			fmt.Fprintf(logFile, "\n[SİSTEM - %s]: Görev hata verdi: %v\n", timestamp, err)
		} else {
			fmt.Fprintf(logFile, "\n[SİSTEM - %s]: Görev başarıyla sonuçlandı.\n", timestamp)
		}

		// Kayıt defterini güncelle
		tm.UpdateStatus(taskID, status)
		logger.Info("Görev bitti: %s", taskID)
	}()

	return fmt.Sprintf("🚀 Görev Arka Planda Başlatıldı! \n🆔 ID: %s \n🔢 PID: %d \n📂 Log: %s \n\nDurumu izlemek için 'check_task' aracını kullan.", taskID, pid, logPath), nil
}