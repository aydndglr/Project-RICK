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

type ToolEditor struct {
	WorkspaceDir string
	PipPath      string
	PythonPath   string
}

func NewEditor(workspaceDir, pipPath, pythonPath string) *ToolEditor {
	return &ToolEditor{WorkspaceDir: workspaceDir, PipPath: pipPath, PythonPath: pythonPath}
}

func (e *ToolEditor) Name() string { return "edit_python_tool" }

func (e *ToolEditor) Description() string {
	return "Mevcut bir Python aracını GÜVENLİ ŞEKİLDE günceller. Hata çıkarsa sistem otomatik olarak rollback yapar. 'replace' ile küçük değişiklikler, 'write' ile baştan yazma yapabilirsin. Gerekirse kütüphane kur ve kesinlikle ÇALIŞTIR (run)."
}

func (e *ToolEditor) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"filename": map[string]interface{}{"type": "string", "description": "Düzenlenecek mevcut dosya adı (örn: word_counter.py)"},
			"actions": map[string]interface{}{
				"type":        "array",
				"description": "Sırasıyla yapılacak güncelleme adımları.",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"step":         map[string]interface{}{"type": "string", "enum": []string{"write", "replace", "install", "run"}},
						"code":         map[string]interface{}{"type": "string", "description": "Sadece 'write' için GÜNCELLENMİŞ Python kodunun TAMAMI."},
						"search_text":  map[string]interface{}{"type": "string", "description": "Sadece 'replace' için. Değiştirilecek olan MEVCUT kod parçası."},
						"replace_text": map[string]interface{}{"type": "string", "description": "Sadece 'replace' için. 'search_text' yerine yazılacak YENİ kod parçası."},
						"packages":     map[string]interface{}{"type": "string", "description": "Sadece 'install' için yeni eklenecek paket adları (Örn: 'requests')"},
						"command":      map[string]interface{}{"type": "string", "description": "Sadece 'run' için terminal test komutu (Örn: 'python word_counter.py test')"},
					},
					"required": []string{"step"},
				},
			},
		},
		"required": []string{"filename", "actions"},
	}
}

