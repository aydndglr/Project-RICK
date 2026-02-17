package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nguyenthenguyen/docx"
	"github.com/xuri/excelize/v2"
)

// ==========================================
// 🛠️ YARDIMCI ARAÇLAR (AUTO-INSTALLER)
// ==========================================

// ensurePythonLibs: Gerekli Python kütüphanelerini kontrol eder, yoksa yükler.
func ensurePythonLibs(ctx context.Context, libs ...string) error {
	// Önce pip listesi al
	cmd := exec.CommandContext(ctx, "pip", "list")
	output, err := cmd.CombinedOutput()
	installedPackages := string(output)

	if err != nil {
		// Pip yoksa veya hata verdiyse, risk almayalım ama uyaralım
		return fmt.Errorf("pip komutu çalıştırılamadı, Python yüklü mü? Hata: %v", err)
	}

	for _, lib := range libs {
		// Basit bir string kontrolü (daha karmaşık regex de olabilir ama bu yeterli)
		if !strings.Contains(strings.ToLower(installedPackages), strings.ToLower(lib)) {
			fmt.Printf("📦 Rick: '%s' eksik, otomatik yükleniyor...\n", lib)
			installCmd := exec.CommandContext(ctx, "pip", "install", lib)
			if out, err := installCmd.CombinedOutput(); err != nil {
				return fmt.Errorf("%s yüklenirken hata oluştu: %s", lib, string(out))
			}
			fmt.Printf("✅ Rick: '%s' başarıyla yüklendi.\n", lib)
		}
	}
	return nil
}

// runEmbeddedPython: Go içinden geçici Python scripti çalıştırır.
func runEmbeddedPython(ctx context.Context, scriptContent string, baseDir string) (string, error) {
	tmpPath := filepath.Join(baseDir, fmt.Sprintf("rick_script_%d.py", os.Getpid()))
	if err := os.WriteFile(tmpPath, []byte(scriptContent), 0644); err != nil {
		return "", err
	}
	defer os.Remove(tmpPath) // Temizlik

	cmd := exec.CommandContext(ctx, "python", tmpPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("script hatası: %v\nÇıktı: %s", err, string(output))
	}
	return string(output), nil
}

// ==========================================
// 📧 OUTLOOK OPERASYONLARI (COM Automation)
// ==========================================

// --- 1. OUTLOOK READ ---
type OutlookReadCommand struct { BaseDir string }

func (c *OutlookReadCommand) Name() string { return "outlook_read" }
func (c *OutlookReadCommand) Description() string { return "Outlook gelen kutusunu okur. Parametre: count (sayı)." }

func (c *OutlookReadCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// 1. Bağımlılık Kontrolü
	if err := ensurePythonLibs(ctx, "pywin32"); err != nil {
		return "", err
	}

	count := 5
	if val, ok := args["count"].(float64); ok { count = int(val) }

	// 2. Python Scripti (Outlook COM)
	// DÜZELTME: limit değişkeni tanımlandı ve UTF-8 ayarı yapıldı.
	script := fmt.Sprintf(`
import win32com.client
import sys
import io

# Türkçe karakter sorunu için UTF-8 encoding zorlaması
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

try:
    limit = %d
    outlook = win32com.client.Dispatch("Outlook.Application").GetNamespace("MAPI")
    inbox = outlook.GetDefaultFolder(6) # 6 = Inbox
    messages = inbox.Items
    messages.Sort("[ReceivedTime]", True)

    print(f"📧 SON {limit} MAIL:\n")
    i = 0
    for message in messages:
        if i >= limit: break
        try:
            # Type casting to string to avoid COM object issues
            sender = str(message.SenderName)
            subject = str(message.Subject)
            body = str(message.Body)[:100].replace('\n', ' ') + "..."
            
            print(f"🔹 KIMDEN: {sender}")
            print(f"   KONU: {subject}")
            print(f"   ÖZET: {body}\n")
            i += 1
        except Exception as inner_e:
            continue
except Exception as e:
    print(f"HATA: Outlook erişimi sağlanamadı. Detay: {e}")
`, count)

	return runEmbeddedPython(ctx, script, c.BaseDir)
}

// --- 2. OUTLOOK SEND ---
type OutlookSendCommand struct { BaseDir string }

