package network

import (
	"context"
	"os"
	"runtime"

	"github.com/chromedp/chromedp"
)

// GetChromeContext: Rick için gizli veya görünür bir tarayıcı penceresi hazırlar.
func GetChromeContext(parentCtx context.Context, visible bool) (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	// 🚀 DİNAMİK GÖRÜNÜRLÜK AYARI
	if !visible {
		// Varsayılan: Arka planda gizli çalış (Hızlı ve performanslı)
		opts = append(opts, chromedp.Flag("headless", true))
	} else {
		// Özel İstek: Ekranda görünür şekilde aç (Müzik çalmak veya görsel test için)
		opts = append(opts, chromedp.Flag("headless", false))
	}

	// Windows üzerinde tarayıcı yolunu otomatik bulalım
	if browserPath := findBrowserPath(); browserPath != "" {
		opts = append(opts, chromedp.ExecPath(browserPath))
	}

	allocCtx, _ := chromedp.NewExecAllocator(parentCtx, opts...)
	return chromedp.NewContext(allocCtx)
}

// findBrowserPath: Sisteme göre yaygın tarayıcı yollarını kontrol eder.
func findBrowserPath() string {
	if runtime.GOOS != "windows" {
		return "" // Linux/Mac'te varsayılanlar genellikle çalışır
	}

	// Windows için olası Chrome ve Edge yolları
	paths := []string{
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		os.Getenv("LOCALAPPDATA") + `\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}