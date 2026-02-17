package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/aydndglr/rick-agent/internal/brain"
	"github.com/aydndglr/rick-agent/internal/commands"
	"github.com/aydndglr/rick-agent/pkg/logger"
)

// Registry: Rick'in kullanabileceği tüm komutları (araçları) barındıran merkezi depo.
type Registry struct {
	commands map[string]commands.Command
	mu       sync.RWMutex // Eşzamanlı erişim (Multitasking) için kilit
	BaseDir  string
}

// NewRegistry: Yeni bir alet çantası oluşturur ve tüm modülleri yükler.
func NewRegistry(baseDir string) *Registry {
	r := &Registry{
		commands: make(map[string]commands.Command),
		BaseDir:  baseDir,
	}

	// Tüm yetenekleri yükle
	r.registerAll()
	return r
}

// registerAll: Rick'in tüm modüllerini tek tek sisteme kaydeder.
func (r *Registry) registerAll() {
	// 1. Temel Dosya İşlemleri (Basic FS)
	r.Register(&commands.ListProjectCommand{BaseDir: r.BaseDir})
	r.Register(&commands.ReadFileCommand{BaseDir: r.BaseDir})
	r.Register(&commands.CreateFileCommand{BaseDir: r.BaseDir})

	// 2. Gelişmiş Dosya İşlemleri (Extended FS)
	r.Register(&commands.CopyPathCommand{BaseDir: r.BaseDir})
	r.Register(&commands.MovePathCommand{BaseDir: r.BaseDir})
	r.Register(&commands.DeletePathCommand{BaseDir: r.BaseDir})
	r.Register(&commands.GetFileInfoCommand{BaseDir: r.BaseDir})

	// 3. Terminal ve Sistem (Sys Ops)
	r.Register(&commands.RunCommand{})
	r.Register(&commands.OpenAppCommand{})
	r.Register(&commands.KillProcessCommand{})
	r.Register(&commands.VirusScanCommand{BaseDir: r.BaseDir})

	// 4. Yama ve Kod Düzenleme (Code Ops)
	r.Register(&commands.ApplyPatchCommand{BaseDir: r.BaseDir})
	r.Register(&commands.CodeAnalyzeCommand{BaseDir: r.BaseDir})

	// 5. Git Entegrasyonu (Git Ops)
	r.Register(&commands.GitStatusCommand{BaseDir: r.BaseDir})
	r.Register(&commands.GitCommitCommand{BaseDir: r.BaseDir})
	r.Register(&commands.GitHistoryCommand{BaseDir: r.BaseDir})

	// 6. Web ve Tarayıcı (Web Ops)
	r.Register(&commands.WebSearchCommand{})
	r.Register(&commands.DownloadFileCommand{BaseDir: r.BaseDir})
	r.Register(&commands.BrowserReadCommand{})    
	r.Register(&commands.BrowserInteractCommand{})

	// 7. Ofis ve Verimlilik (Office Ops)
	r.Register(&commands.ExcelReadCommand{BaseDir: r.BaseDir})
	r.Register(&commands.ExcelWriteCommand{BaseDir: r.BaseDir})
	r.Register(&commands.WordReadCommand{BaseDir: r.BaseDir})
	r.Register(&commands.OutlookReadCommand{BaseDir: r.BaseDir})
	r.Register(&commands.OutlookSendCommand{BaseDir: r.BaseDir})
	r.Register(&commands.WordCreateCommand{BaseDir: r.BaseDir})
	r.Register(&commands.OutlookOrganizeCommand{BaseDir: r.BaseDir})

	// 8. Arka Plan İşlemleri (Async Ops)
	r.Register(&commands.StartBackgroundTaskCommand{})
	r.Register(&commands.CheckTaskCommand{})
	r.Register(&commands.KillTaskCommand{})

	// 9. Uzaktan Erişim (SSH Ops)
	r.Register(&commands.SSHRunCommand{})
	r.Register(&commands.SSHUploadCommand{BaseDir: r.BaseDir})

	// 10. Python çalıştırma ortamı
	r.Register(&commands.PythonRunCommand{BaseDir: r.BaseDir})

	// 11. Etkileşim ve Sunum (Interaction Ops) - YENİ
	r.Register(&commands.PresentAnswerCommand{})
	r.Register(&commands.ConversationalReplyCommand{})
	r.Register(&commands.SetReminderCommand{}) 

}

