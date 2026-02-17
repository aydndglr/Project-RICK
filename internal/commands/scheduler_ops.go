package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/aydndglr/rick-agent/pkg/logger"
)

// --- ALARM VERİ YAPISI ---
type Reminder struct {
	ID        string    `json:"id"`
	Time      time.Time `json:"time"`
	Message   string    `json:"message"`
	Completed bool      `json:"completed"`
}

var (
	reminderFile = "reminders.json"
	reminders    []Reminder
	mu           sync.Mutex
	schedulerOn  bool
)

// --- ALARM MOTORU (Arka Plan) ---

// StartScheduler: Main fonksiyonunda bir kere çağrılmalı.
func StartScheduler() {
	if schedulerOn { return }
	schedulerOn = true
	
	loadReminders()

	go func() {
		ticker := time.NewTicker(30 * time.Second) // Her 30 saniyede bir kontrol et
		for range ticker.C {
			checkReminders()
		}
	}()
}

func checkReminders() {
	mu.Lock()
	defer mu.Unlock()

	now := time.Now()
	dirty := false

	for i, r := range reminders {
		if !r.Completed && now.After(r.Time) {
			// 🔔 ALARM ÇALDI!
			// Konsola bas (İleride WhatsApp'tan da atabilir)
			logger.Success("\n\n⏰ [ALARM] RICK HATIRLATIYOR: %s\n", r.Message)
			
			// Windows Toast Bildirimi (Opsiyonel - PowerShell ile)
			// go exec.Command("powershell", "-Command", fmt.Sprintf(`New-BurntToastNotification -Text "Rick Alarm", "%s"`, r.Message)).Run()

			reminders[i].Completed = true
			dirty = true
		}
	}

	if dirty {
		saveReminders()
	}
}

func loadReminders() {
	data, err := os.ReadFile(reminderFile)
	if err == nil {
		json.Unmarshal(data, &reminders)
	}
}

func saveReminders() {
	data, _ := json.MarshalIndent(reminders, "", "  ")
	os.WriteFile(reminderFile, data, 0644)
}

// --- TOOL: SET REMINDER ---

type SetReminderCommand struct {}

func (c *SetReminderCommand) Name() string { return "set_reminder" }
func (c *SetReminderCommand) Description() string {
	return "İleri tarihli hatırlatıcı/alarm kurar. Parametreler: 'time' (Format: YYYY-MM-DD HH:MM), 'message'."
}

func (c *SetReminderCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	timeStr, _ := args["time"].(string)
	msg, _ := args["message"].(string)

	if timeStr == "" || msg == "" {
		return "", fmt.Errorf("eksik parametre: time ve message zorunlu")
	}

	// Zamanı Parse Et
	// Rick bazen saniye de ekleyebilir, iki formatı da deneyelim
	layout := "2006-01-02 15:04"
	targetTime, err := time.Parse(layout, timeStr)
	if err != nil {
		// Alternatif format (saniyeli)
		targetTime, err = time.Parse("2006-01-02 15:04:05", timeStr)
		if err != nil {
			return "", fmt.Errorf("zaman formatı hatası (Beklenen: YYYY-MM-DD HH:MM): %v", err)
		}
	}

	if targetTime.Before(time.Now()) {
		return "", fmt.Errorf("geçmiş zamana alarm kurulamaz. (Şu an: %s)", time.Now().Format(layout))
	}

	mu.Lock()
	reminders = append(reminders, Reminder{
		ID:      fmt.Sprintf("rem_%d", time.Now().Unix()),
		Time:    targetTime,
		Message: msg,
		Completed: false,
	})
	saveReminders()
	mu.Unlock()

	// Eğer Scheduler henüz başlamadıysa başlat (Garanti olsun)
	StartScheduler()

	return fmt.Sprintf("⏰ Alarm Kuruldu!\nZaman: %s\nMesaj: %s", targetTime.Format(layout), msg), nil
}