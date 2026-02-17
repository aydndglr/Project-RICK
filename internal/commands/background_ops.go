package commands

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// --- GLOBAL TASK MANAGER (İşlem Merkezi) ---
var (
	tmInstance *TaskManager
	once       sync.Once
)

// SafeBuffer: Eşzamanlı okuma/yazma için thread-safe buffer
type SafeBuffer struct {
	b  bytes.Buffer
	mu sync.RWMutex
}

func (sb *SafeBuffer) Write(p []byte) (n int, err error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.b.Write(p)
}

func (sb *SafeBuffer) String() string {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.b.String()
}

type Task struct {
	ID         string
	Command    string
	Status     string // "running", "completed", "failed", "killed"
	Output     *SafeBuffer
	StartTime  time.Time
	EndTime    time.Time
	CancelFunc context.CancelFunc // İptal butonu (Tetiği çeken fonksiyon)
	Error      error
}

type TaskManager struct {
	Tasks map[string]*Task
	mu    sync.RWMutex
}

func GetTaskManager() *TaskManager {
	once.Do(func() {
		tmInstance = &TaskManager{
			Tasks: make(map[string]*Task),
		}
	})
	return tmInstance
}

// --- 1. START BACKGROUND TASK ---

type StartBackgroundTaskCommand struct{}

func (c *StartBackgroundTaskCommand) Name() string { return "start_task" }

func (c *StartBackgroundTaskCommand) Description() string {
	return "Arka planda uzun süreli bir işlem başlatır. ID döner, Rick sonucu beklemez."
}

func (c *StartBackgroundTaskCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Arka planda çalıştırılacak uzun soluklu komut (örn: 'ping google.com -t', 'ffmpeg -i...').",
			},
		},
		"required": []string{"command"},
	}
}

func (c *StartBackgroundTaskCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	cmdStr, ok := args["command"].(string)
	if !ok || cmdStr == "" {
		return "", fmt.Errorf("eksik parametre: command")
	}

	tm := GetTaskManager()
	taskID := fmt.Sprintf("task_%d", time.Now().UnixNano())

	// Context ile İptal Edilebilir Komut Oluşturma
	cmdCtx, cancel := context.WithCancel(context.Background())

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(cmdCtx, "powershell", "-Command", cmdStr)
	} else {
		cmd = exec.CommandContext(cmdCtx, "bash", "-c", cmdStr)
	}

	// Thread-safe buffer kullanıyoruz
	outBuf := &SafeBuffer{}
	cmd.Stdout = outBuf
	cmd.Stderr = outBuf

	task := &Task{
		ID:         taskID,
		Command:    cmdStr,
		Status:     "running",
		StartTime:  time.Now(),
		Output:     outBuf,
		CancelFunc: cancel,
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return "", fmt.Errorf("görev başlatılamadı: %v", err)
	}

	// Görevi listeye ekle
	tm.mu.Lock()
	tm.Tasks[taskID] = task
	tm.mu.Unlock()

	// Arka Plan İzleyicisi (Goroutine)
	go func() {
		err := cmd.Wait()

		tm.mu.Lock()
		defer tm.mu.Unlock()

		task.EndTime = time.Now()

		if cmdCtx.Err() == context.Canceled {
			task.Status = "killed"
			task.Output.Write([]byte("\n[SİSTEM]: Görev kullanıcı tarafından iptal edildi."))
		} else if err != nil {
			task.Status = "failed"
			task.Error = err
			task.Output.Write([]byte(fmt.Sprintf("\n[SİSTEM]: Hata oluştu: %v", err)))
		} else {
			task.Status = "completed"
		}
	}()

	return fmt.Sprintf("🚀 Arka plan görevi başlatıldı! (Asenkron Mod)\n🆔 ID: %s\nKomut: %s\nBen diğer işlerine bakabilirim, durum için: check_task", taskID, cmdStr), nil
}

// --- 2. CHECK TASK STATUS ---

type CheckTaskCommand struct{}

func (c *CheckTaskCommand) Name() string { return "check_task" }

