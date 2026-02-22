package coding

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
)

type DevStudioTool struct {
	WorkspaceDir string
	PipPath      string
	PythonPath   string
}

func NewDevStudio(workspaceDir, pipPath, pythonPath string) *DevStudioTool {
	return &DevStudioTool{WorkspaceDir: workspaceDir, PipPath: pipPath, PythonPath: pythonPath}
}

func (t *DevStudioTool) Name() string { return "dev_studio" }

func (t *DevStudioTool) Description() string {
	return "OTONOM GELİŞTİRME ORTAMI (IDE). Sıfırdan Python kodu yazmak, kütüphane kurmak ve kodu GERÇEKTE çalıştırıp test etmek için bu makroyu kullan. Kod hata verirse çıktıyı okuyup kendini düzelt."
}

func (t *DevStudioTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"actions": map[string]interface{}{
				"type":        "array",
				"description": "Sırasıyla yapılacak geliştirme adımları. Önce 'write' ile kodu yaz, gerekirse 'install' ile kütüphane kur, en son 'run' ile kodu kesinlikle test et!",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"step":     map[string]interface{}{"type": "string", "enum": []string{"write", "install", "run"}},
						"filename": map[string]interface{}{"type": "string", "description": "Sadece 'write' için dosya adı (Örn: script.py)"},
						"code":     map[string]interface{}{"type": "string", "description": "Sadece 'write' için Python kodunun tamamı"},
						"packages": map[string]interface{}{"type": "string", "description": "Sadece 'install' için paket adları (Örn: 'requests pandas')"},
						"command":  map[string]interface{}{"type": "string", "description": "Sadece 'run' için terminal komutu (Örn: 'python script.py test')"},
					},
					"required": []string{"step"},
				},
			},
		},
		"required": []string{"actions"},
	}
}

