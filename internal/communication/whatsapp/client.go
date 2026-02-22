package whatsapp

import (
	"context"
	"fmt"
	"os"

	"github.com/aydndglr/rick-agent-v3/internal/core/kernel"
	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	_ "modernc.org/sqlite"
)

type Listener struct {
	Client     *whatsmeow.Client
	Agent      kernel.Agent
	AdminPhone string
	DBPath     string
}

func New(agent kernel.Agent, adminPhone, dbPath string) *Listener {
	return &Listener{
		Agent:      agent,
		AdminPhone: adminPhone,
		DBPath:     dbPath,
	}
}

func (w *Listener) Start(ctx context.Context) error {
	dbLog := waLog.Stdout("Database", "ERROR", true)
	clientLog := waLog.Stdout("Client", "ERROR", true)

	// 🚀 KRİTİK DÜZELTME: Veritabanı kilitlenmelerini önlemek için WAL modunu aktif ettik.
	dbURL := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_busy_timeout=5000", w.DBPath)
	
	container, err := sqlstore.New(context.Background(), "sqlite", dbURL, dbLog)
	if err != nil {
		return fmt.Errorf("whatsapp db hatası: %v", err)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return err
	}

	w.Client = whatsmeow.NewClient(deviceStore, clientLog)
	w.Client.AddEventHandler(w.EventHandler)

	if w.Client.Store.ID == nil {
		qrChan, _ := w.Client.GetQRChannel(ctx)
		if err = w.Client.Connect(); err != nil {
			return err
		}
		
		fmt.Println("\n📱 WhatsApp Bağlantısı İçin QR Kodu Okut:")
		for evt := range qrChan {
			if evt.Event == "code" {
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			}
		}
	} else {
		if err = w.Client.Connect(); err != nil {
			return fmt.Errorf("bağlantı hatası: %v", err)
		}
		logger.Success("📱 WhatsApp Portalı Aktif")
	}

	// ========================================================================
	// 🚀 RICK CANLI YAYIN MOTORU (LIVE LOGGING)
	// ========================================================================
	// Rick bağlandığı an, logger'daki tüm önemli olayları WhatsApp'a yönlendiriyoruz.
	if w.AdminPhone != "" {
		// Admin JID oluştur (Örn: 905xxxxxxxxx@s.whatsapp.net)
		adminJID := types.NewJID(w.AdminPhone, types.DefaultUserServer)

		logger.SetOutputHook(func(level, message string) {
			// WhatsApp'a anlık "Push" bildirimi gönder
			// Sadece ACTION, SUCCESS, WARN ve ERROR'ları gönderiyoruz (Spam olmasın diye)
			icon := "ℹ️"
			switch level {
			case "ACTION":
				icon = "🛠️"
			case "SUCCESS":
				icon = "✅"
			case "WARN":
				icon = "⚠️"
			case "ERROR":
				icon = "❌"
			}

			liveMsg := fmt.Sprintf("%s *[%s]*\n%s", icon, level, message)
			w.SendReply(adminJID, liveMsg)
		})
		
		logger.Debug("📡 Rick Canlı Yayın Motoru: Aktif. Önemli olaylar %s adresine fırlatılacak.", w.AdminPhone)
	}

	return nil
}

func (w *Listener) Disconnect() {
	if w.Client != nil {
		w.Client.Disconnect()
	}
}