package system

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
)

type KillTaskTool struct{}

func (t *KillTaskTool) Name() string { return "kill_task" }
func (t *KillTaskTool) Description() string {
	return "Aktif bir görevi veya sarkan bir işletim sistemi sürecini (PID) acımasızca sonlandırır. Zombi süreç bırakmaz."
}

func (t *KillTaskTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task_id": map[string]interface{}{"type": "string", "description": "Durdurulacak Görev ID'si (Örn: task_17000000)."},
			"pid":     map[string]interface{}{"type": "integer", "description": "Eğer Task ID yoksa, doğrudan OS süreç kimliği ile öldür (Sadece sys_exec ile tespit edilen süreçler için)."},
		},
	}
}

func (t *KillTaskTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	taskID, _ := args["task_id"].(string)
	pidFloat, ok := args["pid"].(float64)
	pid := int(pidFloat)

	if taskID == "" && !ok {
		return "", fmt.Errorf("eksik parametre: Lütfen 'task_id' veya 'pid' belirtin")
	}

	tm := GetTaskManager()

	// 1. TASK ID ÜZERİNDEN İPTAL ETME
	if taskID != "" {
		tm.mu.RLock()
		task, exists := tm.Tasks[taskID]
		tm.mu.RUnlock()

		if !exists {
			return "", fmt.Errorf("HATA: Bu ID ile kayıtlı bir görev yok: %s", taskID)
		}

		if task.Status != StatusRunning {
			return fmt.Sprintf("⚠️ Görev zaten aktif değil. Mevcut Durum: %s", task.Status), nil
		}

		// Context'i iptal et (Ana süreci durdurur)
		if task.CancelFunc != nil {
			task.CancelFunc()
		}

		// Acımasız Mod: Alt süreçleri de (Child Processes) öldürmeyi garanti altına al
		if task.PID > 0 {
			forceKillProcessTree(task.PID)
		}

		// Registry'i güncelle ki JSON dosyasına yansısın
		tm.UpdateStatus(taskID, StatusKilled)
		logger.Warn("🛑 Görev ZORLA durduruldu: %s (PID: %d)", taskID, task.PID)

		return fmt.Sprintf("🛑 İşlem Başarılı: %s (PID: %d) ve bağlı tüm alt süreçleri acımasızca sonlandırıldı.", taskID, task.PID), nil
	}

	// 2. DOĞRUDAN PID ÜZERİNDEN İPTAL ETME (Kaçak süreç avcısı)
	if pid > 0 {
		process, err := os.FindProcess(pid)
		if err != nil {
			return "", fmt.Errorf("HATA: %d numaralı süreç bulunamadı: %v", pid, err)
		}

		// Acımasız Mod (İşletim sistemi seviyesinde)
		forceKillProcessTree(pid)
		
		// Fallback (Bulunamazsa direkt Go üzerinden kill)
		process.Kill()
		
		logger.Warn("🛑 Kaçak Süreç ZORLA durduruldu (PID: %d)", pid)
		return fmt.Sprintf("🛑 İşlem Başarılı: İşletim sistemi üzerindeki %d PID numaralı süreç zorla kapatıldı.", pid), nil
	}

	return "", fmt.Errorf("geçersiz işlem parametreleri")
}

// forceKillProcessTree: İşletim sistemine göre süreç ağacını (zombiler dahil) tamamen yok eder.
func forceKillProcessTree(pid int) {
	pidStr := strconv.Itoa(pid)
	if runtime.GOOS == "windows" {
		// Windows: /T flag'i süreç ağacındaki (Tree) tüm alt işlemleri öldürür
		exec.Command("taskkill", "/F", "/T", "/PID", pidStr).Run()
	} else {
		// Linux/Mac: Negatif PID ile süreç grubunu öldür (Process Group)
		// Yada doğrudan kill -9
		exec.Command("kill", "-9", pidStr).Run()
	}
}