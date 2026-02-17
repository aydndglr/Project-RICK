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

// StartScheduler: Main fonksiyonunda veya bir komut tetiklendiğinde başlatılır.
func StartScheduler() {
	if schedulerOn {
		return
	}
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
			logger.Success("\n\n⏰ [ALARM] RICK HATIRLATIYOR: %s\n", r.Message)
			
			// Gelecekte buraya WhatsApp veya sistem bildirimi eklenebilir.
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

type SetReminderCommand struct{}

func (c *SetReminderCommand) Name() string { return "set_reminder" }

func (c *SetReminderCommand) Description() string {
	return "Belirli bir tarih ve saat için hatırlatıcı veya alarm kurar."
}

// Parameters: Rick'e bu aracın şemasını bildirir.
func (c *SetReminderCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"time": map[string]interface{}{
				"type":        "string",
				"description": "Hatırlatıcı zamanı (Format: YYYY-MM-DD HH:MM).",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Zamanı geldiğinde hatırlatılacak mesaj.",
			},
		},
		"required": []string{"time", "message"},
	}
}

func (c *SetReminderCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	timeStr, _ := args["time"].(string)
	msg, _ := args["message"].(string)

	if timeStr == "" || msg == "" {
		return "", fmt.Errorf("hata: 'time' ve 'message' parametreleri zorunludur")
	}

	// Zamanı Parse Et
	layout := "2006-01-02 15:04"
	targetTime, err := time.Parse(layout, timeStr)
	if err != nil {
		// Alternatif format (saniyeli) denemesi
		targetTime, err = time.Parse("2006-01-02 15:04:05", timeStr)
		if err != nil {
			return "", fmt.Errorf("zaman formatı hatası (Beklenen: YYYY-MM-DD HH:MM): %v", err)
		}
	}

	if targetTime.Before(time.Now()) {
		return "", fmt.Errorf("geçmiş bir zamana alarm kurulamaz. (Şu anki zaman: %s)", time.Now().Format(layout))
	}

	mu.Lock()
	reminders = append(reminders, Reminder{
		ID:        fmt.Sprintf("rem_%d", time.Now().Unix()),
		Time:      targetTime,
		Message:   msg,
		Completed: false,
	})
	saveReminders()
	mu.Unlock()

	// Scheduler'ın çalıştığından emin ol
	StartScheduler()

	return fmt.Sprintf("⏰ Hatırlatıcı Kuruldu!\n📅 Zaman: %s\n💬 Mesaj: %s", targetTime.Format(layout), msg), nil
}