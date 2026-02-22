package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/aydndglr/rick-agent-v3/internal/agent"
	"github.com/aydndglr/rick-agent-v3/internal/brain/providers"
	"github.com/aydndglr/rick-agent-v3/internal/communication/whatsapp"
	"github.com/aydndglr/rick-agent-v3/internal/core/config"
	"github.com/aydndglr/rick-agent-v3/internal/core/kernel" // 🚀 YENİ: Brain interface'i için eklendi
	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
	"github.com/aydndglr/rick-agent-v3/internal/memory"
	"github.com/aydndglr/rick-agent-v3/internal/skills"
	"github.com/aydndglr/rick-agent-v3/internal/skills/coding"
	"github.com/aydndglr/rick-agent-v3/internal/skills/filesystem"
	"github.com/aydndglr/rick-agent-v3/internal/skills/network"
	"github.com/aydndglr/rick-agent-v3/internal/skills/system"
)

func main() {
	// 1. YAPILANDIRMA YÜKLE
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		fmt.Printf("❌ Config yüklenemedi: %v\n", err)
		os.Exit(1)
	}

	// 2. LOGGER BAŞLAT
	logger.Setup(cfg.App.Debug, "logs")
	logger.Info("🚀 Rick C-137 Uyandırılıyor ")
	logger.Info("🔒 Güvenlik Seviyesi: %s", cfg.Security.Level)

	// 3. BEYİN BAĞLANTISI (DİNAMİK SAĞLAYICI)
	var brain kernel.Brain

	switch cfg.Brain.Primary.Provider {
	case "gemini":
		if cfg.Brain.APIKeys.Gemini == "" {
			logger.Error("💥 Gemini API anahtarı eksik! config.yaml dosyasını kontrol et.")
			os.Exit(1)
		}
		brain = providers.NewGemini(
			cfg.Brain.Primary.BaseURL,
			cfg.Brain.APIKeys.Gemini,
			cfg.Brain.Primary.ModelName,
		)
		logger.Success("🧠 Ana Beyin: Google Gemini (%s)", cfg.Brain.Primary.ModelName)

	case "openai":
		if cfg.Brain.APIKeys.OpenAI == "" {
			logger.Error("💥 OpenAI API anahtarı eksik! config.yaml dosyasını kontrol et.")
			os.Exit(1)
		}
		brain = providers.NewOpenAI(
			cfg.Brain.Primary.BaseURL,
			cfg.Brain.APIKeys.OpenAI,
			cfg.Brain.Primary.ModelName,
		)
		logger.Success("🧠 Ana Beyin: OpenAI (%s)", cfg.Brain.Primary.ModelName)

	case "ollama":
		brain = providers.NewOllama(
			cfg.Brain.Primary.BaseURL,
			cfg.Brain.Primary.ModelName,
			cfg.Brain.Primary.Temperature,
			cfg.Brain.Primary.NumCtx,
		)
		logger.Success("🧠 Ana Beyin: Local Ollama (%s)", cfg.Brain.Primary.ModelName)

	default:
		logger.Error("💥 Bilinmeyen sağlayıcı: %s. (Desteklenenler: gemini, openai, ollama)", cfg.Brain.Primary.Provider)
		os.Exit(1)
	}

	// 4. HAFIZA (VECTOR STORE) BAŞLAT
	memStore := memory.NewVectorStore("rick_memory.json", brain)

	// 4.5. VENV KURULUMU (Sanal Python Ortamı)
	env, err := skills.SetupVenv("tools")
	if err != nil {
		logger.Error("💥 Venv kurulamadı: %v", err)
		os.Exit(1)
	}

	// 5 YETENEK YÖNETİCİSİ (Skill Manager)
	skillMgr := skills.NewManager()

	// 5.1 "YARATICI"YI EKLE (The Creator)
	creator := coding.NewDevStudio("tools", env.PipPath, env.PythonPath)
	editor := coding.NewEditor("tools", env.PipPath, env.PythonPath)
	deleter := coding.NewDeleter("tools")
	
	skillMgr.Register(creator)
	skillMgr.Register(editor)
	skillMgr.Register(deleter)

	// 5.2 NATIVE (GO) ARAÇLARI YÜKLE
	skillMgr.Register(&filesystem.ListTool{})
	skillMgr.Register(&filesystem.ReadTool{})
	skillMgr.Register(&filesystem.WriteTool{})
	skillMgr.Register(&filesystem.DeleteTool{})
	skillMgr.Register(&filesystem.SearchTool{})
	
	// 5.3 INTERNET / BROWSER ARACI
	skillMgr.Register(&network.BrowserTool{})
	skillMgr.Register(&network.SSHTool{})

	// 5.4 YENİ EKLENEN ARKA PLAN ARAÇLARI
	skillMgr.Register(&system.StartTaskTool{})
	skillMgr.Register(&system.CheckTaskTool{})
	skillMgr.Register(&system.KillTaskTool{})

	// 5.5 ZAMANLANMIŞ GÖREVLER
	skillMgr.Register(&system.ScheduleTaskTool{})

	// DİSKTEKİ (PYTHON) ARAÇLARI YÜKLE
	loader := skills.NewLoader(skillMgr, "tools", env.PythonPath)
	if err := loader.LoadAll(); err != nil {
		logger.Warn("⚠️ Araçlar yüklenirken uyarı: %v", err)
	}

	// 6. AJANI OLUŞTUR (Rick)
	rick := agent.NewRick(cfg, brain, skillMgr, memStore)

	// 7. CONTEXT & SHUTDOWN HANDLER
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 8. WHATSAPP LISTENER
	if cfg.Communication.Whatsapp.Enabled {
		wa := whatsapp.New(
			rick,
			cfg.Communication.Whatsapp.AdminPhone,
			cfg.Communication.Whatsapp.DatabasePath,
		)
		
		go func() {
			logger.Info("👂 Portal Açılıyor...")
			if err := wa.Start(ctx); err != nil {
				logger.Error("WhatsApp Hatası: %v", err)
			}
		}()
		defer wa.Disconnect()
	}

	// Graceful Shutdown (CTRL+C)
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		logger.Info("\n🛑 Sistem kapatılıyor...")
		cancel()
		logger.Close()
		os.Exit(0)
	}()

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("🤖 RICK AGENT V4 - ONLINE")
	fmt.Println(strings.Repeat("=", 50))

	scanner := bufio.NewScanner(os.Stdin)
	for {
		if !scanner.Scan() {
			break
		}
		input := scanner.Text()

		if input == "exit" || input == "quit" {
			break
		}
		if input == "" {
			continue
		}

		if _, err := rick.Run(ctx, input, nil); err != nil {
			logger.Error("💥 Döngü Hatası: %v", err)
		}
	}
}