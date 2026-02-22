package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
)

// Loader: Diskten araçları okuyup Manager'a yükler.
type Loader struct {
	Manager    *Manager
	ToolsDir   string
	PythonPath string // venv/bin/python
}

func NewLoader(mgr *Manager, toolsDir, pythonPath string) *Loader {
	return &Loader{
		Manager:    mgr,
		ToolsDir:   toolsDir,
		PythonPath: pythonPath,
	}
}

// LoadAll: Klasörü tarar ve geçerli araçları yükler.
func (l *Loader) LoadAll() error {
	// 1. Registry dosyasını oku (Metadata için)
	regPath := filepath.Join(l.ToolsDir, "registry.json")
	var registry []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Filename    string `json:"filename"`
	}

	data, err := os.ReadFile(regPath)
	if err == nil {
		json.Unmarshal(data, &registry)
	}

	// 2. Python dosyalarını bul
	entries, err := os.ReadDir(l.ToolsDir)
	if err != nil {
		return fmt.Errorf("araç klasörü okunamadı: %v", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".py") {
			continue
		}

		filename := entry.Name()
		name := strings.TrimSuffix(filename, ".py")
		desc := "Otomatik yüklenen Python aracı."

		// Registry'den açıklama bulmaya çalış
		for _, meta := range registry {
			if meta.Filename == filename {
				desc = meta.Description
				if meta.Name != "" {
					name = meta.Name
				}
				break
			}
		}

		fullPath := filepath.Join(l.ToolsDir, filename)
		
		// Adapter ile sarmala
		tool := NewPythonTool(name, desc, fullPath, l.PythonPath)
		
		// Manager'a kaydet
		l.Manager.Register(tool)
		count++
	}

	logger.Info("📂 Diskten %d adet yetenek yüklendi.", count)
	return nil
}