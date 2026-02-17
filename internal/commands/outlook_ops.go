package commands

import (
	"context"
	"fmt"
)

// --- 1. OUTLOOK READ ---
type OutlookReadCommand struct{ BaseDir string }

func (c *OutlookReadCommand) Name() string { return "outlook_read" }
func (c *OutlookReadCommand) Description() string { return "Outlook gelen kutusundaki son mailleri okur." }

func (c *OutlookReadCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"count": map[string]interface{}{
				"type":        "integer",
				"description": "Okunacak son mail sayısı (varsayılan 5).",
			},
		},
	}
}

func (c *OutlookReadCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensurePythonLibs(ctx, "pywin32"); err != nil { return "", err }
	count := 5
	if val, ok := args["count"].(float64); ok { count = int(val) }

	script := fmt.Sprintf(`
import win32com.client
import sys
import io

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
try:
    limit = %d
    outlook = win32com.client.Dispatch("Outlook.Application").GetNamespace("MAPI")
    inbox = outlook.GetDefaultFolder(6)
    messages = inbox.Items
    messages.Sort("[ReceivedTime]", True)
    print(f"📧 SON {limit} MAIL:\n")
    i = 0
    for message in messages:
        if i >= limit: break
        try:
            sender = str(message.SenderName)
            subject = str(message.Subject)
            body = str(message.Body)[:100].replace('\n', ' ') + "..."
            print(f"🔹 KIMDEN: {sender}\n   KONU: {subject}\n   ÖZET: {body}\n")
            i += 1
        except: continue
except Exception as e:
    print(f"HATA: {e}")
`, count)
	return runEmbeddedPython(ctx, script, c.BaseDir)
}

// --- 2. OUTLOOK SEND ---
type OutlookSendCommand struct{ BaseDir string }

func (c *OutlookSendCommand) Name() string { return "outlook_send" }
func (c *OutlookSendCommand) Description() string { return "Outlook üzerinden yeni e-posta gönderir." }

func (c *OutlookSendCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"to":      map[string]interface{}{"type": "string", "description": "Alıcı e-posta adresi."},
			"subject": map[string]interface{}{"type": "string", "description": "E-posta konusu."},
			"body":    map[string]interface{}{"type": "string", "description": "E-posta içeriği."},
		},
		"required": []string{"to", "subject", "body"},
	}
}

func (c *OutlookSendCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensurePythonLibs(ctx, "pywin32"); err != nil { return "", err }
	to, _ := args["to"].(string)
	subj, _ := args["subject"].(string)
	body, _ := args["body"].(string)

	script := fmt.Sprintf(`
import win32com.client
try:
    outlook = win32com.client.Dispatch("Outlook.Application")
    mail = outlook.CreateItem(0)
    mail.To = r"%s"
    mail.Subject = r"%s"
    mail.Body = r"""%s"""
    mail.Send()
    print("✅ Mail başarıyla gönderildi.")
except Exception as e:
    print(f"HATA: {e}")
`, to, subj, body)
	return runEmbeddedPython(ctx, script, c.BaseDir)
}

// --- 3. OUTLOOK ORGANIZE ---
type OutlookOrganizeCommand struct{ BaseDir string }

func (c *OutlookOrganizeCommand) Name() string { return "outlook_organize" }
func (c *OutlookOrganizeCommand) Description() string { return "Mailleri filtreleyerek siler veya klasöre taşır." }

func (c *OutlookOrganizeCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action":           map[string]interface{}{"type": "string", "enum": []string{"delete", "move"}, "description": "Yapılacak işlem."},
			"subject_contains": map[string]interface{}{"type": "string", "description": "Aranacak kelime."},
			"target_folder":    map[string]interface{}{"type": "string", "description": "Taşıma yapılacaksa hedef klasör adı."},
		},
		"required": []string{"action", "subject_contains"},
	}
}

func (c *OutlookOrganizeCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensurePythonLibs(ctx, "pywin32"); err != nil { return "", err }
	action, _ := args["action"].(string)
	filter, _ := args["subject_contains"].(string)
	folderName, _ := args["target_folder"].(string)

	script := fmt.Sprintf(`
import win32com.client
try:
    outlook = win32com.client.Dispatch("Outlook.Application").GetNamespace("MAPI")
    inbox = outlook.GetDefaultFolder(6)
    messages = inbox.Items
    messages.Sort("[ReceivedTime]", True)
    action, filter_text, target_folder_name = "%s", "%s".lower(), "%s"
    count = 0
    msg_list = [messages[i] for i in range(1, min(len(messages), 50) + 1)]
    for message in msg_list:
        try:
            if filter_text in str(message.Subject).lower() or filter_text in str(message.Body).lower():
                if action == "delete":
                    message.Delete()
                    print(f"🗑️ SİLİNDİ: {message.Subject}")
                elif action == "move":
                    try: dest = inbox.Folders(target_folder_name)
                    except: dest = inbox.Folders.Add(target_folder_name)
                    message.Move(dest)
                    print(f"📂 TAŞINDI: {message.Subject} -> {target_folder_name}")
                count += 1
        except: continue
    print(f"✅ İşlem tamamlandı. {count} mail etkilendi." if count > 0 else "⚠️ Mail bulunamadı.")
except Exception as e: print(f"HATA: {e}")
`, action, filter, folderName)
	return runEmbeddedPython(ctx, script, c.BaseDir)
}