package commands

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery" // HTML parse etmek için (go get github.com/PuerkitoBio/goquery)
)

// --- 1. WEB SEARCH COMMAND ---

type WebSearchCommand struct{}

func (c *WebSearchCommand) Name() string { return "web_search" }

func (c *WebSearchCommand) Description() string {
	return "İnternette arama yapar ve ilk 5 sonucu özet olarak döner."
}

func (c *WebSearchCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Aranacak kelime veya cümle.",
			},
		},
		"required": []string{"query"},
	}
}

func (c *WebSearchCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("eksik parametre: 'query' zorunludur")
	}

	// DuckDuckGo HTML sürümü üzerinden arama yapıyoruz (API Key gerektirmez)
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))
	
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	// User-Agent eklemezsek bot olduğumuzu anlar ve engeller
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("arama hatası: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("arama motoru hata verdi (Status: %d)", resp.StatusCode)
	}

	// HTML Parse Etme (goquery kullanarak)
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("html parse hatası: %v", err)
	}

	var results []string
	count := 0

	// DuckDuckGo sonuçlarını ayıkla (.result__body sınıfı)
	doc.Find(".result__body").Each(func(i int, s *goquery.Selection) {
		if count >= 5 {
			return
		}
		title := strings.TrimSpace(s.Find(".result__title").Text())
		link := s.Find(".result__a").AttrOr("href", "")
		snippet := strings.TrimSpace(s.Find(".result__snippet").Text())

		if title != "" && link != "" {
			results = append(results, fmt.Sprintf("🔹 **%s**\n   🔗 %s\n   📝 %s", title, link, snippet))
			count++
		}
	})

	if len(results) == 0 {
		return "⚠️ Arama yapıldı ama anlamlı sonuç bulunamadı. Sorguyu değiştirmeyi dene.", nil
	}

	return fmt.Sprintf("🔍 '%s' için Arama Sonuçları:\n\n%s", query, strings.Join(results, "\n\n")), nil
}

// --- 2. DOWNLOAD FILE COMMAND ---

type DownloadFileCommand struct {
	BaseDir string
}

func (c *DownloadFileCommand) Name() string { return "download_file" }

func (c *DownloadFileCommand) Description() string {
	return "İnternetten dosya indirir."
}

func (c *DownloadFileCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "İndirilecek dosyanın bağlantısı (URL).",
			},
			"filename": map[string]interface{}{
				"type":        "string",
				"description": "Kaydedilecek dosya adı (Opsiyonel). Boş bırakılırsa URL'den tahmin edilir.",
			},
		},
		"required": []string{"url"},
	}
}

func (c *DownloadFileCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	fileURL, ok := args["url"].(string)
	if !ok || fileURL == "" {
		return "", fmt.Errorf("eksik parametre: 'url' zorunludur")
	}

	// Dosya adını belirle
	fileName, okN := args["filename"].(string)
	if !okN || fileName == "" {
		// URL'den dosya adını çıkar (örn: .../installer.exe)
		tokens := strings.Split(fileURL, "/")
		if len(tokens) > 0 {
			fileName = tokens[len(tokens)-1]
		} else {
			fileName = fmt.Sprintf("downloaded_%d.bin", time.Now().Unix())
		}
		// Temizle (Query parametrelerini at)
		if idx := strings.Index(fileName, "?"); idx != -1 {
			fileName = fileName[:idx]
		}
	}

	fullPath := filepath.Join(c.BaseDir, fileName)

	// İndirme İşlemi
	client := &http.Client{Timeout: 600 * time.Second} // Büyük dosyalar için uzun timeout
	resp, err := client.Get(fileURL)
	if err != nil {
		return "", fmt.Errorf("indirme başlatılamadı: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("indirme hatası (Status: %d)", resp.StatusCode)
	}

	// Dosyayı oluştur
	out, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("dosya oluşturulamadı: %v", err)
	}
	defer out.Close()

	// Veriyi yaz
	n, err := io.Copy(out, resp.Body)
	if err != nil {
		return "", fmt.Errorf("yazma hatası: %v", err)
	}

	return fmt.Sprintf("✅ İndirme Başarılı:\n📂 %s\n📦 Boyut: %.2f MB", fileName, float64(n)/1024/1024), nil
}