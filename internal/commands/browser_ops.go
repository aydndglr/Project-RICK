package commands

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// Tarayıcı yollarını otomatik bulan yardımcı fonksiyon
func findBrowserPath() string {
	var paths []string

	switch runtime.GOOS {
	case "windows":
		paths = []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,      // Edge (Her Windows'ta var)
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe`,
			os.Getenv("LOCALAPPDATA") + `\Google\Chrome\Application\chrome.exe`,
		}
	case "darwin": // MacOS
		paths = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	default: // Linux
		paths = []string{
			"/usr/bin/google-chrome",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/usr/bin/microsoft-edge",
		}
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return "" // Hiçbiri yoksa boş dön, chromedp varsayılanı denesin
}

// --- 1. BROWSE PAGE (Gezinme ve Okuma) ---

type BrowserReadCommand struct{}

func (c *BrowserReadCommand) Name() string { return "browser_read" }
func (c *BrowserReadCommand) Description() string {
	return "Tam tarayıcı ile siteye girer, JS çalıştırır ve metni döner. Parametre: url, selector (opsiyonel)."
}

func (c *BrowserReadCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	urlStr, ok := args["url"].(string)
	selector, _ := args["selector"].(string)
	if !ok || urlStr == "" {
		return "", fmt.Errorf("eksik parametre: url")
	}
	if selector == "" {
		selector = "body"
	}

	// Otomatik tarayıcı yolu bul
	execPath := findBrowserPath()
	
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"),
	)

	// Eğer tarayıcı bulduysak yolunu belirtelim
	if execPath != "" {
		opts = append(opts, chromedp.ExecPath(execPath))
	}

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	ctx, cancel = chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 45*time.Second) // Timeout biraz artırıldı
	defer cancel()

	var res string
	var title string

	err := chromedp.Run(ctx,
		chromedp.Navigate(urlStr),
		chromedp.Sleep(3*time.Second), // SPA siteler için bekleme
		chromedp.Title(&title),
		chromedp.Text(selector, &res, chromedp.NodeVisible),
	)

	if err != nil {
		return "", fmt.Errorf("tarayıcı hatası (Path: %s): %v", execPath, err)
	}

	cleanText := strings.Join(strings.Fields(res), " ")
	if len(cleanText) > 3000 {
		cleanText = cleanText[:3000] + "...(kısaltıldı)"
	}

	return fmt.Sprintf("🌐 SİTE: %s\nBAŞLIK: %s\n\nİÇERİK:\n%s", urlStr, title, cleanText), nil
}

// --- 2. BROWSE INTERACT (Tıkla ve Yaz) ---

type BrowserInteractCommand struct{}

func (c *BrowserInteractCommand) Name() string { return "browser_action" }
func (c *BrowserInteractCommand) Description() string {
	return "Sayfada işlem yapar. Parametreler: url, actions (liste)."
}

func (c *BrowserInteractCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	urlStr, ok := args["url"].(string)
	actionsRaw, okA := args["actions"].([]interface{})
	if !ok || !okA {
		return "", fmt.Errorf("eksik parametre: url ve actions listesi")
	}

	execPath := findBrowserPath()
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.UserAgent("RickAgent/3.0"),
	)
	if execPath != "" {
		opts = append(opts, chromedp.ExecPath(execPath))
	}

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()
	ctx, cancel = chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var tasks []chromedp.Action
	tasks = append(tasks, chromedp.Navigate(urlStr))
	tasks = append(tasks, chromedp.Sleep(3*time.Second))

	logBuilder := strings.Builder{}

	for _, a := range actionsRaw {
		actStr := a.(string)
		parts := strings.SplitN(actStr, ":", 3)
		if len(parts) < 2 { continue }
		
		actionType := parts[0]
		selector := parts[1]

		switch actionType {
		case "click":
			tasks = append(tasks, chromedp.Click(selector, chromedp.NodeVisible))
			logBuilder.WriteString(fmt.Sprintf("🖱️ Tıklandı: %s\n", selector))
		case "type":
			if len(parts) < 3 { continue }
			text := parts[2]
			tasks = append(tasks, chromedp.SendKeys(selector, text, chromedp.NodeVisible))
			logBuilder.WriteString(fmt.Sprintf("⌨️ Yazıldı: %s -> '%s'\n", selector, text))
		case "screenshot":
			var buf []byte
			tasks = append(tasks, chromedp.CaptureScreenshot(&buf))
			logBuilder.WriteString("📸 Ekran görüntüsü alındı (hafızada)\n")
		}
		tasks = append(tasks, chromedp.Sleep(1*time.Second))
	}

	if err := chromedp.Run(ctx, tasks...); err != nil {
		return "", fmt.Errorf("etkileşim hatası: %v", err)
	}

	return fmt.Sprintf("✅ İşlemler Tamamlandı:\n%s", logBuilder.String()), nil
}