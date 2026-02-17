package commands

import (
	"bytes"
	"context"
	"fmt"
	"os"
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

type Task struct {
	ID        string
	Command   string
	Status    string // "running", "completed", "failed", "killed"
	Output    bytes.Buffer
	StartTime time.Time
	EndTime   time.Time
	Process   *os.Process
	Error     error
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
	return "Arka planda uzun süreli bir işlem başlatır ve hemen Görev ID döner. Rick sonucu beklemez. Parametre: command."
}

func (c *StartBackgroundTaskCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	cmdStr, ok := args["command"].(string)
	if !ok || cmdStr == "" {
		return "", fmt.Errorf("eksik parametre: command")
	}

	tm := GetTaskManager()
	taskID := fmt.Sprintf("task_%d", time.Now().UnixNano())

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-Command", cmdStr)
	} else {
		cmd = exec.Command("bash", "-c", cmdStr)
	}

	task := &Task{
		ID:        taskID,
		Command:   cmdStr,
		Status:    "running",
		StartTime: time.Now(),
	}

	cmd.Stdout = &task.Output
	cmd.Stderr = &task.Output

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("görev başlatılamadı: %v", err)
	}

	task.Process = cmd.Process

	tm.mu.Lock()
	tm.Tasks[taskID] = task
	tm.mu.Unlock()

	go func() {
		err := cmd.Wait()
		
		tm.mu.Lock()
		defer tm.mu.Unlock()
		
		task.EndTime = time.Now()
		if err != nil {
			task.Status = "failed"
			task.Error = err
			task.Output.WriteString(fmt.Sprintf("\n[SİSTEM]: İşlem hata koduyla bitti: %v", err))
		} else {
			task.Status = "completed"
		}
	}()

	return fmt.Sprintf("🚀 Arka plan görevi başlatıldı!\n🆔 ID: %s\nKomut: %s\nDurumu kontrol etmek için: check_task", taskID, cmdStr), nil
}

// --- 2. CHECK TASK STATUS ---

type CheckTaskCommand struct{}

func (c *CheckTaskCommand) Name() string { return "check_task" }

func (c *CheckTaskCommand) Description() string {
	return "Arka planda çalışan bir görevin durumunu ve çıktısını kontrol eder. Parametre: task_id."
}

func (c *CheckTaskCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	taskID, ok := args["task_id"].(string)
	if !ok {
		return listAllTasks()
	}

	tm := GetTaskManager()
	tm.mu.RLock()
	task, exists := tm.Tasks[taskID]
	tm.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("bu ID ile bir görev bulunamadı: %s", taskID)
	}

	duration := time.Since(task.StartTime)
	if task.Status != "running" {
		duration = task.EndTime.Sub(task.StartTime)
	}

	output := task.Output.String()
	if len(output) > 2000 {
		output = "...(önceki çıktılar kırpıldı)...\n" + output[len(output)-2000:]
	}
	if output == "" {
		output = "(Henüz çıktı yok)"
	}

	statusIcon := "⏳"
	if task.Status == "completed" {
		statusIcon = "✅"
	} else if task.Status == "failed" {
		statusIcon = "❌"
	}

	return fmt.Sprintf(
		"%s GÖREV DURUMU (%s)\n"+
			"----------------------\n"+
			"Komut: %s\n"+
			"Durum: %s\n"+
			"Süre: %s\n"+
			"----------------------\n"+
			"📝 SON ÇIKTI:\n%s",
		statusIcon, task.ID, task.Command, strings.ToUpper(task.Status), duration.Round(time.Second), output,
	), nil
}

// --- 3. KILL TASK COMMAND ---

type KillTaskCommand struct{}

func (c *KillTaskCommand) Name() string { return "kill_task" }

func (c *KillTaskCommand) Description() string {
	return "Çalışan bir arka plan görevini zorla durdurur. Parametre: task_id."
}

func (c *KillTaskCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	taskID, ok := args["task_id"].(string)
	if !ok {
		return "", fmt.Errorf("eksik parametre: task_id")
	}

	tm := GetTaskManager()
	tm.mu.Lock()
	task, exists := tm.Tasks[taskID]
	tm.mu.Unlock()

	if !exists {
		return "", fmt.Errorf("görev bulunamadı")
	}

	if task.Status != "running" {
		return fmt.Sprintf("⚠️ Bu görev zaten bitmiş durumda: %s", task.Status), nil
	}

	if task.Process != nil {
		if err := task.Process.Kill(); err != nil {
			return "", fmt.Errorf("görev durdurulamadı: %v", err)
		}
	}

	tm.mu.Lock()
	task.Status = "killed"
	task.EndTime = time.Now()
	tm.mu.Unlock()

	return fmt.Sprintf("🛑 Görev başarıyla sonlandırıldı: %s", taskID), nil
}

// Helper: List All Tasks
func listAllTasks() (string, error) {
	tm := GetTaskManager()
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if len(tm.Tasks) == 0 {
		return "📭 Şu an kayıtlı hiçbir arka plan görevi yok.", nil
	}

	var sb strings.Builder
	sb.WriteString("📋 TÜM GÖREVLER LİSTESİ:\n")
	for id, task := range tm.Tasks {
		icon := "⏳"
		// DÜZELTİLEN KISIM: Go syntax hatası giderildi (if-else blokları)
		if task.Status == "completed" {
			icon = "✅"
		} else if task.Status == "failed" {
			icon = "❌"
		}
		
		sb.WriteString(fmt.Sprintf("- %s %s | %s | %s\n", icon, id, task.Status, task.Command))
	}
	return sb.String(), nil
}