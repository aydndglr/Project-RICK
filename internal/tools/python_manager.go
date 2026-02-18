package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/aydndglr/rick-agent/pkg/logger"
)

type PythonManager struct {
	BaseDir string
	VenvDir string
	ExePath string
}

// NewPythonManager: Ortamı kontrol eder ve hazırlar
func NewPythonManager(baseDir string) (*PythonManager, error) {
	venvPath := filepath.Join(baseDir, "rick_venv")
	
	pm := &PythonManager{
		BaseDir: baseDir,
		VenvDir: venvPath,
	}

	// 1. İşletim sistemine göre Python yolunu belirle
	if runtime.GOOS == "windows" {
		pm.ExePath = filepath.Join(venvPath, "Scripts", "python.exe")
	} else {
		pm.ExePath = filepath.Join(venvPath, "bin", "python")
	}

	// 2. Sanal ortam var mı?
	if _, err := os.Stat(pm.ExePath); os.IsNotExist(err) {
		logger.Warn("🐍 Sanal Python ortamı bulunamadı, oluşturuluyor...")
		if err := pm.createVenv(); err != nil {
			return nil, err
		}
	} else {
		logger.Success("🐍 Python Sanal Ortamı (Venv) Aktif: %s", pm.VenvDir)
	}

	return pm, nil
}

func (pm *PythonManager) createVenv() error {
	// Global python var mı?
	cmdCheck := exec.Command("python", "--version")
	if err := cmdCheck.Run(); err != nil {
		return fmt.Errorf("sistemde Python yüklü değil! Lütfen Python kur")
	}

	// Venv oluştur
	logger.Action("⚙️ Sanal ortam kuruluyor (biraz sürebilir)...")
	cmd := exec.Command("python", "-m", "venv", pm.VenvDir)
	cmd.Dir = pm.BaseDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("venv oluşturma hatası: %v\n%s", err, string(out))
	}

	logger.Success("✅ Sanal ortam hazır! Gerekli kütüphaneler izole edildi.")
	
	// Temel pip güncellemesi (Opsiyonel)
	// pm.Execute("-m", "pip", "install", "--upgrade", "pip")
	
	return nil
}

// Execute: Sanal ortamdaki Python ile komut çalıştırır
func (pm *PythonManager) Execute(args ...string) (string, error) {
	cmd := exec.Command(pm.ExePath, args...)
	// cmd.Dir = pm.BaseDir // İsteğe bağlı
	
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("python hatası: %v\nÇıktı: %s", err, string(out))
	}
	return string(out), nil
}

// GetInterpreterPath: Diğer araçların kullanması için exe yolunu döner
func (pm *PythonManager) GetInterpreterPath() string {
	return pm.ExePath
}