func (c *CheckTaskCommand) Description() string {
	return "Arka plan görevini kontrol eder veya tüm görevleri listeler."
}

func (c *CheckTaskCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "Kontrol edilecek görevin ID'si. Boş bırakılırsa tüm görevleri listeler.",
			},
		},
		// required alanı boş, çünkü task_id opsiyonel
	}
}

func (c *CheckTaskCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	taskID, ok := args["task_id"].(string)
	if !ok || taskID == "" {
		return listAllTasks()
	}

	tm := GetTaskManager()
	tm.mu.RLock()
	task, exists := tm.Tasks[taskID]
	tm.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("bu ID ile görev bulunamadı: %s", taskID)
	}

	// Süre Hesapla
	duration := time.Since(task.StartTime)
	if task.Status != "running" {
		duration = task.EndTime.Sub(task.StartTime)
	}

	// Çıktıyı Güvenli Al
	output := task.Output.String()
	
	// Çok uzun çıktıları kırp
	if len(output) > 1500 {
		output = "...(önceki çıktılar)...\n" + output[len(output)-1500:]
	}
	if output == "" {
		output = "(Henüz çıktı yok veya işlem sessiz çalışıyor)"
	}

	statusIcon := "⏳"
	if task.Status == "completed" {
		statusIcon = "✅"
	} else if task.Status == "failed" {
		statusIcon = "❌"
	} else if task.Status == "killed" {
		statusIcon = "🛑"
	}

	return fmt.Sprintf(
		"%s GÖREV RAPORU (%s)\n"+
			"----------------------\n"+
			"Komut: %s\n"+
			"Durum: %s\n"+
			"Süre: %s\n"+
			"----------------------\n"+
			"📝 CANLI ÇIKTI:\n%s",
		statusIcon, task.ID, task.Command, strings.ToUpper(task.Status), duration.Round(time.Second), output,
	), nil
}

// --- 3. KILL TASK COMMAND ---

type KillTaskCommand struct{}

func (c *KillTaskCommand) Name() string { return "kill_task" }

func (c *KillTaskCommand) Description() string {
	return "Arka plan görevini İPTAL EDER."
}

func (c *KillTaskCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "Durdurulacak görevin ID'si.",
			},
		},
		"required": []string{"task_id"},
	}
}

func (c *KillTaskCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	taskID, ok := args["task_id"].(string)
	if !ok {
		return "", fmt.Errorf("eksik parametre: task_id")
	}

	tm := GetTaskManager()
	tm.mu.Lock()
	task, exists := tm.Tasks[taskID]
	
	if !exists {
		tm.mu.Unlock()
		return "", fmt.Errorf("görev bulunamadı")
	}

	if task.Status != "running" {
		tm.mu.Unlock()
		return fmt.Sprintf("⚠️ Bu görev zaten aktif değil: %s", task.Status), nil
	}

	// 1. Context Cancel Fonksiyonunu Çağır
	if task.CancelFunc != nil {
		task.CancelFunc()
	}

	// 2. Durumu güncelle
	task.Status = "killed"
	task.EndTime = time.Now()
	
	tm.mu.Unlock()

	return fmt.Sprintf("🛑 Görev iptal sinyali gönderildi: %s\nRick: 'Emrin üzerine işlemi durdurdum patron.'", taskID), nil
}

// Helper: List All Tasks
func listAllTasks() (string, error) {
	tm := GetTaskManager()
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if len(tm.Tasks) == 0 {
		return "📭 Şu an arka planda çalışan veya bitmiş görev yok.", nil
	}

	var sb strings.Builder
	sb.WriteString("📋 GÖREV YÖNETİCİSİ:\n")
	for id, task := range tm.Tasks {
		icon := "⏳"
		if task.Status == "completed" {
			icon = "✅"
		} else if task.Status == "failed" {
			icon = "❌"
		} else if task.Status == "killed" {
			icon = "🛑"
		}
		
		shortID := id
		if len(id) > 15 { shortID = "..." + id[len(id)-10:] }

		sb.WriteString(fmt.Sprintf("- %s %s | %s\n", icon, shortID, task.Status))
	}
	return sb.String(), nil
}