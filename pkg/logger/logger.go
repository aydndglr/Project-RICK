package logger

import (
	"fmt"
	"log"
	"os"
	"time"
)

// Renk Seviyeleri
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
	ColorPurple = "\033[35m"
)

var (
	debugMode bool
	logFile   *os.File
)

// Setup: Logger'ı yapılandırır. Debug kapalıysa dosyaya yönlendirir.
func Setup(debug bool) {
	debugMode = debug
	log.SetFlags(0)

	// Debug kapalıysa tüm logları (ve özellikle debug loglarını) dosyaya yazmak için dosya aç
	if !debug {
		f, err := os.OpenFile("rick_debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			logFile = f
			Info("📝 Debug modu kapalı. Ayrıntılı loglar 'rick_debug.log' dosyasına yazılıyor.")
		} else {
			fmt.Printf("⚠️ Log dosyası oluşturulamadı: %v\n", err)
		}
	}
}

// logMessage: Mesajın nereye gideceğine karar veren yardımcı fonksiyon
func logToDest(color, level, msg string, forceTerminal bool) {
	timestamp := time.Now().Format("15:04:05")
	formattedTerminal := fmt.Sprintf("%s[%-7s]%s %s %s", color, level, ColorReset, timestamp, msg)
	formattedFile := fmt.Sprintf("[%s] [%-7s] %s\n", time.Now().Format("2006-01-02 15:04:05"), level, msg)

	// Dosyaya her zaman yaz (yedek olması iyidir)
	if logFile != nil {
		logFile.WriteString(formattedFile)
	}

	// Terminale ne zaman yazılacak?
	// 1. debugMode true ise HER ŞEYİ yaz.
	// 2. forceTerminal true ise (Info, Success, Error, Warn) HER ZAMAN yaz.
	if debugMode || forceTerminal {
		if level == "ERROR" {
			fmt.Fprintln(os.Stderr, formattedTerminal)
		} else {
			fmt.Println(formattedTerminal)
		}
	}
}

func Info(formatStr string, v ...interface{})    { logToDest(ColorBlue, "INFO", fmt.Sprintf(formatStr, v...), true) }
func Success(formatStr string, v ...interface{}) { logToDest(ColorGreen, "SUCCESS", fmt.Sprintf(formatStr, v...), true) }
func Action(formatStr string, v ...interface{})  { logToDest(ColorPurple, "ACTION", fmt.Sprintf(formatStr, v...), true) }
func Warn(formatStr string, v ...interface{})    { logToDest(ColorYellow, "WARN", fmt.Sprintf(formatStr, v...), true) }
func Error(formatStr string, v ...interface{})   { logToDest(ColorRed, "ERROR", fmt.Sprintf(formatStr, v...), true) }

// Debug: Debug kapalıyken terminale basmaz, sadece dosyaya yazar.
func Debug(formatStr string, v ...interface{}) {
	logToDest(ColorCyan, "DEBUG", fmt.Sprintf(formatStr, v...), false)
}