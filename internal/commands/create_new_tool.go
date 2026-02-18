package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Registry Interface (Circular Dependency olmaması için)
type ToolRegistrar interface {
	RegisterDynamicTool(name, desc, scriptName string)
}

type CreateNewToolCommand struct {
	BaseDir  string
	ToolsDir string
	Registry ToolRegistrar
}

func (c *CreateNewToolCommand) Name() string { return "create_new_tool" }

func (c *CreateNewToolCommand) Description() string {
	return "Sık kullanılacak veya özel bir işlem için YENİ BİR ARAÇ oluşturur ve sisteme kaydeder."
}

// Parameters: Şemayı sıkı tutuyoruz
func (c *CreateNewToolCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"tool_name": map[string]interface{}{
				"type":        "string",
				"description": "Aracın adı (Sadece harf, örn: 'resim_boyutlandir'). Uzantı yazma.",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Aracın ne işe yaradığına dair kısa açıklama.",
			},
			"python_code": map[string]interface{}{
				"type":        "string",
				"description": "Python kodu. Gerekli kütüphaneleri 'ensure_lib' ile yükleyebilirsin.",
			},
		},
		"required": []string{"tool_name", "description", "python_code"},
	}
}

func (c *CreateNewToolCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	nameRaw, _ := args["tool_name"].(string)
	desc, _ := args["description"].(string)
	code, _ := args["python_code"].(string)

	if nameRaw == "" || desc == "" || code == "" {
		return "", fmt.Errorf("eksik parametre: tool_name, description ve python_code zorunlu")
	}

	// 🛡️ GÜVENLİK DUVARI: Model ne gönderirse göndersin temizle
	// 1. Dosya yolunu temizle (../ gibi dizin atlamalarını engelle)
	name := filepath.Base(nameRaw)
	// 2. Uzantıyı kaldır (eğer model 'arac.py' yazdıysa 'arac' kalsın)
	name = strings.TrimSuffix(name, ".py")
	// 3. Küçük harfe çevir ve boşlukları temizle
	name = strings.ToLower(strings.ReplaceAll(name, " ", "_"))

	// 📂 HEDEF DİZİN GARANTİSİ
	// Eğer ToolsDir boşsa (ki olmamalı ama güvenli olsun) manuel ata
	targetDir := c.ToolsDir
	if targetDir == "" {
		targetDir = filepath.Join(c.BaseDir, "user_tools")
	}
	// Klasör yoksa oluştur
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("araç klasörü oluşturulamadı: %v", err)
	}

	fileName := name + ".py"
	fullPath := filepath.Join(targetDir, fileName)

	// 🐍 "IRONCLAD" PYTHON ŞABLONU
	finalCode := c.wrapPythonCode(code)

	// Dosyayı Kaydet
	if err := os.WriteFile(fullPath, []byte(finalCode), 0644); err != nil {
		return "", fmt.Errorf("araç dosyası yazılamadı: %v", err)
	}

	// Sisteme Kaydet (Registry Update)
	c.Registry.RegisterDynamicTool(name, desc, fileName)

	return fmt.Sprintf("✨ Yeni Yetenek Kazanıldı!\nAraç: %s\nAçıklama: %s\nDosya Konumu: user_tools/%s\nDurum: Sisteme ve Hafızaya İşlendi.", name, desc, fileName), nil
}

// wrapPythonCode: Rick'in kodunu karakter sorunlarına ve eksik kütüphanelere karşı zırhlar.
func (c *CreateNewToolCommand) wrapPythonCode(rawCode string) string {
	wrapper := `import sys
import json
import os
import io
import subprocess

# --- 1. GLOBAL WINDOWS ENCODING FIX ---
# Windows'ta Türkçe karakterlerin patlamasını önler (cp1254 -> utf-8)
if sys.platform == "win32":
    try:
        sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')
        sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8', errors='replace')
    except:
        pass

# --- 2. OTO-PIP YÜKLEYİCİ ---
def ensure_lib(lib_name):
    """Kütüphane eksikse sessizce kurar."""
    try:
        __import__(lib_name)
    except ImportError:
        try:
            # Kullanıcıya bilgi ver (stderr loglara düşer, stdout'u kirletmez)
            sys.stderr.write(f"📦 Otomatik yükleniyor: {lib_name}...\n")
            subprocess.check_call([sys.executable, "-m", "pip", "install", lib_name, "--quiet"])
            sys.stderr.write(f"✅ Yüklendi: {lib_name}\n")
        except Exception as e:
            sys.stderr.write(f"❌ Yükleme hatası ({lib_name}): {e}\n")

# --- 3. ARGÜMAN OKUYUCU ---
args = {}
try:
    if len(sys.argv) > 1:
        raw_arg = sys.argv[1]
        try:
            args = json.loads(raw_arg)
        except:
            args = {"raw": raw_arg}
except:
    pass

# --- RICK'IN ORİJİNAL KODU ---
` + rawCode + `
# --- KOD BİTİŞİ ---
`
	return wrapper
}