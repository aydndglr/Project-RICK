package network

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
	"github.com/chromedp/chromedp"
)

func (t *BrowserTool) doRead(ctx context.Context, args map[string]interface{}) (string, error) {
	targetUrl, _ := args["url"].(string)
	if targetUrl == "" {
		return "", fmt.Errorf("HATA: 'read' modu için 'url' parametresi zorunludur")
	}

	visible, _ := args["visible"].(bool)
	cCtx, cancel := GetChromeContext(ctx, visible)
	defer cancel()

	var title, body string
	logger.Action("📖 Sayfa Okunuyor: %s", targetUrl)

	err := chromedp.Run(cCtx,
		chromedp.Navigate(targetUrl),
		chromedp.Sleep(3*time.Second),
		chromedp.Title(&title),
		chromedp.Evaluate(`
			(() => {
				const clone = document.body.cloneNode(true);
				['script', 'style', 'nav', 'footer', 'iframe'].forEach(tag => clone.querySelectorAll(tag).forEach(el => el.remove()));
				return clone.innerText;
			})()
		`, &body),
	)

	if err != nil {
		return "", fmt.Errorf("okuma hatası: %v", err)
	}

	cleanBody := strings.Join(strings.Fields(body), " ")
	if len(cleanBody) > 3000 {
		cleanBody = cleanBody[:3000] + "\n...(Kırpıldı)..."
	}
	return fmt.Sprintf("📄 BAŞLIK: %s\n\nİÇERİK:\n%s", title, cleanBody), nil
}