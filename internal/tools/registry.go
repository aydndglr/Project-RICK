package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/aydndglr/rick-agent/internal/brain"
	"github.com/aydndglr/rick-agent/internal/commands"
	"github.com/aydndglr/rick-agent/pkg/logger"
)

// DynamicTool: Rick'in sonradan oluşturduğu Python araçları için wrapper
type DynamicTool struct {
	NameStr    string
	DescStr    string
	ScriptPath string
	BaseDir    string
}

func (d *DynamicTool) Name() string        { return d.NameStr }
func (d *DynamicTool) Description() string { return d.DescStr }

// Parameters: Dinamik araçlar için esnek şema.
func (d *DynamicTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"properties":           map[string]interface{}{},
		"additionalProperties": true,
	}
}

func (d *DynamicTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	argsJSON, _ := json.Marshal(args)
	cmd := exec.CommandContext(ctx, "python", d.ScriptPath, string(argsJSON))
	cmd.Dir = d.BaseDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("❌ Araç Hatası (%s): %v\nÇıktı: %s", d.NameStr, err, string(output)), nil
	}
	return string(output), nil
}

// Registry: Tüm komutların toplandığı merkez
type Registry struct {
	commands map[string]commands.Command
	mu       sync.RWMutex
	BaseDir  string
	ToolsDir string // user_tools klasörü
}

func NewRegistry(baseDir string) *Registry {
	toolsDir := filepath.Join(baseDir, "user_tools")
	_ = os.MkdirAll(toolsDir, 0755)

	r := &Registry{
		commands: make(map[string]commands.Command),
		BaseDir:  baseDir,
		ToolsDir: toolsDir,
	}

	r.registerAll()
	r.LoadDynamicTools()
	return r
}

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

	// 7. Ofis ve Verimlilik (Office Ops - PARÇALANMIŞ YENİ YAPI)
	r.Register(&commands.OutlookReadCommand{BaseDir: r.BaseDir})
	r.Register(&commands.OutlookSendCommand{BaseDir: r.BaseDir})
	r.Register(&commands.OutlookOrganizeCommand{BaseDir: r.BaseDir})
	r.Register(&commands.ExcelReadCommand{BaseDir: r.BaseDir})
	r.Register(&commands.ExcelWriteCommand{BaseDir: r.BaseDir})
	r.Register(&commands.WordReadCommand{BaseDir: r.BaseDir})
	r.Register(&commands.WordCreateCommand{BaseDir: r.BaseDir})

	// 8. Arka Plan İşlemleri (Async Ops)
	r.Register(&commands.StartBackgroundTaskCommand{})
	r.Register(&commands.CheckTaskCommand{})
	r.Register(&commands.KillTaskCommand{})

	// 9. Uzaktan Erişim (SSH Ops)
	r.Register(&commands.SSHRunCommand{})
	r.Register(&commands.SSHUploadCommand{BaseDir: r.BaseDir})

	// 10. Etkileşim ve Sunum (Interaction Ops)
	r.Register(&commands.PresentAnswerCommand{})
	r.Register(&commands.ConversationalReplyCommand{})
	r.Register(&commands.SetReminderCommand{})

	// 11. Araç Oluşturucu (Meta Ops)
	r.Register(&commands.CreateNewToolCommand{BaseDir: r.BaseDir, ToolsDir: r.ToolsDir, Registry: r})
}

// --- Dynamic Tools Logic ---

func (r *Registry) LoadDynamicTools() {
	regPath := filepath.Join(r.ToolsDir, "registry.json")
	data, err := os.ReadFile(regPath)
	if os.IsNotExist(err) {
		return
	}

	var toolsMeta []map[string]string
	if err := json.Unmarshal(data, &toolsMeta); err != nil {
		logger.Error("Araç kayıt defteri okunamadı: %v", err)
		return
	}

	for _, meta := range toolsMeta {
		r.Register(&DynamicTool{
			NameStr:    meta["name"],
			DescStr:    meta["description"],
			ScriptPath: filepath.Join(r.ToolsDir, meta["script"]),
			BaseDir:    r.BaseDir,
		})
	}
}

func (r *Registry) RegisterDynamicTool(name, desc, scriptName string) {
	r.Register(&DynamicTool{
		NameStr:    name,
		DescStr:    desc,
		ScriptPath: filepath.Join(r.ToolsDir, scriptName),
		BaseDir:    r.BaseDir,
	})
	r.saveToRegistryJSON(name, desc, scriptName)
}

func (r *Registry) saveToRegistryJSON(name, desc, script string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	regPath := filepath.Join(r.ToolsDir, "registry.json")
	var toolsMeta []map[string]string
	data, err := os.ReadFile(regPath)
	if err == nil {
		json.Unmarshal(data, &toolsMeta)
	}

	found := false
	for i, t := range toolsMeta {
		if t["name"] == name {
			toolsMeta[i] = map[string]string{"name": name, "description": desc, "script": script}
			found = true
			break
		}
	}
	if !found {
		toolsMeta = append(toolsMeta, map[string]string{"name": name, "description": desc, "script": script})
	}

	newData, _ := json.MarshalIndent(toolsMeta, "", "  ")
	os.WriteFile(regPath, newData, 0644)
}

// Register: Tekil bir komutu thread-safe olarak ekler.
func (r *Registry) Register(cmd commands.Command) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commands[cmd.Name()] = cmd
}

// Dispatch: Gelen komut ismine göre ilgili aracı çalıştırır.
func (r *Registry) Dispatch(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	r.mu.RLock()
	cmd, exists := r.commands[name]
	r.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("bilinmeyen komut: '%s'", name)
	}
	return cmd.Execute(ctx, args)
}

func (r *Registry) GetToolboxDescription() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("MEVCUT ARAÇLAR (COMMANDS):\n")
	for name, cmd := range r.commands {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", name, cmd.Description()))
	}
	return sb.String()
}

func (r *Registry) GetToolDefinitions() []brain.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tools []brain.ToolDefinition
	var keys []string
	for k := range r.commands {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		cmd := r.commands[k]
		tools = append(tools, brain.ToolDefinition{
			Name:        cmd.Name(),
			Description: cmd.Description(),
			Parameters:  cmd.Parameters(), // Artık her komutun kendi Parameters() metodu var
		})
	}
	return tools
}

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