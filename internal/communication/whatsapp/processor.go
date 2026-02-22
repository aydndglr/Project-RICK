package whatsapp

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aydndglr/rick-agent-v3/internal/agent"
	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// 🚀 ÇOKLU GÖREV YÖNETİCİSİ (SWARM ROUTER)
var (
	chatTracker   = make(map[types.JID]int) // Hangi sohbette kaç aktif görev var sayar
	chatTrackerMu sync.Mutex
	hookOnce      sync.Once // Global kancanın sadece 1 kez atılmasını sağlar
)

func (w *Listener) EventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		w.HandleMessage(v)
	}
}

func (w *Listener) HandleMessage(evt *events.Message) {
	if evt.Info.IsFromMe {
		return
	}

	sender := evt.Info.Sender.User
	if w.AdminPhone != "" && !strings.Contains(sender, w.AdminPhone) {
		return
	}

	var msgText string
	var images []string

	// 1. İçerik Ayıklama (Metin)
	msgText = evt.Message.GetConversation()
	if msgText == "" && evt.Message.GetExtendedTextMessage() != nil {
		msgText = evt.Message.GetExtendedTextMessage().GetText()
	}

	// 🔄 Alıntılanan Mesajı Yakala
	var quotedText string
	if ext := evt.Message.GetExtendedTextMessage(); ext != nil && ext.GetContextInfo() != nil {
		if quotedMsg := ext.GetContextInfo().GetQuotedMessage(); quotedMsg != nil {
			if conv := quotedMsg.GetConversation(); conv != "" {
				quotedText = conv
			} else if qExt := quotedMsg.GetExtendedTextMessage(); qExt != nil {
				quotedText = qExt.GetText()
			}
			
			if quotedText != "" {
				if len(quotedText) > 500 {
					quotedText = quotedText[:500] + "..."
				}
				msgText = fmt.Sprintf("[Bağlam - Kullanıcı şu mesaja yanıt veriyor: \"%s\"]\n\nYeni Mesaj: %s", quotedText, msgText)
				logger.Info("🔄 Alıntı yakalandı ve bağlama eklendi.")
			}
		}
	}

	// 2. İçerik Ayıklama (Görsel)
	imgMsg := evt.Message.GetImageMessage()
	if imgMsg != nil {
		if msgText == "" && imgMsg.Caption != nil {
			msgText = *imgMsg.Caption
		}
		
		downloadCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		data, err := w.Client.Download(downloadCtx, imgMsg)
		cancel()

		if err != nil {
			logger.Error("❌ Resim indirilemedi: %v", err)
		} else {
			images = append(images, base64.StdEncoding.EncodeToString(data))
			logger.Info("📸 Görsel yakalandı.")
		}
	}

	if msgText == "" && len(images) == 0 {
		return
	}

	// 3. UI İşlemleri
	w.MarkAsRead(evt)
	w.SetPresence(evt.Info.Chat, types.ChatPresenceComposing)

	// 4. Ajanı Çalıştır ve CANLI YAYINI Başlat
	go func() {
		// ========================================================================
		// 🚀 RICK CANLI YAYIN MOTORU (MULTI-THREAD GÜVENLİ)
		// ========================================================================
		
		// Kancayı sisteme sadece ilk görev geldiğinde asıyoruz
		hookOnce.Do(func() {
			logger.SetOutputHook(func(level, message string) {
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
				
				// Logu aktif olarak görev bekleyen tüm sohbetlere fırlat
				chatTrackerMu.Lock()
				for jid := range chatTracker {
					w.SendReply(jid, liveMsg)
				}
				chatTrackerMu.Unlock()
			})
		})

		// Bu sohbetin aktif görev sayısını 1 artır
		chatTrackerMu.Lock()
		chatTracker[evt.Info.Chat]++
		chatTrackerMu.Unlock()

		// Görev bittiğinde (veya hata verdiğinde) sayacı 1 azalt. Eğer 0 olduysa bu sohbeti dinlemeden çıkar.
		defer func() {
			chatTrackerMu.Lock()
			chatTracker[evt.Info.Chat]--
			if chatTracker[evt.Info.Chat] <= 0 {
				delete(chatTracker, evt.Info.Chat)
			}
			chatTrackerMu.Unlock()
		}()

		timeoutMin := 15 
		
		if rickAgent, ok := w.Agent.(*agent.Rick); ok {
			if rickAgent.Config.App.TimeoutMinutes > 0 {
				timeoutMin = rickAgent.Config.App.TimeoutMinutes
			}
		}

		logger.Debug("⏳ Görev zaman aşımı süresi: %d dakika", timeoutMin)
		
		timeoutDuration := time.Duration(timeoutMin) * time.Minute
		ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
		defer cancel()

		// Beyin düşünmeye başlıyor... 🧠
		response, err := w.Agent.Run(ctx, msgText, images)
		
		w.SetPresence(evt.Info.Chat, types.ChatPresencePaused)

		// Final raporunu gönder
		if err != nil {
			w.SendReply(evt.Info.Chat, "💥 Sistemsel Hata: "+err.Error())
		} else {
			w.SendReply(evt.Info.Chat, response)
		}
	}()
}