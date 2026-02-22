package system

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
)

type ScheduleTaskTool struct{}

func (t *ScheduleTaskTool) Name() string { return "schedule_task" }

func (t *ScheduleTaskTool) Description() string {
	return "İleri tarihli veya gecikmeli komutlar için (örn: sistemi kapatma, belirli bir saatte script çalıştırma) işletim sistemine özel (.bat veya .sh) zamanlanmış görev betiği oluşturur ve arka planda tetikler. Rick kapansa bile bu görev OS seviyesinde çalışır."
}

func (t *ScheduleTaskTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type": "string", 
				"description": "Zamanı gelince çalıştırılacak asıl komut (Örn: 'shutdown /s /t 0' veya 'python script.py')",
			},
			"delay_minutes": map[string]interface{}{
				"type": "integer", 
				"description": "Kaç dakika sonra çalıştırılacağı (Örn: 10). Eğer 'time_hhmm' kullanılıyorsa boş bırak.",
			},
			"time_hhmm": map[string]interface{}{
				"type": "string", 
				"description": "Belirli bir saatte çalıştırmak için (Örn: '14:30' veya '03:00'). Eğer 'delay_minutes' kullanılıyorsa boş bırak.",
			},
			"task_name": map[string]interface{}{
				"type": "string", 
				"description": "Oluşturulacak script dosyasının adı (Örn: 'gece_yedekleme'). Uzantı yazma.",
			},
		},
		"required": []string{"command", "task_name"},
	}
}

func (t *ScheduleTaskTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	command, ok := args["command"].(string)
	if !ok || command == "" {
		return "", fmt.Errorf("HATA: Çalıştırılacak 'command' belirtilmedi")
	}

	taskName, _ := args["task_name"].(string)
	if taskName == "" {
		taskName = fmt.Sprintf("rick_task_%d", time.Now().Unix())
	}
	taskName = strings.ReplaceAll(taskName, " ", "_") // Boşlukları temizle

	// 1. Gecikme (Delay) Süresini Saniye Cinsinden Hesapla
	var delaySeconds int

	if delayFloat, ok := args["delay_minutes"].(float64); ok && delayFloat > 0 {
		delaySeconds = int(delayFloat * 60)
	} else if timeStr, ok := args["time_hhmm"].(string); ok && timeStr != "" {
		// "15:30" formatını parse et
		now := time.Now()
		targetTime, err := time.Parse("15:04", timeStr)
		if err != nil {
			return "", fmt.Errorf("HATA: 'time_hhmm' formatı yanlış. '15:30' şeklinde olmalı")
		}

		// Hedef saati bugünün tarihiyle birleştir
		target := time.Date(now.Year(), now.Month(), now.Day(), targetTime.Hour(), targetTime.Minute(), 0, 0, now.Location())

		// Eğer saat geçmişse, yarına kur
		if target.Before(now) {
			target = target.Add(24 * time.Hour)
		}

		delaySeconds = int(target.Sub(now).Seconds())
	} else {
		return "", fmt.Errorf("HATA: Lütfen ya 'delay_minutes' ya da 'time_hhmm' parametresini belirtin")
	}

	// Klasör oluştur
	scheduleDir := ".rick_schedules"
	os.MkdirAll(scheduleDir, 0755)

	var scriptPath string
	var runCmd *exec.Cmd

	// 2. İşletim Sistemine Göre Script Üretimi
	if runtime.GOOS == "windows" {
		scriptPath = filepath.Join(scheduleDir, taskName+".bat")
		
		batContent := fmt.Sprintf(`@echo off
echo ==========================================
echo RICK OS - ZAMANLANMIS GOREV: %s
echo ==========================================
echo %d saniye bekleniyor...
timeout /t %d /nobreak > NUL
echo Gorev baslatiliyor: %s
%s
`, taskName, delaySeconds, delaySeconds, command, command)

		if err := os.WriteFile(scriptPath, []byte(batContent), 0644); err != nil {
			return "", fmt.Errorf("Bat dosyası oluşturulamadı: %v", err)
		}

		// Windows'ta arka planda kopararak çalıştırma (start /b)
		runCmd = exec.Command("cmd", "/C", "start", "/b", scriptPath)
		logger.Action("⏳ Windows Zamanlanmış Görevi Kuruldu: %s (%d sn sonra)", scriptPath, delaySeconds)

	} else {
		// Linux / macOS için
		scriptPath = filepath.Join(scheduleDir, taskName+".sh")
		
		shContent := fmt.Sprintf(`#!/bin/bash
echo "=========================================="
echo "RICK OS - ZAMANLANMIS GOREV: %s"
echo "=========================================="
echo "%d saniye bekleniyor..."
sleep %d
echo "Gorev baslatiliyor: %s"
%s
`, taskName, delaySeconds, delaySeconds, command, command)

		if err := os.WriteFile(scriptPath, []byte(shContent), 0755); err != nil { // 0755: Çalıştırma izni
			return "", fmt.Errorf("Shell script oluşturulamadı: %v", err)
		}

		// Linux'ta arka planda kopararak çalıştırma (nohup)
		runCmd = exec.Command("nohup", scriptPath, "&")
		logger.Action("⏳ Linux/Unix Zamanlanmış Görevi Kuruldu: %s (%d sn sonra)", scriptPath, delaySeconds)
	}

	// 3. Görevi İşletim Sistemine Emanet Et (Start ile arka plana atıyoruz, Wait YAPMIYORUZ)
	if err := runCmd.Start(); err != nil {
		return "", fmt.Errorf("Görev arka planda başlatılamadı: %v", err)
	}

	// Rapor döndür
	var infoMsg string
	if delaySeconds >= 3600 {
		infoMsg = fmt.Sprintf("%.1f saat", float64(delaySeconds)/3600.0)
	} else if delaySeconds >= 60 {
		infoMsg = fmt.Sprintf("%.1f dakika", float64(delaySeconds)/60.0)
	} else {
		infoMsg = fmt.Sprintf("%d saniye", delaySeconds)
	}

	return fmt.Sprintf("✅ BAŞARILI: '%s' görevi işletim sistemine emanet edildi.\n📜 Script Yolu: %s\n⏳ Bekleme Süresi: %s sonra tetiklenecek.\n⚙️ Komut: %s", taskName, scriptPath, infoMsg, command), nil
}