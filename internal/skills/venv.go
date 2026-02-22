package skills

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
)

// PythonEnv: Sanal ortamın yollarını tutan yapı
type PythonEnv struct {
	BaseDir    string
	VenvDir    string
	PythonPath string
	PipPath    string
}

// SetupVenv: Araçlar diziniyle aynı seviyede bir Python sanal ortamı kurar.
func SetupVenv(toolsDir string) (*PythonEnv, error) {
	// toolsDir'in bulunduğu ana dizine (örn: proje kök dizini) "rick_venv" adında bir klasör oluştur
	basePath := filepath.Dir(toolsDir)
	venvDir := filepath.Join(basePath, "rick_venv")
	
	env := &PythonEnv{
		BaseDir: toolsDir,
		VenvDir: venvDir,
	}

	// İşletim sistemine göre Python ve Pip yollarını belirle
	if runtime.GOOS == "windows" {
		env.PythonPath = filepath.Join(venvDir, "Scripts", "python.exe")
		env.PipPath = filepath.Join(venvDir, "Scripts", "pip.exe")
	} else {
		env.PythonPath = filepath.Join(venvDir, "bin", "python")
		env.PipPath = filepath.Join(venvDir, "bin", "pip")
	}

	// Tools klasörü yoksa oluştur (Rick'in kodları buraya gelecek)
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		return nil, fmt.Errorf("araçlar klasörü oluşturulamadı: %v", err)
	}

	// Venv klasöründeki Python exe'si yoksa ortamı sıfırdan kur
	if _, err := os.Stat(env.PythonPath); os.IsNotExist(err) {
		logger.Action("🐍 Python Sanal Ortamı (rick_venv) kuruluyor... (Sadece ilk açılışta olur)")
		
		// Sistemdeki ana python'u kullanarak venv oluştur
		cmd := exec.Command("python", "-m", "venv", venvDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("venv oluşturulamadı: %v\nÇıktı: %s", err, string(out))
		}
		
		logger.Success("✅ İzole Python ortamı başarıyla hazırlandı: %s", venvDir)
	} else {
		logger.Debug("🐍 İzole Python ortamı aktif: %s", venvDir)
	}

	return env, nil
}