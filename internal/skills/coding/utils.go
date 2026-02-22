package coding

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
)

type ToolMeta struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Filename    string `json:"filename"`
}

// 🛡️ REGISTRY KİLİDİ: Aynı anda iki işlem registry dosyasını bozmasın diye.
var registryMutex sync.Mutex

// updateRegistryFile: Registry'ye yeni araç ekler veya günceller (Thread-Safe & Atomic)
func updateRegistryFile(workspaceDir, filename, desc string) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	regPath := filepath.Join(workspaceDir, "registry.json")
	var tools []ToolMeta
	
	if data, err := os.ReadFile(regPath); err == nil {
		json.Unmarshal(data, &tools)
	}

	found := false
	for i, t := range tools {
		if t.Filename == filename {
			if desc != "" { tools[i].Description = desc }
			tools[i].Name = strings.TrimSuffix(filename, ".py")
			found = true
			break
		}
	}
	if !found {
		tools = append(tools, ToolMeta{
			Name:        strings.TrimSuffix(filename, ".py"),
			Description: desc,
			Filename:    filename,
		})
	}

	data, _ := json.MarshalIndent(tools, "", "  ")
	
	// 🚀 ATOMIC WRITE (Sıfır Veri Kaybı Garantisi)
	tempPath := regPath + ".tmp"
	os.WriteFile(tempPath, data, 0644)
	os.Rename(tempPath, regPath) // İşletim sistemi bu işlemi milisaniyede ve güvenle yapar
}

// removeFromRegistryFile: Registry'den aracı siler (Thread-Safe & Atomic)
func removeFromRegistryFile(workspaceDir, filename string) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	regPath := filepath.Join(workspaceDir, "registry.json")
	var tools []ToolMeta
	
	if data, err := os.ReadFile(regPath); err == nil {
		json.Unmarshal(data, &tools)
	}

	var newTools []ToolMeta
	for _, t := range tools {
		if t.Filename != filename {
			newTools = append(newTools, t)
		}
	}

	data, _ := json.MarshalIndent(newTools, "", "  ")
	
	// 🚀 ATOMIC WRITE
	tempPath := regPath + ".tmp"
	os.WriteFile(tempPath, data, 0644)
	os.Rename(tempPath, regPath)
}

// installDependencies: VENV içine kütüphaneleri akıllıca kurar (Gömülü modülleri atlar)
func installDependencies(ctx context.Context, pipPath string, packagesStr string) (string, error) {
	if strings.TrimSpace(packagesStr) == "" {
		return "", nil
	}

	// 🧠 AKILLI FİLTRE: Python'un gömülü modüllerini pip ile kurmaya çalışıp çökmesini engeller
	builtInModules := map[string]bool{
		"os": true, "sys": true, "json": true, "math": true, "time": true, 
		"datetime": true, "re": true, "random": true, "collections": true, "io": true,
		"urllib": true, "subprocess": true, "shutil": true, "pathlib": true,
	}

	rawPkgs := strings.Fields(packagesStr)
	var packages []string
	for _, pkg := range rawPkgs {
		pkg = strings.TrimSpace(pkg)
		if pkg != "" && !builtInModules[pkg] {
			packages = append(packages, pkg)
		}
	}
	
	if len(packages) == 0 {
		return "Sadece gömülü modüller talep edildi, PIP kurulumu atlandı.", nil
	}

	logger.Action("📦 Kütüphaneler VENV'e enjekte ediliyor: %v", packages)
	
	pipArgs := append([]string{"install", "--quiet"}, packages...)
	cmd := exec.CommandContext(ctx, pipPath, pipArgs...) 
	
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("PIP HATASI: Kütüphane adı yanlış olabilir.\nDetay: %v", err)
	}
	
	logger.Success("✅ Bağımlılıklar hazır.")
	return string(out), nil
}

// formatPythonCode: Rick'in koduna "God Mode" zırhı giydirir.
func formatPythonCode(filename, desc, code string) string {
	// Markdown kalıntılarını temizle
	code = strings.TrimPrefix(code, "```python")
	code = strings.TrimPrefix(code, "```")
	code = strings.TrimSuffix(code, "```")
	code = strings.TrimSpace(code)

	// 🚀 ZIRHLI WRAPPER:
	wrapper := `"""
NAME: %s
DESCRIPTION: %s
"""
import sys, json, os, io, traceback

# 1. Windows UTF-8 ve Standart Çıktı Ayarı (Encoding Çökmelerini Önler)
if sys.platform == "win32":
    try:
        sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')
        sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8', errors='replace')
    except: pass

# 2. RickOS Özel Hata Yakalayıcı (Crash Handler)
def rickos_excepthook(exc_type, exc_value, exc_traceback):
    print("\n" + "="*50)
    print("🚨 [RICK_OS FATAL PYTHON ERROR] 🚨")
    print("="*50)
    print(f"Hata Türü : {exc_type.__name__}")
    print(f"Detay     : {exc_value}")
    print("\n--- Traceback (Hataya Giden Yol) ---")
    traceback.print_exception(exc_type, exc_value, exc_traceback, file=sys.stdout)
    print("="*50)

sys.excepthook = rickos_excepthook

# 3. Akıllı Argüman Yakalama (JSON Zırhı)
args = {}
if len(sys.argv) > 1:
    raw_arg = " ".join(sys.argv[1:]) # Tüm argümanları birleştir
    try:
        data = json.loads(raw_arg)
        args = data.get("args", data) if isinstance(data, dict) else data
    except Exception as e:
        args = {"raw_input": raw_arg}

# ==========================================
# --- RICK'S AUTONOMOUS CODE START ---
# ==========================================
%s
# ==========================================
# --- RICK'S AUTONOMOUS CODE END ---
# ==========================================
`
	return fmt.Sprintf(wrapper, filename, desc, code)
}