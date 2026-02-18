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
	PID        int    // YENİ: İşletim Sistemi Süreç Kimliği
	Status     string // "running", "completed", "failed", "killed"
	Output     *SafeBuffer
	StartTime  time.Time
	EndTime    time.Time
	CancelFunc context.CancelFunc // İptal butonu (Tetiği çeken fonksiyon)
	Error      error
}

type TaskManager struct {
	Tasks map[string]*Task // TaskID -> Task
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

// IsRickProcess: Verilen PID'nin Rick tarafından başlatılıp başlatılmadığını kontrol eder.
func (tm *TaskManager) IsRickProcess(pid int) (bool, string) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	for _, task := range tm.Tasks {
		if task.Status == "running" && task.PID == pid {
			return true, task.ID
		}
	}
	return false, ""
}

// --- 1. START BACKGROUND TASK ---

type StartBackgroundTaskCommand struct{}

func (c *StartBackgroundTaskCommand) Name() string { return "start_task" }

func (c *StartBackgroundTaskCommand) Description() string {
	return "Arka planda uzun süreli bir işlem başlatır. PID takibi yapar."
}

func (c *StartBackgroundTaskCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Çalıştırılacak komut (örn: 'python server.py', 'ping google.com -t').",
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

	cmdCtx, cancel := context.WithCancel(context.Background())

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(cmdCtx, "powershell", "-Command", cmdStr)
	} else {
		cmd = exec.CommandContext(cmdCtx, "bash", "-c", cmdStr)
	}

	outBuf := &SafeBuffer{}
	cmd.Stdout = outBuf
	cmd.Stderr = outBuf

	// Komutu başlat ama bekleme
	if err := cmd.Start(); err != nil {
		cancel()
		return "", fmt.Errorf("görev başlatılamadı: %v", err)
	}

	// YENİ: PID Yakalama
	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}

	task := &Task{
		ID:         taskID,
		Command:    cmdStr,
		PID:        pid, // PID Kaydedildi
		Status:     "running",
		StartTime:  time.Now(),
		Output:     outBuf,
		CancelFunc: cancel,
	}

	tm.mu.Lock()
	tm.Tasks[taskID] = task
	tm.mu.Unlock()

	go func() {
		err := cmd.Wait()
		tm.mu.Lock()
		defer tm.mu.Unlock()
		task.EndTime = time.Now()

		if cmdCtx.Err() == context.Canceled {
			task.Status = "killed"
			task.Output.Write([]byte("\n[SİSTEM]: Görev Rick tarafından durduruldu."))
		} else if err != nil {
			task.Status = "failed"
			task.Error = err
			task.Output.Write([]byte(fmt.Sprintf("\n[SİSTEM]: Hata oluştu: %v", err)))
		} else {
			task.Status = "completed"
		}
	}()

	return fmt.Sprintf("🚀 Görev Başlatıldı!\n🆔 ID: %s\n🔢 PID: %d\nKomut: %s", taskID, pid, cmdStr), nil
}

// --- 2. CHECK TASK STATUS ---

type CheckTaskCommand struct{}

func (c *CheckTaskCommand) Name() string { return "check_task" }
func (c *CheckTaskCommand) Description() string { return "Rick'in başlattığı görevlerin durumunu kontrol eder." }
func (c *CheckTaskCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task_id": map[string]interface{}{"type": "string", "description": "ID boş bırakılırsa tüm görevler listelenir."},
		},
	}
}

func (c *CheckTaskCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	taskID, _ := args["task_id"].(string)
	if taskID == "" {
		return listAllRickTasks()
	}

	tm := GetTaskManager()
	tm.mu.RLock()
	task, exists := tm.Tasks[taskID]
	tm.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("görev bulunamadı: %s", taskID)
	}

	output := task.Output.String()
	if len(output) > 1000 {
		output = "...(önceki loglar)...\n" + output[len(output)-1000:]
	}

	statusIcon := "⏳"
	if task.Status == "completed" { statusIcon = "✅" }
	if task.Status == "failed" { statusIcon = "❌" }
	if task.Status == "killed" { statusIcon = "🛑" }

	return fmt.Sprintf("%s GÖREV DETAYI (%s)\nPID: %d\nDurum: %s\nLog:\n%s", statusIcon, task.ID, task.PID, strings.ToUpper(task.Status), output), nil
}

// --- 3. KILL TASK (SMART) ---

type KillTaskCommand struct{}

