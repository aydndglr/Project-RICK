package patcher

import (
	"fmt"
	"os"
	"strings"

	"github.com/aydndglr/rick-agent/pkg/logger"
)

// SmartPatcher: Dil bağımsız, metin tabanlı akıllı yama yöneticisi.
type SmartPatcher struct {
	BaseDir string
}

func NewSmartPatcher(baseDir string) *SmartPatcher {
	return &SmartPatcher{BaseDir: baseDir}
}

// Apply: Dosya içinde 'searchBlock' kısmını bulur ve 'replaceBlock' ile değiştirir.
func (p *SmartPatcher) Apply(filePath, searchBlock, replaceBlock string) error {
	logger.Action("🧩 Patch işlemi başlatılıyor: %s", filePath)

	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("dosya okunamadı: %v", err)
	}
	content := string(contentBytes)

	// Normalizasyon (CRLF -> LF)
	content = strings.ReplaceAll(content, "\r\n", "\n")
	searchBlock = strings.ReplaceAll(searchBlock, "\r\n", "\n")
	replaceBlock = strings.ReplaceAll(replaceBlock, "\r\n", "\n")

	searchBlock = strings.TrimSpace(searchBlock)
	replaceBlock = strings.TrimSpace(replaceBlock)

	// 1. Tam Eşleşme (Exact Match)
	if strings.Contains(content, searchBlock) {
		newContent := strings.Replace(content, searchBlock, replaceBlock, 1)
		return p.saveFile(filePath, newContent)
	}

	// 2. Esnek Arama (Fuzzy Match)
	logger.Warn("⚠️ Tam eşleşme bulunamadı, esnek arama deneniyor...")
	newContent, err := p.fuzzyReplace(content, searchBlock, replaceBlock)
	if err != nil {
		return fmt.Errorf("patch uygulanamadı! Modelin verdiği 'SEARCH' bloğu dosyada bulunamıyor. Hata: %v", err)
	}

	return p.saveFile(filePath, newContent)
}

// fuzzyReplace: Boşluk duyarsız satır eşleştirme
func (p *SmartPatcher) fuzzyReplace(content, search, replace string) (string, error) {
	fileLines := strings.Split(content, "\n")
	searchLines := strings.Split(search, "\n")

	if len(searchLines) == 0 {
		return "", fmt.Errorf("arama bloğu boş olamaz")
	}

	matchIndex := -1

	// Dosyayı satır satır gez
	for i := 0; i <= len(fileLines)-len(searchLines); i++ {
		match := true
		for j := 0; j < len(searchLines); j++ {
			if strings.TrimSpace(fileLines[i+j]) != strings.TrimSpace(searchLines[j]) {
				match = false
				break
			}
		}

		if match {
			matchIndex = i
			break
		}
	}

	if matchIndex == -1 {
		return "", fmt.Errorf("kod bloğu bulunamadı")
	}

	var result []string
	result = append(result, fileLines[:matchIndex]...)
	result = append(result, replace)
	result = append(result, fileLines[matchIndex+len(searchLines):]...)

	return strings.Join(result, "\n"), nil
}

func (p *SmartPatcher) saveFile(path, content string) error {
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return err
	}
	logger.Success("✅ Dosya başarıyla güncellendi: %s", path)
	return nil
}