package whatsapp

import (
	"context"
	"time"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// SendReply: Kullanıcıya cevap mesajı gönderir.
func (w *Listener) SendReply(jid types.JID, text string) {
	w.Client.SendMessage(context.Background(), jid, &waProto.Message{
		Conversation: proto.String(text),
	})
}

// MarkAsRead: Mesajı "okundu" olarak işaretler.
func (w *Listener) MarkAsRead(evt *events.Message) {
	w.Client.MarkRead(context.Background(), []types.MessageID{evt.Info.ID}, time.Now(), evt.Info.Chat, evt.Info.Sender)
}

// SetPresence: WhatsApp'ta "yazıyor..." veya "kaydediyor..." bilgisini günceller.
func (w *Listener) SetPresence(jid types.JID, presence types.ChatPresence) {
	w.Client.SendChatPresence(context.Background(), jid, presence, types.ChatPresenceMediaText)
}