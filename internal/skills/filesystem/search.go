package filesystem

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
)

type SearchTool struct{}

func (t *SearchTool) Name() string { return "fs_search" }

func (t *SearchTool) Description() string {
	return "Dosya içeriklerinde metin araması (grep) yapar. Hangi dosyanın hangi satırında ne geçtiğini bulur. Büyük projelerde kod analizi için çok güçlüdür."
}

func (t *SearchTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":        map[string]interface{}{"type": "string", "description": "Aramanın başlatılacağı klasör yolu."},
			"query":       map[string]interface{}{"type": "string", "description": "Aranacak kelime veya metin parçası."},
			"recursive":   map[string]interface{}{"type": "boolean", "description": "Alt klasörlerde de ara (Varsayılan: true)."},
			"extension":   map[string]interface{}{"type": "string", "description": "Sadece belirli uzantılı dosyalarda ara (Örn: '.go', '.cs')."},
			"show_hidden": map[string]interface{}{"type": "boolean", "description": "Gizli dosya ve klasörleri de tara."},
		},
		"required": []string{"path", "query"},
	}
}

func (t *SearchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path := ResolvePath(args["path"].(string))
	query, _ := args["query"].(string)
	recursive, ok := args["recursive"].(bool)
	if !ok { recursive = true } // Varsayılan recursive
	showHidden, _ := args["show_hidden"].(bool)
	
	extFilter := ""
	if ext, ok := args["extension"].(string); ok {
		extFilter = strings.ToLower(ext)
	}

	if query == "" {
		return "⚠️ HATA: Aranacak bir 'query' belirtmedin balım!", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 İÇERİK ARAMASI: '%s'\n", query))
	sb.WriteString(fmt.Sprintf("📍 Konum: %s\n", path))
	sb.WriteString(strings.Repeat("-", 60) + "\n")

	matchCount := 0
	fileCount := 0
	const maxMatches = 100 // Rick'in aklının karışmaması için limit

	// Arama Motoru
	searchInFile := func(filePath string) {
		file, err := os.Open(filePath)
		if err != nil { return }
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 1
		for scanner.Scan() {
			if matchCount >= maxMatches { break }
			
			line := scanner.Text()
			if strings.Contains(strings.ToLower(line), strings.ToLower(query)) {
				// Eşleşme bulundu!
				relPath, _ := filepath.Rel(path, filePath)
				sb.WriteString(fmt.Sprintf("📍 %s [Satır %d]:\n   %s\n\n", relPath, lineNum, strings.TrimSpace(line)))
				matchCount++
			}
			lineNum++
		}
	}

	// Klasör Gezme
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil { return nil }
		if matchCount >= maxMatches { return filepath.SkipDir }

		name := d.Name()
		isHidden := strings.HasPrefix(name, ".")

		// Gizli klasör/dosya koruması
		if !showHidden && isHidden {
			if d.IsDir() { return filepath.SkipDir }
			return nil
		}

		if d.IsDir() {
			if !recursive && p != path { return filepath.SkipDir }
			return nil
		}

		// Uzantı filtresi
		if extFilter != "" && !strings.HasSuffix(strings.ToLower(name), extFilter) {
			return nil
		}

		// Sadece metin dosyalarını taramaya çalış (Basit bir güvenlik)
		// Binary dosyaları (exe, dll, png) taramayı atlayalım
		ignoredExts := []string{".exe", ".dll", ".png", ".jpg", ".zip", ".pdf", ".bin"}
		for _, ignore := range ignoredExts {
			if strings.HasSuffix(strings.ToLower(name), ignore) { return nil }
		}

		fileCount++
		searchInFile(p)
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("Arama sırasında hata oluştu: %v", err)
	}

	// Sonuç Raporu
	if matchCount == 0 {
		return fmt.Sprintf("📭 '%s' konumu ve altındaki %d dosyada '%s' ifadesine rastlanmadı.", path, fileCount, query), nil
	}

	footer := fmt.Sprintf(strings.Repeat("-", 60)+"\n✅ TOPLAM: %d dosyada %d eşleşme bulundu.", fileCount, matchCount)
	if matchCount >= maxMatches {
		footer += "\n⚠️ UYARI: Çok fazla eşleşme olduğu için liste kısıtlandı."
	}
	sb.WriteString(footer)

	logger.Success("🔍 Arama tamamlandı: %d eşleşme.", matchCount)
	return sb.String(), nil
}