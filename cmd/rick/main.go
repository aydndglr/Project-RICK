package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"fmt"

	"github.com/aydndglr/rick-agent/internal/agent"
	"github.com/aydndglr/rick-agent/internal/brain"
	"github.com/aydndglr/rick-agent/internal/communication"
	"github.com/aydndglr/rick-agent/internal/config"
	"github.com/aydndglr/rick-agent/internal/memory"
	"github.com/aydndglr/rick-agent/internal/tools"
	"github.com/aydndglr/rick-agent/pkg/logger"
	"github.com/aydndglr/rick-agent/internal/commands"
)

func main() {
	// 1. CONFIG YÜKLE
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		logger.Error("❌ Config yüklenemedi: %v", err)
		os.Exit(1)
	}

	// 2. LOGGER SETUP (Debug modunu config'den alıyoruz ama gereksiz info'yu azalttık)
	logger.Setup(cfg.App.Debug)
	// Sadece hayatta olduğuna dair tek bir log yeterli
	logger.Success("🚀 Rick Agent V3 - Online")

	// 3. BRAIN PROVIDER SEÇİMİ
	ollama := brain.NewOllamaProvider(cfg.Brain.Ollama.BaseURL, cfg.Brain.Ollama.ModelName)
	if !ollama.HealthCheck() {
		logger.Error("❌ LLM Provider Bağlantı Hatası!")
		os.Exit(1)
	}
	var llm brain.LLMProvider = ollama


	// 4. MEMORY LAYER
	vectorDB := memory.NewLocalVectorDB("rick_memory.json")

	// 5. TOOL REGISTRY
	reg := tools.NewRegistry(cfg.App.WorkDir)

	//6. Arka plan alarm servisini başlat
    commands.StartScheduler()

	// 7. AGENT OLUŞTURMA
	rick := agent.NewAgent(cfg, llm, reg, vectorDB)

	// 8. CONTEXT & SHUTDOWN HANDLER
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		// Kapanırken tek bir temiz log
		fmt.Println("\n⚠️  Kapatılıyor...") 
		cancel()
	}()

	// 8. WHATSAPP LISTENER BAŞLAT
	if cfg.Communication.Whatsapp.Enabled {
		wa := communication.NewWhatsappListener(cfg.Communication.Whatsapp.AdminPhone, rick)
		// Burayı da sadeleştirdik
		logger.Success("👂 WhatsApp Dinleniyor...")
		go wa.Start(ctx)
	}

	// 9. OTONOM TEST DÖNGÜSÜ İPTAL EDİLDİ ❌
	// Artık Rick sadece emir bekler.

	// Uygulamanın kapanmasını engelle
	<-ctx.Done()
}