func (c *OutlookSendCommand) Name() string { return "outlook_send" }
func (c *OutlookSendCommand) Description() string { return "Outlook üzerinden mail atar. Parametreler: to, subject, body." }

func (c *OutlookSendCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensurePythonLibs(ctx, "pywin32"); err != nil { return "", err }

	to, _ := args["to"].(string)
	subj, _ := args["subject"].(string)
	body, _ := args["body"].(string)

	if to == "" || subj == "" { return "", fmt.Errorf("eksik parametre: to ve subject zorunlu") }

	// Python Scripti
	script := fmt.Sprintf(`
import win32com.client
import sys
import io

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

try:
    outlook = win32com.client.Dispatch("Outlook.Application")
    mail = outlook.CreateItem(0)
    mail.To = "%s"
    mail.Subject = "%s"
    mail.Body = """%s"""
    mail.Send()
    print("✅ Mail başarıyla gönderildi.")
except Exception as e:
    print(f"HATA: Mail gönderilemedi. {e}")
`, to, subj, body)

	return runEmbeddedPython(ctx, script, c.BaseDir)
}

// ==========================================
// 📊 EXCEL OPERASYONLARI (Native Go)
// ==========================================

// --- 3. EXCEL READ ---
type ExcelReadCommand struct { BaseDir string }

func (c *ExcelReadCommand) Name() string { return "excel_read" }
func (c *ExcelReadCommand) Description() string { return "Excel dosyasını okur. Parametre: path, sheet." }

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

// --- 4. EXCEL WRITE ---
type ExcelWriteCommand struct { BaseDir string }

func (c *ExcelWriteCommand) Name() string { return "excel_write" }
func (c *ExcelWriteCommand) Description() string { return "Excel dosyası oluşturur. Parametre: path, data." }

func (c *ExcelWriteCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	dataRaw, okD := args["data"].([]interface{})
	if !ok || !okD { return "", fmt.Errorf("eksik parametre: path ve data") }

	fullPath := filepath.Join(c.BaseDir, path)
	f := excelize.NewFile()
	sheet := "Sheet1"

	for i, rowRaw := range dataRaw {
		if rowItems, ok := rowRaw.([]interface{}); ok {
			cell, _ := excelize.CoordinatesToCellName(1, i+1)
			f.SetSheetRow(sheet, cell, &rowItems)
		}
	}

	if err := f.SaveAs(fullPath); err != nil { return "", err }
	return fmt.Sprintf("✅ Excel oluşturuldu: %s", path), nil
}

// ==========================================
// 📝 WORD OPERASYONLARI (Hybrid)
// ==========================================

// --- 5. WORD READ ---
type WordReadCommand struct { BaseDir string }

func (c *WordReadCommand) Name() string { return "word_read" }
func (c *WordReadCommand) Description() string { return "Word dosyasını okur. Parametre: path." }

func (c *WordReadCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok { return "", fmt.Errorf("eksik parametre: path") }
	
	fullPath := filepath.Join(c.BaseDir, path)
	r, err := docx.ReadDocxFile(fullPath)
	if err != nil { return "", fmt.Errorf("dosya açılamadı: %v", err) }
	defer r.Close()

	content := r.Editable().GetContent()
	return fmt.Sprintf("📝 WORD İÇERİĞİ:\n%s", stripXML(content)), nil
}

// --- 6. WORD CREATE (Advanced Python Bridge) ---
type WordCreateCommand struct { BaseDir string }

func (c *WordCreateCommand) Name() string { return "word_create" }
func (c *WordCreateCommand) Description() string { return "Profesyonel Word dosyası oluşturur. Parametre: path, title, content (paragraf listesi)." }

func (c *WordCreateCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Go kütüphanesi yetersiz, Python'un gücünü kullanıyoruz
	if err := ensurePythonLibs(ctx, "python-docx"); err != nil { return "", err }

	path, _ := args["path"].(string)
	title, _ := args["title"].(string)
	contentRaw, _ := args["content"].([]interface{}) // Liste olarak içerik

	if path == "" { return "", fmt.Errorf("eksik parametre: path") }

	// İçeriği Python list string'ine çevir
	var contentStr strings.Builder
	contentStr.WriteString("[")
	for _, item := range contentRaw {
		contentStr.WriteString(fmt.Sprintf("r'''%v''', ", item))
	}
	contentStr.WriteString("]")

	script := fmt.Sprintf(`
from docx import Document
import sys
import io

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

try:
    doc = Document()
    doc.add_heading(r'%s', 0)

    paragraphs = %s
    for p in paragraphs:
        doc.add_paragraph(p)

    doc.save(r'%s')
    print("✅ Word dosyası oluşturuldu.")
except Exception as e:
    print(f"HATA: {e}")
`, title, contentStr.String(), filepath.Join(c.BaseDir, path))

	return runEmbeddedPython(ctx, script, c.BaseDir)
}

