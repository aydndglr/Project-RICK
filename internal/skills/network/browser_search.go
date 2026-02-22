package network

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
	"github.com/chromedp/chromedp"
)

func (t *BrowserTool) doSearch(ctx context.Context, args map[string]interface{}) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("HATA: 'search' modu için 'query' parametresi zorunludur")
	}

	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))
	
	visible, _ := args["visible"].(bool)
	cCtx, cancel := GetChromeContext(ctx, visible)
	defer cancel()

	var res string
	logger.Action("🔍 Arama Motoru: %s", query)

	jsEval := `
		(() => {
			if (document.body.innerText.includes("If this is not a bot")) return "BOT_BLOCKED";
			return Array.from(document.querySelectorAll('.result')).slice(0, 5).map(el => {
				const title = el.querySelector('.result__title')?.innerText.trim() || '';
				const link = el.querySelector('.result__a')?.href || '';
				const snippet = el.querySelector('.result__snippet')?.innerText.trim() || '';
				return title ? ("### " + title + "\n🔗 " + link + "\n📝 " + snippet + "\n") : "";
			}).join("\n");
		})()
	`
	err := chromedp.Run(cCtx,
		chromedp.Navigate(searchURL),
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(jsEval, &res),
	)

	if err != nil {
		return "", fmt.Errorf("arama başarısız: %v", err)
	}
	if res == "BOT_BLOCKED" {
		return "⚠️ Arama motoru bot korumasına takıldı.", nil
	}
	if res == "" {
		return "⚠️ Hiçbir sonuç bulunamadı.", nil
	}

	return fmt.Sprintf("🔍 ARAMA SONUÇLARI (%s):\n\n%s", query, res), nil
}