func (t *DevStudioTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	actionsRaw, ok := args["actions"].([]interface{})
	if !ok {
		return "", fmt.Errorf("HATA: 'dev_studio' için 'actions' dizisi (array) gereklidir")
	}

	var report strings.Builder
	report.WriteString(fmt.Sprintf("💻 RICK DEV STUDIO RAPORU\n%s\n", strings.Repeat("=", 30)))

	os.MkdirAll(t.WorkspaceDir, 0755)

	// Arka planda hızlı syntax kontrolü yapan yardımcı fonksiyon
	checkSyntax := func(path string) error {
		chkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(chkCtx, t.PythonPath, "-m", "py_compile", path)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("Syntax Hatası:\n%s", string(out))
		}
		return nil
	}

	for i, actRaw := range actionsRaw {
		act, ok := actRaw.(map[string]interface{})
		if !ok { continue }

		step, _ := act["step"].(string)
		
		switch step {
		case "write":
			filename, _ := act["filename"].(string)
			code, _ := act["code"].(string)
			
			if filename == "" || code == "" {
				return "", fmt.Errorf("Adım %d [write] Başarısız: filename veya code eksik", i+1)
			}

			// 🛡️ DİNAMİK YASAKLI İTHALAT (IMPORT) KONTROLÜ - HARD GUARDRAIL
			systemTools := []string{
				"browser", "sys_exec", "fs_read", "fs_list", "fs_write", 
				"fs_delete", "dev_studio", "edit_python_tool", "delete_python_tool",
			}
			
			for _, tool := range systemTools {
				importPattern := fmt.Sprintf("import %s", tool)
				fromPattern := fmt.Sprintf("from %s", tool)
				
				if strings.Contains(code, importPattern) || strings.Contains(code, fromPattern) {
					errStr := fmt.Sprintf("🚨 KURAL İHLALİ: '%s' bir Go SİSTEM ARACIDIR, Python kütüphanesi DEĞİLDİR! Kod içine import edemezsin. Lütfen '%s' importunu sil ve veriyi aracı kullanarak önceden çekip, Python'a parametre/değişken olarak ver.", tool, tool)
					report.WriteString(fmt.Sprintf("❌ Adım %d [write]: %s\n", i+1, errStr))
					return report.String(), fmt.Errorf(errStr) 
				}
			}

			finalCode := formatPythonCode(filename, "Otonom Araç", code)

			fullPath := filepath.Join(t.WorkspaceDir, filepath.Base(filename))
			logger.Action("📝 [%d] Zırhlı kod yazılıyor: %s", i+1, fullPath)
			
			if err := os.WriteFile(fullPath, []byte(finalCode), 0644); err != nil {
				return "", fmt.Errorf("Adım %d [write] Dosya yazılamadı: %v", i+1, err)
			}

			// 🚀 Syntax Kontrolü (Erken Hata Yakalama)
			if err := checkSyntax(fullPath); err != nil {
				os.Remove(fullPath) // Bozuk dosyayı sil, çöp bırakma
				errStr := fmt.Sprintf("🚨 SYNTAX HATASI YAKALANDI!\nYazdığın yeni kod sözdizimi hatası içeriyor. İşlem iptal edildi.\nHata:\n%v", err)
				return errStr, nil // Hata mesajını dön ki Rick düzeltsin
			}

			report.WriteString(fmt.Sprintf("✅ Adım %d [write]: %s zırhlanarak diske kaydedildi ve Syntax testini geçti.\n", i+1, filename))

		case "install":
			packages, _ := act["packages"].(string)
			if packages == "" { continue }

			// 🛡️ DİNAMİK YASAKLI PAKET KONTROLÜ
			systemTools := []string{
				"browser", "sys_exec", "fs_read", "fs_list", "fs_write", 
				"fs_delete", "dev_studio", "edit_python_tool", "delete_python_tool",
			}
			
			pkgList := strings.Fields(packages)
			for _, pkg := range pkgList {
				for _, tool := range systemTools {
					if pkg == tool {
						errStr := fmt.Sprintf("🚨 KURAL İHLALİ: '%s' bir Go SİSTEM ARACIDIR, PyPI'da bulunan bir Python kütüphanesi DEĞİLDİR! 'pip install %s' yapılamaz.", tool, tool)
						logger.Error("❌ [%d/3] KURAL İHLALİ YAKALANDI: %s paketi kurulamaz!", i+1, tool)
						report.WriteString(fmt.Sprintf("❌ Adım %d [install]: %s\n", i+1, errStr))
						return report.String(), fmt.Errorf(errStr) 
					}
				}
			}

			logger.Info("⏳ [%d/3] PIP İŞLEMİ BAŞLADI: %s kütüphaneleri indiriliyor...", i+1, packages)
			fmt.Printf("   -> İnternet hızına göre bu işlem 1-2 dakika sürebilir. Lütfen bekleyin...\n")
			
			logger.Action("📦 [%d] Kütüphaneler kuruluyor: %s", i+1, packages)
			
			installCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			out, err := installDependencies(installCtx, t.PipPath, packages)
			cancel()

			if err != nil {
				logger.Error("❌ [%d/3] PIP KURULUMU PATLADI! Hata: %v", i+1, err)
				errStr := fmt.Sprintf("Adım %d [install] Başarısız: %v\nÇıktı: %s", i+1, err, out)
				report.WriteString("❌ " + errStr + "\n")
				return report.String(), fmt.Errorf(errStr)
			}
			logger.Success("✅ [%d/3] KÜTÜPHANELER HAZIR: %s başarıyla sanal ortama (VENV) kuruldu.", i+1, packages)
			report.WriteString(fmt.Sprintf("✅ Adım %d [install]: Paketler kuruldu.\n", i+1))


		case "run":
			command, _ := act["command"].(string)
			if command == "" { continue }
			
			logger.Info("⏳ [%d/3] TEST MOTORU ÇALIŞIYOR: '%s' komutu ateşlendi...", i+1, command)
			fmt.Printf("   -> Çıktılar canlı olarak bekleniyor...\n")

			logger.Action("⚙️ [%d] Test çalıştırılıyor: %s", i+1, command)
			
			runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			parts := strings.Fields(command)
			var cmd *exec.Cmd

			if parts[0] == "python" || parts[0] == "python3" {
				absPythonPath, _ := filepath.Abs(t.PythonPath)
				runArgs := append([]string{"-u"}, parts[1:]...)
				cmd = exec.CommandContext(runCtx, absPythonPath, runArgs...)
			} else {
				cmd = exec.CommandContext(runCtx, parts[0], parts[1:]...)
			}
			
			cmd.Dir = t.WorkspaceDir
			out, err := cmd.CombinedOutput()
			cancel()

			outputStr := strings.TrimSpace(string(out))

			// 🚀 Akıllı Hata Yönlendirmesi
			if err != nil {
				systemPrompt := fmt.Sprintf(`🚨 KOD TEST SIRASINDA ÇÖKTÜ!
Yazdığın kod çalışırken aşağıdaki hatayı verdi.

💻 Go Sistem Hatası: %v
💻 Terminal Hata Çıktısı (RickOS Traceback):
%s

🧠 [SİSTEM YÖNERGESİ]:
1. Hata mesajını incele. Sorun mantıkta mı, eksik kütüphanede mi?
2. Gerekiyorsa 'browser' aracıyla hatayı internette araştır.
3. Sorunu tespit ettikten sonra 'edit_python_tool' aracını kullanarak dosyayı (replace veya write ile) onar ve tekrar test et!`, err, outputStr)

				report.WriteString(fmt.Sprintf("❌ Adım %d [run]: KOD ÇÖKTÜ.\n", i+1))
				return systemPrompt, nil
			}

			var lastFilename string
			for j := i; j >= 0; j-- {
				if a, ok := actionsRaw[j].(map[string]interface{}); ok && a["step"] == "write" {
					lastFilename, _ = a["filename"].(string)
					break
				}
			}
			if lastFilename != "" {
				updateRegistryFile(t.WorkspaceDir, lastFilename, "Otonom olarak geliştirilen ve testten geçen araç.")
			}

			report.WriteString(fmt.Sprintf("✅ Adım %d [run]: Başarılı.\nTerminal Çıktısı:\n%s\n", i+1, outputStr))
		}
	}

	report.WriteString(strings.Repeat("-", 30) + "\n🏁 Dev Studio Makrosu başarıyla tamamlandı.")
	return report.String(), nil
}