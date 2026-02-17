package commands

import (
	"context"
	"fmt"
)

// ConversationalReplyCommand: Sohbet amaçlı "boş" araç.
type ConversationalReplyCommand struct{}

func (c *ConversationalReplyCommand) Name() string { return "conversational_reply" }

func (c *ConversationalReplyCommand) Description() string {
	return "Kullanıcı sohbet ediyor, hal hatır soruyor veya işlem gerektirmeyen bir şey söylüyorsa bu aracı kullan. Parametre: 'reply' (Vereceğin cevap)."
}

func (c *ConversationalReplyCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	reply, ok := args["reply"].(string)
	if !ok || reply == "" {
		return "", fmt.Errorf("boş cevap verilemez")
	}
	// Teknik olarak bir işlem yapmıyoruz, sadece cevabı döndürüyoruz.
	// Ajan (Agent) bu cevabı alıp kullanıcıya iletecek.
	return reply, nil
}