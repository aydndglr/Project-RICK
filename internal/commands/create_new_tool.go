package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Registry Interface (Circular Dependency olmaması için interface tanımlıyoruz)
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
	return "Sık kullanılacak veya özel bir işlem için YENİ BİR ARAÇ oluşturur."
}

// Parameters: Rick'e bu aracın hangi parametreleri (input) kabul ettiğini anlatan şema.
func (c *CreateNewToolCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"tool_name": map[string]interface{}{
				"type":        "string",
				"description": "Yeni aracın adı (boşluksuz, ingilizce karakter, örn: resim_sayac)",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Bu aracın ne işe yaradığına dair kısa açıklama",
			},
			"python_code": map[string]interface{}{
				"type":        "string",
				"description": "Aracın çalıştıracağı Python kodu. Argümanları 'args' değişkeninden kullanmalı.",
			},
		},
		"required": []string{"tool_name", "description", "python_code"},
	}
}

func (c *CreateNewToolCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	name, _ := args["tool_name"].(string)
	desc, _ := args["description"].(string)
	code, _ := args["python_code"].(string)

	if name == "" || desc == "" || code == "" {
		return "", fmt.Errorf("eksik parametre: tool_name, description ve python_code zorunlu")
	}

	// Güvenlik ve Format
	name = strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	fileName := name + ".py"
	fullPath := filepath.Join(c.ToolsDir, fileName)

	// Python koduna otomatik olarak argüman okuma şablonu ekleyelim
	finalCode := c.wrapPythonCode(code)

	// Dosyayı Kaydet
	if err := os.WriteFile(fullPath, []byte(finalCode), 0644); err != nil {
		return "", fmt.Errorf("araç dosyası yazılamadı: %v", err)
	}

	// Sisteme Kaydet (Canlı olarak hafızaya ekle)
	c.Registry.RegisterDynamicTool(name, desc, fileName)

	return fmt.Sprintf("✨ Yeni Yetenek Kazanıldı!\nAraç: %s\nAçıklama: %s\nDosya: %s\nArtık bu aracı ismiyle çağırabilirsin.", name, desc, fileName), nil
}

// wrapPythonCode: Rick'in yazdığı kodu, sistemimizle uyumlu hale getirmek için sarmalar.
func (c *CreateNewToolCommand) wrapPythonCode(rawCode string) string {
	wrapper := `
import sys
import json
import os

# Rick Agent Argüman Okuyucu
args = {}
try:
    if len(sys.argv) > 1:
        # Gelen argüman bir JSON string mi diye bakıyoruz
        raw_arg = sys.argv[1]
        try:
            args = json.loads(raw_arg)
        except:
            # Değilse düz text olarak kabul et (Bazen lazım olabilir)
            args = {"raw": raw_arg}
except:
    pass

# --- RICK'IN KODU BAŞLANGIÇ ---
` + rawCode + `
# --- RICK'IN KODU BİTİŞ ---
`
	return wrapper
}