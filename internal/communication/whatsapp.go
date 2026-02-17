package communication

import (
	"context"
	"database/sql"
	"encoding/json" // EKLENDİ: JSON ayrıştırmak için
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
	// 1. Log Seviyesi Ayarı
	logLevel := "INFO"
	if a.Config != nil && !a.Config.App.Debug {
		logLevel = "ERROR"
	}
	
	dbLog := waLog.Stdout("Database", logLevel, true)
	clientLog := waLog.Stdout("Client", logLevel, true)

	// 2. Veritabanı Bağlantısı 
	// DÜZELTME: _pragma=foreign_keys(1) eklendi. SQLite için bu zorunlu.
	db, err := sql.Open("sqlite", "file:rick_whatsapp.db?_pragma=foreign_keys(1)&_journal_mode=WAL")
	if err != nil {
		panic(fmt.Sprintf("Veritabanı açılamadı: %v", err))
	}

	// Garanti olsun diye manuel de açıyoruz
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		panic(fmt.Sprintf("Foreign keys açılamadı: %v", err))
	}

	// Bağlantı havuzu ayarları
	db.SetMaxOpenConns(1)

	container := sqlstore.NewWithDB(db, "sqlite", dbLog)
	if err := container.Upgrade(context.Background()); err != nil {
		panic(fmt.Sprintf("Veritabanı yükseltme hatası: %v", err))
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		panic(err)
	}

	// 3. Client Oluştur
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
				return
			}

			msgText := v.Message.GetConversation()
			if msgText == "" {
				msgText = v.Message.GetExtendedTextMessage().GetText()
			}

			if msgText != "" {
				logger.Info("📩 WhatsApp Mesajı: %s", msgText)
				
				// 🚀 ASENKRON İŞLEYİCİ
				go w.handleMessage(ctx, v.Info.Chat, msgText)
			}
		}
	})

	if w.Client.Store.ID == nil {
		qrChan, _ := w.Client.GetQRChannel(ctx)
		if err := w.Client.Connect(); err != nil {
			panic(err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				fmt.Println("\n📱 Rick için QR Kodu okut patron:")
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			}
		}
	} else {
		if err := w.Client.Connect(); err != nil {
			fmt.Printf("Bağlantı hatası: %v\n", err)
		}
	}
}

// handleMessage: Mesajı işleyip parça parça cevap gönderen fonksiyon
// GÜNCELLEME: JSON verisini temizleyip sadece 'content' kısmını gönderir.
func (w *WhatsappListener) handleMessage(ctx context.Context, chatJID waTypes.JID, text string) {
	// Rick'in düşünce akışını (stream) başlat
	streamChan := w.Agent.RunStream(ctx, text)

	// Kanaldan gelen her mesajı anında WhatsApp'a ilet
	for partialResponse := range streamChan {
		cleanText := strings.TrimSpace(partialResponse)
		if cleanText == "" {
			continue
		}

		// Rick bazen JSON ("{ content: '...' }") bazen de düz metin gönderir (Tool çıktıları).
		// Önce JSON olup olmadığını kontrol edelim.
		var responseObj struct {
			Content string `json:"content"`
		}

		// Eğer gelen veri geçerli bir JSON ise ve içinde 'content' alanı varsa:
		if err := json.Unmarshal([]byte(cleanText), &responseObj); err == nil {
			if responseObj.Content != "" {
				w.SendMessage(chatJID, responseObj.Content)
			}
			// JSON geçerli ama content boşsa (örneğin sadece tool_call varsa)
			// sessizce geçiyoruz, çünkü asıl çıktıyı tool çalışınca verecek.
		} else {
			// JSON değilse (Örn: "Task started ID: 123" gibi sistem mesajları veya hatalar)
			// Mesajı olduğu gibi gönder.
			w.SendMessage(chatJID, cleanText)
		}
	}
}

func (w *WhatsappListener) SendMessage(jid waTypes.JID, text string) {
	for i := 0; i < 2; i++ {
		_, err := w.Client.SendMessage(context.Background(), jid, &waProto.Message{
			Conversation: proto.String(text),
		})
		if err == nil {
			break
		}
	}
}