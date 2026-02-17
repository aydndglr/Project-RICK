package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// --- 1. COPY PATH COMMAND ---

type CopyPathCommand struct {
	BaseDir string
}

func (c *CopyPathCommand) Name() string { return "copy_path" }

func (c *CopyPathCommand) Description() string {
	return "Dosya veya klasörü kopyalar. Klasörleri recursive (içindekilerle birlikte) kopyalar."
}

func (c *CopyPathCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"source": map[string]interface{}{
				"type":        "string",
				"description": "Kopyalanacak kaynak dosya veya klasörün yolu.",
			},
			"destination": map[string]interface{}{
				"type":        "string",
				"description": "Hedef yol.",
			},
		},
		"required": []string{"source", "destination"},
	}
}

func (c *CopyPathCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	src, okS := args["source"].(string)
	dst, okD := args["destination"].(string)
	if !okS || !okD {
		return "", fmt.Errorf("eksik parametre: 'source' ve 'destination' zorunludur")
	}

	fullSrc := c.resolvePath(src)
	fullDst := c.resolvePath(dst)

	info, err := os.Stat(fullSrc)
	if err != nil {
		return "", fmt.Errorf("kaynak bulunamadı: %v", err)
	}

	if info.IsDir() {
		if err := copyDir(fullSrc, fullDst); err != nil {
			return "", fmt.Errorf("klasör kopyalama hatası: %v", err)
		}
	} else {
		if err := copyFile(fullSrc, fullDst); err != nil {
			return "", fmt.Errorf("dosya kopyalama hatası: %v", err)
		}
	}

	return fmt.Sprintf("✅ Kopyalama başarılı:\nKaynak: %s\nHedef: %s", src, dst), nil
}

// --- 2. MOVE PATH COMMAND ---

type MovePathCommand struct {
	BaseDir string
}

func (c *MovePathCommand) Name() string { return "move_path" }

func (c *MovePathCommand) Description() string {
	return "Dosya veya klasörü taşır (veya ismini değiştirir)."
}

func (c *MovePathCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"source": map[string]interface{}{
				"type":        "string",
				"description": "Taşınacak kaynak dosya veya klasör.",
			},
			"destination": map[string]interface{}{
				"type":        "string",
				"description": "Yeni hedef yol veya yeni isim.",
			},
		},
		"required": []string{"source", "destination"},
	}
}

func (c *MovePathCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	src, okS := args["source"].(string)
	dst, okD := args["destination"].(string)
	if !okS || !okD {
		return "", fmt.Errorf("eksik parametre: 'source' ve 'destination' zorunludur")
	}

	fullSrc := c.resolvePath(src)
	fullDst := c.resolvePath(dst)

	// Hedef klasör yolu yoksa oluştur
	if err := os.MkdirAll(filepath.Dir(fullDst), 0755); err != nil {
		return "", fmt.Errorf("hedef dizin oluşturulamadı: %v", err)
	}

	if err := os.Rename(fullSrc, fullDst); err != nil {
		// Farklı diskler arası taşıma gerekirse kopyala+sil yapılabilir, şimdilik basit rename.
		return "", fmt.Errorf("taşıma hatası: %v", err)
	}

	return fmt.Sprintf("✅ Taşıma/Yeniden adlandırma başarılı:\n%s -> %s", src, dst), nil
}

// --- 3. DELETE PATH COMMAND ---

type DeletePathCommand struct {
	BaseDir string
}

func (c *DeletePathCommand) Name() string { return "delete_path" }

func (c *DeletePathCommand) Description() string {
	return "Dosya veya klasörü KALICI olarak siler. DİKKAT: Geri alınamaz!"
}

func (c *DeletePathCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Silinecek dosya veya klasörün yolu.",
			},
		},
		"required": []string{"path"},
	}
}

func (c *DeletePathCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("eksik parametre: 'path' zorunludur")
	}

	fullPath := c.resolvePath(path)

	// Basit bir güvenlik önlemi: Kök dizini silmeye çalışmasın
	if fullPath == c.BaseDir || fullPath == filepath.Dir(c.BaseDir) {
		return "", fmt.Errorf("güvenlik uyarısı: Ana proje dizinini veya üst dizini silemezsin!")
	}

	if err := os.RemoveAll(fullPath); err != nil {
		return "", fmt.Errorf("silme hatası: %v", err)
	}

	return fmt.Sprintf("🗑️ Silme başarılı: %s", path), nil
}

// --- 4. GET FILE INFO COMMAND ---

type GetFileInfoCommand struct {
	BaseDir string
}

func (c *GetFileInfoCommand) Name() string { return "get_file_info" }

func (c *GetFileInfoCommand) Description() string {
	return "Dosya veya klasör hakkında detaylı bilgi (boyut, tarih, izinler) döner."
}

func (c *GetFileInfoCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Bilgisi istenen dosya veya klasörün yolu.",
			},
		},
		"required": []string{"path"},
	}
}

func (c *GetFileInfoCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("eksik parametre: 'path' zorunludur")
	}

	fullPath := c.resolvePath(path)
	info, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf("bilgi alınamadı: %v", err)
	}

	typeStr := "Dosya"
	if info.IsDir() {
		typeStr = "Klasör"
	}

	return fmt.Sprintf(
		"📄 Dosya Bilgileri:\n"+
			"- Ad: %s\n"+
			"- Tip: %s\n"+
			"- Boyut: %d bytes\n"+
			"- İzinler: %s\n"+
			"- Son Düzenleme: %s",
		info.Name(),
		typeStr,
		info.Size(),
		info.Mode(),
		info.ModTime().Format(time.RFC822),
	), nil
}

// --- YARDIMCI FONKSİYONLAR ---

// resolvePath: Absolute veya Relative yolları güvenli şekilde birleştirir
func (c *CopyPathCommand) resolvePath(p string) string   { return resolve(c.BaseDir, p) }
func (c *MovePathCommand) resolvePath(p string) string   { return resolve(c.BaseDir, p) }
func (c *DeletePathCommand) resolvePath(p string) string { return resolve(c.BaseDir, p) }
func (c *GetFileInfoCommand) resolvePath(p string) string { return resolve(c.BaseDir, p) }

func resolve(base, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(base, p)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Hedef klasörü oluştur
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}
		return copyFile(path, destPath)
	})
}