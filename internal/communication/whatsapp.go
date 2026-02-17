package communication

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/aydndglr/rick-agent/internal/agent"
	"github.com/aydndglr/rick-agent/pkg/logger"
	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	waTypes "go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"
)

type WhatsappListener struct {
	Client     *whatsmeow.Client
	AdminPhone string
	Agent      *agent.Agent
}

func NewWhatsappListener(adminPhone string, a *agent.Agent) *WhatsappListener {
	// 1. Log seviyesini config'e göre belirle
	logLevel := "INFO"
	if a.Config != nil && !a.Config.App.Debug {
		logLevel = "ERROR" // Debug kapalıysa terminali kirletme
	}
	
	// Database logları da aynı seviyede olsun ki terminal temiz kalsın
	dbLog := waLog.Stdout("Database", logLevel, true)
	clientLog := waLog.Stdout("Client", logLevel, true)

	// 2. Veritabanı bağlantısı
	db, err := sql.Open("sqlite", "file:rick_whatsapp.db?cache=shared")
	if err != nil {
		panic(fmt.Sprintf("Veritabanı açılamadı: %v", err))
	}

	_, err = db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		panic(fmt.Sprintf("Foreign key aktif edilemedi: %v", err))
	}

	container := sqlstore.NewWithDB(db, "sqlite", dbLog)

	err = container.Upgrade(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Veritabanı yükseltme hatası: %v", err))
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		panic(err)
	}

	// 3. Client oluştur
	client := whatsmeow.NewClient(deviceStore, clientLog)

	return &WhatsappListener{
		Client:     client,
		AdminPhone: strings.TrimPrefix(adminPhone, "+"),
		Agent:      a,
	}
}

func (w *WhatsappListener) Start(ctx context.Context) {
	w.Client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			if v.Info.IsFromMe {
				return
			}

			sender := v.Info.Sender.User
			// Yetki Kontrolü
			if w.AdminPhone != "" && !strings.Contains(sender, w.AdminPhone) {
				// Gereksiz log kirliliğini önlemek için burayı sessize aldık
				// logger.Warn("🚫 Yetkisiz mesaj: %s", sender)
				return
			}

			msgText := v.Message.GetConversation()
			if msgText == "" {
				msgText = v.Message.GetExtendedTextMessage().GetText()
			}

			if msgText != "" {
				logger.Info("📩 WhatsApp Mesajı: %s", msgText)
				
				go func() {
					// Rick'i çalıştır ve cevabı al
					result, err := w.Agent.Run(ctx, msgText)
					
					if err != nil {
						errorMsg := fmt.Sprintf("🚨 Rick bir hata ile karşılaştı: %v", err)
						w.SendMessage(v.Info.Chat, errorMsg)
						return
					}
					
					// Nihai cevabı WhatsApp'tan gönder
					w.SendMessage(v.Info.Chat, result)
				}()
			}
		}
	})

	if w.Client.Store.ID == nil {
		qrChan, _ := w.Client.GetQRChannel(ctx)
		err := w.Client.Connect()
		if err != nil {
			panic(err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				fmt.Println("\n📱 Rick için QR Kodu okut patron:")
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			}
		}
	} else {
		w.Client.Connect()
	}
}

func (w *WhatsappListener) SendMessage(jid waTypes.JID, text string) {
	w.Client.SendMessage(context.Background(), jid, &waProto.Message{
		Conversation: proto.String(text),
	})
}