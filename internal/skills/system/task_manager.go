package system

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Görev Durumları
const (
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusKilled    = "killed"
)

// Task: Arka planda koşan sürecin dijital kimliği
type Task struct {
	ID         string             `json:"id"`
	Command    string             `json:"command"`
	PID        int                `json:"pid"`
	Status     string             `json:"status"`
	LogPath    string             `json:"log_path"`
	StartedAt  time.Time          `json:"started_at"`
	FinishedAt time.Time          `json:"finished_at,omitempty"`
	CancelFunc context.CancelFunc `json:"-"` // Bellekte tutulur, JSON'da saklanmaz
}

// TaskManager: Rick'in süreç fabrikası ve hafızası
type TaskManager struct {
	Tasks        map[string]*Task
	LogDir       string
	RegistryPath string
	mu           sync.RWMutex
}

var (
	tmInstance *TaskManager
	once       sync.Once
)

// GetTaskManager: Singleton örneğini kurar ve sistem dizinlerini hazırlar
func GetTaskManager() *TaskManager {
	once.Do(func() {
		logDir := filepath.Join("logs", "tasks")
		os.MkdirAll(logDir, 0755)
		
		tmInstance = &TaskManager{
			Tasks:        make(map[string]*Task),
			LogDir:       logDir,
			RegistryPath: filepath.Join(logDir, "task_registry.json"),
		}
		tmInstance.LoadRegistry() // Başlangıçta eski görevleri hatırla
	})
	return tmInstance
}

// AddTask: Yeni bir süreci sisteme kaydeder
func (tm *TaskManager) AddTask(id, cmd string, pid int, cancel context.CancelFunc) *Task {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task := &Task{
		ID:         id,
		Command:    cmd,
		PID:        pid,
		Status:     StatusRunning,
		LogPath:    filepath.Join(tm.LogDir, fmt.Sprintf("%s.log", id)),
		StartedAt:  time.Now(),
		CancelFunc: cancel,
	}

	tm.Tasks[id] = task
	tm.saveRegistry()
	return task
}

// UpdateStatus: Görev durumunu günceller ve bitiş zamanını damgalar
func (tm *TaskManager) UpdateStatus(id, status string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if task, ok := tm.Tasks[id]; ok {
		task.Status = status
		if status != StatusRunning {
			task.FinishedAt = time.Now()
		}
		tm.saveRegistry()
	}
}

// GetLogContent: Log dosyasını yormadan sadece son kısmını (Tail) okur
func (tm *TaskManager) GetLogContent(id string, limitBytes int64) (string, error) {
	tm.mu.RLock()
	task, ok := tm.Tasks[id]
	tm.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("görev bulunamadı: %s", id)
	}

	file, err := os.Open(task.LogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "⚠️ Log dosyası henüz oluşmadı veya boş.", nil
		}
		return "", err
	}
	defer file.Close()

	stat, _ := file.Stat()
	if stat.Size() <= limitBytes {
		content, _ := io.ReadAll(file)
		return string(content), nil
	}

	// Dosyanın sonundan 'limitBytes' kadar geri git (Smart Tailing)
	buffer := make([]byte, limitBytes)
	_, err = file.ReadAt(buffer, stat.Size()-limitBytes)
	if err != nil && err != io.EOF {
		return "", err
	}

	return "...(önceki loglar diskte saklanıyor)...\n" + string(buffer), nil
}

// saveRegistry: Mevcut görev listesini JSON olarak yedekler
func (tm *TaskManager) saveRegistry() {
	data, _ := json.MarshalIndent(tm.Tasks, "", "  ")
	os.WriteFile(tm.RegistryPath, data, 0644)
}

// LoadRegistry: Rick uyandığında eski (zombie) görevleri tespit eder
func (tm *TaskManager) LoadRegistry() {
	data, err := os.ReadFile(tm.RegistryPath)
	if err == nil {
		json.Unmarshal(data, &tm.Tasks)
		// Rick kapandığında 'running' kalan süreçleri 'interrupted' olarak işaretle
		for _, t := range tm.Tasks {
			if t.Status == StatusRunning {
				t.Status = "interrupted"
			}
		}
	}
}