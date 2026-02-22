package agent

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aydndglr/rick-agent-v3/internal/core/kernel"
	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
)

// BuildSystemPrompt: Rick'in anayasasını, mühendislik disiplinini ve karakterini oluşturur.
func BuildSystemPrompt(promptPath, workDir, securityLevel string, tools []kernel.Tool) kernel.Message {
	var toolDescriptions []string
	for _, t := range tools {
		toolDescriptions = append(toolDescriptions, fmt.Sprintf("- %s: %s", t.Name(), t.Description()))
	}

	// Yılı dinamik olarak alalım ki "2023 verileri" diye zırvalamasın.
	currentYear := time.Now().Year()

	var prompt string
	data, err := os.ReadFile(promptPath)
	
	if err == nil {
		// Dosya başarıyla okundu, formatı uygula
		prompt = fmt.Sprintf(string(data), currentYear, workDir, securityLevel, strings.Join(toolDescriptions, "\n"))
	} else {
		// Dosya bulunamazsa veya okunamazsa log bas ve varsayılana dön
		logger.Warn("⚠️ Prompt dosyası bulunamadı (%s), model saf hali ile çalışacak ")

	}

	return kernel.Message{
		Role:    "system",
		Content: prompt,
	}
}