func (c *KillTaskCommand) Name() string { return "kill_task" }
func (c *KillTaskCommand) Description() string { return "Rick'in başlattığı bir görevi ID ile durdurur." }
func (c *KillTaskCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task_id": map[string]interface{}{"type": "string", "description": "Durdurulacak Görev ID'si."},
		},
		"required": []string{"task_id"},
	}
}

func (c *KillTaskCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	taskID, _ := args["task_id"].(string)
	tm := GetTaskManager()
	tm.mu.Lock()
	task, exists := tm.Tasks[taskID]
	if !exists {
		tm.mu.Unlock()
		return "", fmt.Errorf("bu ID ile aktif bir görevim yok.")
	}
	
	if task.Status == "running" && task.CancelFunc != nil {
		task.CancelFunc() // Context Cancel -> Process Kill
		task.Status = "killed"
		task.EndTime = time.Now()
		tm.mu.Unlock()
		return fmt.Sprintf("🛑 Görev (PID: %d) başarıyla sonlandırıldı.", task.PID), nil
	}
	tm.mu.Unlock()
	return fmt.Sprintf("⚠️ Görev zaten aktif değil: %s", task.Status), nil
}

// --- 4. YENİ: LIST SYSTEM PROCESSES (GÖZLEM YETENEĞİ) ---

type ListSystemProcessesCommand struct{}

func (c *ListSystemProcessesCommand) Name() string { return "list_processes" }

func (c *ListSystemProcessesCommand) Description() string {
	return "Sistemde çalışan programları listeler. Hangi programın Rick'e, hangisinin sisteme/kullanıcıya ait olduğunu gösterir."
}

func (c *ListSystemProcessesCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"filter": map[string]interface{}{
				"type": "string", 
				"description": "Program adı filtresi (örn: 'chrome', 'python'). Boş bırakılırsa hepsini (ilk 50) getirir.",
			},
		},
	}
}

func (c *ListSystemProcessesCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	filter, _ := args["filter"].(string)
	filter = strings.ToLower(filter)

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// CSV formatında al: "Image Name","PID","Session Name","Session#","Mem Usage"
		cmd = exec.Command("tasklist", "/FO", "CSV", "/NH")
	} else {
		// Linux/Mac: pid, command
		cmd = exec.Command("ps", "-e", "-o", "pid,comm")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("süreç listesi alınamadı: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	var sb strings.Builder
	tm := GetTaskManager()

	sb.WriteString(fmt.Sprintf("🖥️ SİSTEM SÜREÇLERİ (Filtre: '%s')\n", filter))
	sb.WriteString("PID    | SAHİBİ        | PROGRAM\n")
	sb.WriteString("-------|---------------|------------------\n")

	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" { continue }

		var pid int
		var name string

		// Basit parsing (Windows/Linux ayrımı)
		if runtime.GOOS == "windows" {
			parts := strings.Split(line, "\",\"")
			if len(parts) >= 2 {
				name = strings.Trim(parts[0], "\"")
				fmt.Sscanf(strings.Trim(parts[1], "\""), "%d", &pid)
			}
		} else {
			fmt.Sscanf(strings.TrimSpace(line), "%d %s", &pid, &name)
		}

		// Filtreleme
		if filter != "" && !strings.Contains(strings.ToLower(name), filter) {
			continue
		}

		// SAHİPLİK KONTROLÜ
		owner := "SİSTEM/KULLANICI"
		isMine, taskID := tm.IsRickProcess(pid)
		if isMine {
			owner = fmt.Sprintf("RICK (%s)", taskID)
		}

		// Sadece ilk 30 sonucu veya filtrelenenleri göster
		if count < 30 || filter != "" {
			sb.WriteString(fmt.Sprintf("%-6d | %-13s | %s\n", pid, owner, name))
		}
		count++
	}

	if count > 30 && filter == "" {
		sb.WriteString("... (ve daha fazlası. Tam liste için filtre kullanın)\n")
	}

	return sb.String(), nil
}

// Helper
func listAllRickTasks() (string, error) {
	tm := GetTaskManager()
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if len(tm.Tasks) == 0 {
		return "📭 Rick'e ait aktif görev yok.", nil
	}

	var sb strings.Builder
	sb.WriteString("🤖 RICK'İN GÖREVLERİ:\n")
	for id, task := range tm.Tasks {
		sb.WriteString(fmt.Sprintf("🆔 %s | PID: %d | Durum: %s | Komut: %s\n", id, task.PID, task.Status, task.Command))
	}
	return sb.String(), nil
}