// --- 3. OUTLOOK ORGANIZE (DELETE / MOVE) ---
type OutlookOrganizeCommand struct { BaseDir string }

func (c *OutlookOrganizeCommand) Name() string { return "outlook_organize" }
func (c *OutlookOrganizeCommand) Description() string {
	return "Mailleri siler veya klasöre taşır. Parametreler: action ('delete' veya 'move'), subject_contains (aranacak kelime), target_folder (taşınacaksa klasör adı)."
}

func (c *OutlookOrganizeCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensurePythonLibs(ctx, "pywin32"); err != nil { return "", err }

	action, _ := args["action"].(string) // "delete" veya "move"
	filter, _ := args["subject_contains"].(string) // Rick buna "subject" dese de biz içerikte de arayacağız
	folderName, _ := args["target_folder"].(string)

	if action == "" || filter == "" {
		return "", fmt.Errorf("eksik parametre: action ve subject_contains zorunlu")
	}
	if action == "move" && folderName == "" {
		return "", fmt.Errorf("taşıma işlemi için 'target_folder' belirtmelisin")
	}

	// Python Scripti - GÜNCELLENDİ
	script := fmt.Sprintf(`
import win32com.client
import sys
import io

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

try:
    outlook = win32com.client.Dispatch("Outlook.Application").GetNamespace("MAPI")
    inbox = outlook.GetDefaultFolder(6) # Inbox
    messages = inbox.Items
    
    # Mailleri tersten tara (Silme/Taşıma sırasında index kaymasını önler)
    # Ve ReceivedTime'a göre sırala ki en son gelenlere önce baksın
    messages.Sort("[ReceivedTime]", True)

    action = "%s"
    filter_text = "%s".lower() # Küçük harfe çevir
    target_folder_name = "%s"
    
    count = 0
    checked = 0
    limit_check = 50 # Çok eski maillere gidip boğulmasın, son 50 maili kontrol etsin

    # Mesajları kopyalayarak listeye al (COM objeleri loop içinde sıkıntı çıkarabilir)
    # Performans için sadece son 50 mesaj üzerinde işlem yapalım
    msg_list = []
    for i in range(1, min(len(messages), limit_check) + 1):
         msg_list.append(messages[i])

    for message in msg_list:
        try:
            # Hem KONU hem de İÇERİK kontrolü
            subject = str(message.Subject).lower()
            body = str(message.Body).lower()
            
            # Arama kelimesi konuda VEYA içerikte geçiyor mu?
            if filter_text in subject or filter_text in body:
                if action == "delete":
                    print(f"🗑️ SİLİNDİ: {message.Subject}")
                    message.Delete()
                elif action == "move":
                    dest_folder = None
                    try:
                        dest_folder = inbox.Folders(target_folder_name)
                    except:
                        dest_folder = inbox.Folders.Add(target_folder_name)
                    
                    print(f"📂 TAŞINDI: {message.Subject} -> {target_folder_name}")
                    message.Move(dest_folder)
                
                count += 1
        except Exception as inner:
            # Bazı özel öğeler (Toplantı daveti vb.) body okurken hata verebilir
            continue

    if count == 0:
        print("⚠️ Kriterlere uygun mail bulunamadı (Son 50 mail tarandı).")
    else:
        print(f"✅ İşlem tamamlandı. Toplam {count} mail etkilendi.")

except Exception as e:
    print(f"HATA: {e}")
`, action, filter, folderName)

	return runEmbeddedPython(ctx, script, c.BaseDir)
}


func stripXML(input string) string {
	var sb strings.Builder
	inTag := false
	for _, r := range input {
		if r == '<' { inTag = true; continue }
		if r == '>' { inTag = false; sb.WriteString(" "); continue }
		if !inTag { sb.WriteRune(r) }
	}
	return strings.TrimSpace(sb.String())
}