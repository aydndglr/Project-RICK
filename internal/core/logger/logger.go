package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Renk Kodları
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
	ColorPurple = "\033[35m"
)

// LogHook: Log mesajlarını dışarıya (örneğin WhatsApp'a) fırlatmak için fonksiyon tipi
type LogHook func(level, message string)

var (
	debugMode   bool
	logFile     *os.File
	multiWriter io.Writer
	publishHook LogHook // WhatsApp veya diğer portallar buraya abone olacak
)

// SetOutputHook: Dışarıdan bir portalın (WhatsApp gibi) logları dinlemesini sağlar
func SetOutputHook(hook LogHook) {
	publishHook = hook
}

// Setup: Logger'ı başlatır. Hem terminale hem dosyaya yazar.
func Setup(debug bool, logDir string) {
	debugMode = debug

	// Log klasörünü oluştur
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("⚠️ Log dizini oluşturulamadı: %v\n", err)
		return
	}

	// Dosyayı aç
	path := filepath.Join(logDir, "rick_system.log")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("⚠️ Log dosyası açılamadı: %v\n", err)
		return
	}
	logFile = f

	// MultiWriter: Hem stdout hem dosya
	multiWriter = io.MultiWriter(os.Stdout, f)
}

func logMessage(color, level, msg string) {
	timestamp := time.Now().Format("15:04:05")
	
	// Terminal için renkli
	consoleMsg := fmt.Sprintf("%s[%-7s]%s %s %s\n", color, level, ColorReset, timestamp, msg)
	
	// Dosya için renksiz ve tarihli
	fileMsg := fmt.Sprintf("[%s] [%-7s] %s\n", time.Now().Format("2006-01-02 15:04:05"), level, msg)

	// 1. Terminale Yaz
	if level == "DEBUG" && !debugMode {
		// Debug kapalıysa ekrana basma
	} else {
		fmt.Print(consoleMsg)
	}

	// 2. Dosyaya Yaz
	if logFile != nil {
		logFile.WriteString(fileMsg)
	}

	// 3. 🚀 CANLI YAYIN (WhatsApp Hook)
	// Sadece önemli logları (Action, Success, Error, Warn) WhatsApp'a gönderelim.
	// Debug ve Info çok fazla mesaj birikmesine (spam) neden olabilir.
	if publishHook != nil && level != "DEBUG" && level != "INFO" {
		// Mesajın içindeki olası kaçış karakterlerini temizle ve gönder
		cleanMsg := strings.ReplaceAll(msg, "\033", "") 
		publishHook(level, cleanMsg)
	}
}

func Info(format string, v ...interface{})    { logMessage(ColorBlue, "INFO", fmt.Sprintf(format, v...)) }
func Success(format string, v ...interface{}) { logMessage(ColorGreen, "SUCCESS", fmt.Sprintf(format, v...)) }
func Action(format string, v ...interface{})  { logMessage(ColorPurple, "ACTION", fmt.Sprintf(format, v...)) }
func Warn(format string, v ...interface{})    { logMessage(ColorYellow, "WARN", fmt.Sprintf(format, v...)) }
func Error(format string, v ...interface{})   { logMessage(ColorRed, "ERROR", fmt.Sprintf(format, v...)) }
func Debug(format string, v ...interface{})   { logMessage(ColorCyan, "DEBUG", fmt.Sprintf(format, v...)) }

// Close: Dosyayı kapatır
func Close() {
	if logFile != nil {
		logFile.Close()
	}
}