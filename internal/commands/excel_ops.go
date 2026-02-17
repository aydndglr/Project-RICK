package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"os"

	"github.com/xuri/excelize/v2"
)

// --- 1. EXCEL READ ---
type ExcelReadCommand struct{ BaseDir string }

func (c *ExcelReadCommand) Name() string { return "excel_read" }
func (c *ExcelReadCommand) Description() string { return "Excel dosyasını okur ve içeriğini metin olarak döner." }

func (c *ExcelReadCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Okunacak Excel dosyasının yolu.",
			},
			"sheet": map[string]interface{}{
				"type":        "string",
				"description": "Okunacak sayfa adı (varsayılan 'Sheet1').",
			},
		},
		"required": []string{"path"},
	}
}

func (c *ExcelReadCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok { return "", fmt.Errorf("eksik parametre: path") }
	sheet, _ := args["sheet"].(string)
	if sheet == "" { sheet = "Sheet1" }

	fullPath := filepath.Join(c.BaseDir, path)
	f, err := excelize.OpenFile(fullPath)
	if err != nil { return "", fmt.Errorf("excel açılamadı: %v", err) }
	defer f.Close()

	rows, err := f.GetRows(sheet)
	if err != nil { return "", fmt.Errorf("sayfa okunamadı: %v", err) }

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 EXCEL VERİSİ (%s):\n", path))
	limit := 20
	for i, row := range rows {
		if i >= limit {
			sb.WriteString("... (devamı var)")
			break
		}
		sb.WriteString(strings.Join(row, " | ") + "\n")
	}
	return sb.String(), nil
}

// --- 2. EXCEL WRITE ---
type ExcelWriteCommand struct{ BaseDir string }

func (c *ExcelWriteCommand) Name() string { return "excel_write" }
func (c *ExcelWriteCommand) Description() string { return "Yeni bir Excel dosyası oluşturur ve verileri yazar." }

func (c *ExcelWriteCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Oluşturulacak dosyanın yolu.",
			},
			"data": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{"type": "string"},
				},
				"description": "Satırlardan oluşan veri listesi (Örn: [['Ad', 'Soyad'], ['Rick', 'Sanchez']]).",
			},
		},
		"required": []string{"path", "data"},
	}
}

func (c *ExcelWriteCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// 1. Parametre Kontrolü
	path, ok := args["path"].(string)
	dataRaw, okD := args["data"].([]interface{})
	if !ok || !okD {
		return "", fmt.Errorf("hata: 'path' ve 'data' parametreleri zorunludur")
	}

	// 2. KRİTİK YOL DÜZELTMESİ (Absolute vs Relative)
	var fullPath string
	if filepath.IsAbs(path) {
		fullPath = path
	} else {
		fullPath = filepath.Join(c.BaseDir, path)
	}

	// 3. Hedef klasörün varlığından emin ol (Yoksa Excelize hata verir)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("hedef dizin oluşturulamadı: %v", err)
	}

	// 4. Excel Dosyasını Oluştur
	f := excelize.NewFile()
	defer f.Close()
	
	sheet := "Sheet1"
	// f.NewSheet(sheet) // Varsayılan olarak Sheet1 zaten oluşur

	// 5. Verileri Satır Satır Yaz
	for i, rowRaw := range dataRaw {
		if rowItems, ok := rowRaw.([]interface{}); ok {
			// Satırın başlangıç hücresini belirle (A1, A2, A3...)
			cell, err := excelize.CoordinatesToCellName(1, i+1)
			if err != nil {
				continue
			}
			// Satırı komple bas
			if err := f.SetSheetRow(sheet, cell, &rowItems); err != nil {
				return "", fmt.Errorf("satır %d yazılamadı: %v", i+1, err)
			}
		}
	}

	// 6. Kaydet
	if err := f.SaveAs(fullPath); err != nil {
		return "", fmt.Errorf("excel dosyası kaydedilemedi: %v", err)
	}

	return fmt.Sprintf("✅ Excel başarıyla oluşturuldu:\nYol: %s", fullPath), nil
}