func (e *ToolEditor) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	filename, ok := args["filename"].(string)
	if !ok || filename == "" {
		return "", fmt.Errorf("HATA: 'filename' parametresi eksik! Hangi dosyayı düzenleyeceğini belirtmelisin")
	}

	actionsRaw, ok := args["actions"].([]interface{})
	if !ok {
		return "", fmt.Errorf("HATA: 'edit_python_tool' için 'actions' dizisi (array) gereklidir")
	}

	if !strings.HasSuffix(filename, ".py") { filename += ".py" }
	filename = filepath.Base(filename)
	fullPath := filepath.Join(e.WorkspaceDir, filename)

	// 1. DOSYA VARLIK KONTROLÜ
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return "", fmt.Errorf("HATA: '%s' adında bir dosya yok. Sıfırdan araç yapmak için 'dev_studio' kullanmalısın", filename)
	}

	// ==========================================
	// 🛡️ YEDEKLEME (BACKUP) SİSTEMİ
	// ==========================================
	backupCode, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("Güvenlik hatası: Mevcut dosyanın yedeği alınamadı: %v", err)
	}
	logger.Action("🛡️ [%s] Yedek (Backup) hafızaya alındı. Ameliyat başlıyor...", filename)

	var report strings.Builder
	report.WriteString(fmt.Sprintf("🛠️ RICK GÜVENLİ GÜNCELLEME RAPORU\n%s\n", strings.Repeat("=", 30)))

	// Arka planda hızlı syntax kontrolü yapan yardımcı fonksiyon
	checkSyntax := func(path string) error {
		chkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(chkCtx, e.PythonPath, "-m", "py_compile", path)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("Syntax Hatası:\n%s", string(out))
		}
		return nil
	}

	// 2. MAKRO ADIMLARINI İŞLE
	for i, actRaw := range actionsRaw {
		act, ok := actRaw.(map[string]interface{})
		if !ok { continue }

		step, _ := act["step"].(string)
		
		switch step {
		case "replace":
			searchText, _ := act["search_text"].(string)
			replaceText, _ := act["replace_text"].(string)
			
			if searchText == "" {
				e.rollback(fullPath, backupCode)
				return "", fmt.Errorf("Adım %d [replace]: 'search_text' boş olamaz. Rollback yapıldı", i+1)
			}

			// Kural İhlali Kontrolü
			systemTools := []string{"browser", "sys_exec", "fs_read", "fs_list", "fs_write", "fs_delete", "dev_studio", "edit_python_tool", "delete_python_tool"}
			for _, tool := range systemTools {
				if strings.Contains(replaceText, fmt.Sprintf("import %s", tool)) || strings.Contains(replaceText, fmt.Sprintf("from %s", tool)) {
					errStr := fmt.Sprintf("🚨 KURAL İHLALİ: '%s' bir Go SİSTEM ARACIDIR! Kod içine import edemezsin.", tool)
					e.rollback(fullPath, backupCode)
					return report.String(), fmt.Errorf(errStr)
				}
			}

			currentContent, _ := os.ReadFile(fullPath)
			contentStr := string(currentContent)

			if !strings.Contains(contentStr, searchText) {
				e.rollback(fullPath, backupCode)
				return "", fmt.Errorf("Adım %d [replace] HATA: 'search_text' dosya içinde bulunamadı. Tam olarak eşleştiğinden emin ol.", i+1)
			}

			newContent := strings.Replace(contentStr, searchText, replaceText, 1) // Sadece ilk eşleşmeyi değiştir

			logger.Action("📝 [%d] Kod noktası '%s' içinde değiştiriliyor...", i+1, filename)
			if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
				e.rollback(fullPath, backupCode)
				return "", fmt.Errorf("Yazma hatası. Rollback yapıldı: %v", err)
			}

			// Syntax Kontrolü
			if err := checkSyntax(fullPath); err != nil {
				e.rollback(fullPath, backupCode)
				errStr := fmt.Sprintf("🚨 SYNTAX HATASI YAKALANDI (ROLLBACK YAPILDI)!\nYazdığın yeni kod parçası sözdizimi hatasına yol açtı:\n%v", err)
				return errStr, nil
			}

			report.WriteString(fmt.Sprintf("✅ Adım %d [replace]: Belirtilen kod parçası başarıyla değiştirildi ve Syntax testini geçti.\n", i+1))

		case "write":
			code, _ := act["code"].(string)
			if code == "" {
				e.rollback(fullPath, backupCode)
				return "", fmt.Errorf("Adım %d [write]: Kod içeriği boş. Rollback yapıldı", i+1)
			}
			
			// Kural İhlali Kontrolü
			systemTools := []string{"browser", "sys_exec", "fs_read", "fs_list", "fs_write", "fs_delete", "dev_studio", "edit_python_tool", "delete_python_tool"}
			for _, tool := range systemTools {
				if strings.Contains(code, fmt.Sprintf("import %s", tool)) || strings.Contains(code, fmt.Sprintf("from %s", tool)) {
					errStr := fmt.Sprintf("🚨 KURAL İHLALİ: '%s' bir Go SİSTEM ARACIDIR! Kod içine import edemezsin.", tool)
					e.rollback(fullPath, backupCode)
					return report.String(), fmt.Errorf(errStr) 
				}
			}

			finalCode := formatPythonCode(filename, "Güncellenmiş Otonom Araç", code)

			logger.Action("📝 [%d] Yeni kod '%s' üzerine zırhlanarak yazılıyor...", i+1, filename)
			if err := os.WriteFile(fullPath, []byte(finalCode), 0644); err != nil {
				e.rollback(fullPath, backupCode)
				return "", fmt.Errorf("Yazma hatası. Rollback yapıldı: %v", err)
			}

			// Syntax Kontrolü
			if err := checkSyntax(fullPath); err != nil {
				e.rollback(fullPath, backupCode)
				errStr := fmt.Sprintf("🚨 SYNTAX HATASI YAKALANDI (ROLLBACK YAPILDI)!\nYazdığın yeni kod sözdizimi hatası içeriyor:\n%v", err)
				return errStr, nil
			}

			report.WriteString(fmt.Sprintf("✅ Adım %d [write]: Yeni kod diske güvenle yazıldı ve Syntax testini geçti.\n", i+1))

		case "install":
			packages, _ := act["packages"].(string)
			if packages == "" { continue }
			
			// 🛡️ DİNAMİK YASAKLI PAKET KONTROLÜ
			systemTools := []string{"browser", "sys_exec", "fs_read", "fs_list", "fs_write", "fs_delete", "dev_studio", "edit_python_tool", "delete_python_tool"}
			pkgList := strings.Fields(packages)
			for _, pkg := range pkgList {
				for _, tool := range systemTools {
					if pkg == tool {
						errStr := fmt.Sprintf("🚨 KURAL İHLALİ: '%s' bir Go SİSTEM ARACIDIR, PyPI'da bulunan bir Python kütüphanesi DEĞİLDİR!", tool)
						e.rollback(fullPath, backupCode)
						return report.String(), fmt.Errorf(errStr) 
					}
				}
			}

			logger.Action("📦 [%d] Yeni kütüphaneler kuruluyor: %s", i+1, packages)
			
			installCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			out, err := installDependencies(installCtx, e.PipPath, packages)
			cancel()

			if err != nil {
				e.rollback(fullPath, backupCode)
				return fmt.Sprintf("Kütüphane kurulumu başarısız (%v). Rollback yapıldı.\nÇıktı: %s", err, out), nil
			}
			report.WriteString(fmt.Sprintf("✅ Adım %d [install]: Paketler güncellendi.\n", i+1))

		case "run":
			command, _ := act["command"].(string)
			if command == "" { continue }
			
			logger.Action("🧪 [%d] Güncellenmiş kod test ediliyor: %s", i+1, command)
			
			runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			parts := strings.Fields(command)
			var cmd *exec.Cmd
			if parts[0] == "python" || parts[0] == "python3" {
				absPythonPath, _ := filepath.Abs(e.PythonPath) 
				runArgs := append([]string{"-u"}, parts[1:]...)
				cmd = exec.CommandContext(runCtx, absPythonPath, runArgs...)
			} else {
				cmd = exec.CommandContext(runCtx, parts[0], parts[1:]...)
			}
			cmd.Dir = e.WorkspaceDir
			
			out, err := cmd.CombinedOutput()
			cancel()

			outputStr := strings.TrimSpace(string(out))

			// ==========================================
			// 🚨 TEST BAŞARISIZ -> ROLLBACK VE ANTI-DÖNGÜ
			// ==========================================
			if err != nil {
				e.rollback(fullPath, backupCode)
				
				if len(outputStr) > 1000 {
					outputStr = "...(önceki loglar)...\n" + outputStr[len(outputStr)-1000:]
				}

				systemPrompt := fmt.Sprintf(`🚨 GÜNCELLEME TESTİ BAŞARISIZ (ROLLBACK YAPILDI)!
Yazdığın yeni kod (%s) çalışırken çöktüğü için sistem güvenliği gereği ESKİ ÇALIŞAN KODA geri dönüldü.

💻 Go Sistem Hatası: %v
💻 Terminal Hata Çıktısı:
%s

🧠 [SİSTEM YÖNERGESİ - KRİTİK]:
1. Hata mesajını dikkatlice oku. Sorun mantıkta mı yoksa veri yapısında mı?
2. Çözümden %%100 emin olduktan sonra 'edit_python_tool' aracını tekrar kullan!`, filename, err, outputStr)

				report.WriteString(fmt.Sprintf("❌ GÜNCELLEME PATLADI. Rollback devrede. Go Hatası: %v\n", err))
				return systemPrompt, nil
			}

			updateRegistryFile(e.WorkspaceDir, filename, "Otonom olarak revize edilmiş araç.")
			report.WriteString(fmt.Sprintf("✅ Adım %d [run]: Test Başarılı.\nTerminal Çıktısı:\n%s\n", i+1, outputStr))
		}
	}

	report.WriteString(strings.Repeat("-", 30) + "\n🏁 Araç Başarıyla Güncellendi ve Testi Geçti!")
	logger.Success("✏️ %s başarıyla revize edildi.", filename)
	return report.String(), nil
}

func (e *ToolEditor) rollback(path string, backup []byte) {
	logger.Warn("⚠️ ROLLBACK TETİKLENDİ: %s eski çalışan haline döndürülüyor.", filepath.Base(path))
	os.WriteFile(path, backup, 0644)
}