package system

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

type CheckTaskTool struct{}

func (t *CheckTaskTool) Name() string { return "check_task" }
func (t *CheckTaskTool) Description() string {
	return "Arka planda çalışan veya biten bir görevin durumunu, CANLI KAYNAK TÜKETİMİNİ (CPU/RAM) ve loglarını kontrol eder. Task ID verilmezse tüm görevleri listeler."
}
func (t *CheckTaskTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task_id": map[string]interface{}{"type": "string", "description": "Kontrol edilecek görev ID'si (Örn: task_17000000)."},
		},
	}
}

func (t *CheckTaskTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	taskID, _ := args["task_id"].(string)
	tm := GetTaskManager()

	// 1. TÜM GÖREVLERİ LİSTELEME MODU
	if taskID == "" {
		tm.mu.RLock()
		defer tm.mu.RUnlock()
		
		if len(tm.Tasks) == 0 {
			return "📭 Sistemde kayıtlı herhangi bir görev bulunamadı.", nil
		}

		var sb strings.Builder
		sb.WriteString("🤖 GÖREV KAYIT DEFTERİ:\n")
		sb.WriteString(strings.Repeat("-", 70) + "\n")
		
		for id, task := range tm.Tasks {
			duration := time.Since(task.StartedAt).Round(time.Second).String()
			if task.Status != StatusRunning {
				duration = task.FinishedAt.Sub(task.StartedAt).Round(time.Second).String()
			}
			
			statusIcon := "⏳"
			switch task.Status {
			case StatusCompleted: statusIcon = "✅"
			case StatusFailed:    statusIcon = "❌"
			case StatusKilled:    statusIcon = "🛑"
			}

			sb.WriteString(fmt.Sprintf("%s ID: %-15s | %-10s | Süre: %-8s | Komut: %s\n", 
				statusIcon, id, strings.ToUpper(task.Status), duration, task.Command))
		}
		return sb.String(), nil
	}

	// 2. TEKİL GÖREV DETAYI MODU
	tm.mu.RLock()
	task, exists := tm.Tasks[taskID]
	tm.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("HATA: '%s' ID'li bir görev bulunamadı", taskID)
	}

	// 🚀 CANLI KAYNAK TÜKETİMİ (CPU / RAM) HESAPLAMA
	var resourceInfo string
	if task.Status == StatusRunning && task.PID > 0 {
		p, err := process.NewProcess(int32(task.PID))
		if err == nil {
			cpuPercent, _ := p.CPUPercent()
			memInfo, _ := p.MemoryInfo()
			
			var memUsageMB float64
			if memInfo != nil {
				memUsageMB = float64(memInfo.RSS) / 1024 / 1024
			}
			
			resourceInfo = fmt.Sprintf("\n🔹 CPU Tüketimi: %.2f%%\n🔹 RAM Tüketimi: %.2f MB", cpuPercent, memUsageMB)
		} else {
			resourceInfo = "\n🔹 Kaynak Tüketimi: (Süreç bilgisi işletim sisteminden okunamadı)"
		}
	}

	// Logları diskten "Smart Tail" yöntemiyle çek (Son 4096 byte)
	logContent, err := tm.GetLogContent(taskID, 4096)
	if err != nil {
		logContent = fmt.Sprintf("⚠️ Loglar okunamadı: %v", err)
	}

	duration := time.Since(task.StartedAt).Round(time.Second).String()
	finishInfo := "Halen Çalışıyor..."
	if task.Status != StatusRunning {
		duration = task.FinishedAt.Sub(task.StartedAt).Round(time.Second).String()
		finishInfo = task.FinishedAt.Format("15:04:05")
	}

	res := fmt.Sprintf("📋 GÖREV RAPORU: %s\n", task.ID)
	res += strings.Repeat("=", 40) + "\n"
	res += fmt.Sprintf("🔹 Durum    : %s\n", strings.ToUpper(task.Status))
	res += fmt.Sprintf("🔹 PID      : %d\n", task.PID)
	res += fmt.Sprintf("🔹 Başlangıç: %s\n", task.StartedAt.Format("15:04:05"))
	res += fmt.Sprintf("🔹 Bitiş    : %s\n", finishInfo)
	res += fmt.Sprintf("🔹 Toplam   : %s", duration)
	res += resourceInfo + "\n" // Eklenen kaynak tüketimi verisi
	res += fmt.Sprintf("🔹 Komut    : %s\n", task.Command)
	res += strings.Repeat("-", 40) + "\n"
	res += fmt.Sprintf("📄 SON LOGLAR:\n%s\n", logContent)

	return res, nil
}