// Register: Tekil bir komutu thread-safe olarak ekler.
func (r *Registry) Register(cmd commands.Command) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Komut isminin çakışıp çakışmadığını kontrol et
	if _, exists := r.commands[cmd.Name()]; exists {
		logger.Warn("⚠️ Komut çakışması algılandı, üzerine yazılıyor: %s", cmd.Name())
	}
	
	r.commands[cmd.Name()] = cmd
	// Logu çok şişirmemek için debug seviyesinde tutuyoruz
	// logger.Debug("🔧 Araç yüklendi: %s", cmd.Name())
}

// Dispatch: Gelen komut ismine göre ilgili aracı çalıştırır.
func (r *Registry) Dispatch(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	r.mu.RLock()
	cmd, exists := r.commands[name]
	r.mu.RUnlock()

	if !exists {
		// Benzer komut önerisi (Fuzzy search basit hali)
		suggestions := []string{}
		for k := range r.commands {
			if strings.Contains(k, name) || strings.Contains(name, k) {
				suggestions = append(suggestions, k)
			}
		}
		errMsg := fmt.Sprintf("Bilinmeyen komut: '%s'.", name)
		if len(suggestions) > 0 {
			errMsg += fmt.Sprintf(" Bunu mu demek istedin? %v", suggestions)
		}
		return "", fmt.Errorf(errMsg)
	}

	// Çalıştırma öncesi log
	// logger.Action("🛠️ %s çalıştırılıyor...", name)
	
	return cmd.Execute(ctx, args)
}

// GetToolboxDescription: System Prompt için metin tabanlı açıklama döner.
func (r *Registry) GetToolboxDescription() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var keys []string
	for k := range r.commands {
		keys = append(keys, k)
	}
	sort.Strings(keys) // Alfabetik sıra (Modelin kafası karışmasın)

	var sb strings.Builder
	sb.WriteString("MEVCUT ARAÇLAR (COMMANDS):\n")
	for _, k := range keys {
		cmd := r.commands[k]
		sb.WriteString(fmt.Sprintf("- %s: %s\n", cmd.Name(), cmd.Description()))
	}
	return sb.String()
}

// GetToolDefinitions: Ollama/OpenAI için JSON Schema formatında araç tanımları.
func (r *Registry) GetToolDefinitions() []brain.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tools []brain.ToolDefinition
	
	// Harita sırasız olduğu için her seferinde aynı sırayla göndermek iyidir
	var keys []string
	for k := range r.commands {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		cmd := r.commands[k]
		
		// Not: Burada her komutun parametre şemasını dinamik üretmek için
		// Command interface'ine 'GetSchema()' eklemek en doğrusudur.
		// Ancak şimdilik "Description" alanını akıllıca kullanarak modelin
		// parametreleri anlamasını sağlıyoruz.
		
		tools = append(tools, brain.ToolDefinition{
			Name:        cmd.Name(),
			Description: cmd.Description(),
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					// Modelin herhangi bir argüman gönderebilmesi için esnek bırakıyoruz.
					// Eva-9B gibi modeller Description'dan parametreleri anlar.
					"arguments": map[string]interface{}{
						"type": "object",
						"description": "Komutun gerektirdiği parametreler (path, url, content vb.)",
					},
				},
			},
		})
	}
	return tools
}

// ListCommandNames: Sadece isimleri döner (Hata ayıklama için)
func (r *Registry) ListCommandNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var names []string
	for name := range r.commands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}