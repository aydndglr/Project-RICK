package commands

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
)

// --- 1. SSH RUN COMMAND ---

type SSHRunCommand struct {}

func (c *SSHRunCommand) Name() string { return "ssh_run" }

func (c *SSHRunCommand) Description() string {
	return "Uzak sunucuda komut çalıştırır. Parametreler: host (ip:port), user, password (veya key_path), command."
}

func (c *SSHRunCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// 1. Parametreleri Al
	host, _ := args["host"].(string)
	user, _ := args["user"].(string)
	pass, _ := args["password"].(string)
	keyPath, _ := args["key_path"].(string)
	cmd, _ := args["command"].(string)

	if host == "" || user == "" || cmd == "" {
		return "", fmt.Errorf("eksik parametre: host, user ve command zorunludur")
	}

	// 2. SSH Config Hazırla
	config := &ssh.ClientConfig{
		User: user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Test için host key kontrolünü kapatıyoruz
		Timeout: 10 * time.Second,
	}

	// Auth Yöntemini Seç (Şifre mi Key mi?)
	if keyPath != "" {
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return "", fmt.Errorf("key dosyası okunamadı: %v", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return "", fmt.Errorf("key parse hatası: %v", err)
		}
		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	} else {
		config.Auth = []ssh.AuthMethod{ssh.Password(pass)}
	}

	// 3. Bağlantıyı Kur
	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return "", fmt.Errorf("bağlantı hatası: %v", err)
	}
	defer client.Close()

	// 4. Session Oluştur
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("oturum açma hatası: %v", err)
	}
	defer session.Close()

	// 5. Komutu Çalıştır ve Çıktıyı Yakala
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(cmd); err != nil {
		return fmt.Sprintf("⚠️ Komut hata verdi:\n%s\nHata: %v", stderr.String(), err), nil
	}

	return fmt.Sprintf("✅ Sunucu Yanıtı (%s):\n%s", host, stdout.String()), nil
}

// --- 2. SSH UPLOAD COMMAND (SCP Benzeri) ---

type SSHUploadCommand struct {
    BaseDir string
}

func (c *SSHUploadCommand) Name() string { return "ssh_upload" }

func (c *SSHUploadCommand) Description() string {
	return "Yerel dosyayı sunucuya yükler. Parametreler: host, user, password, local_path, remote_path."
}

func (c *SSHUploadCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
    // Parametreler
    host, _ := args["host"].(string)
    user, _ := args["user"].(string)
    pass, _ := args["password"].(string)
    localPath, _ := args["local_path"].(string)
    remotePath, _ := args["remote_path"].(string)

    fullLocalPath := filepath.Join(c.BaseDir, localPath)
    if filepath.IsAbs(localPath) {
        fullLocalPath = localPath
    }

    // Dosyayı Oku
    file, err := os.Open(fullLocalPath)
    if err != nil {
        return "", fmt.Errorf("yerel dosya okunamadı: %v", err)
    }
    defer file.Close()

    stat, _ := file.Stat()

    // SSH Bağlantısı (Yukarıdakiyle aynı mantık)
    config := &ssh.ClientConfig{
		User: user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        Auth: []ssh.AuthMethod{ssh.Password(pass)}, // Basitlik için sadece şifre
		Timeout: 10 * time.Second,
	}

    client, err := ssh.Dial("tcp", host, config)
    if err != nil { return "", err }
    defer client.Close()

    session, err := client.NewSession()
    if err != nil { return "", err }
    defer session.Close()

    // SCP Protokolü ile dosya gönderimi
    go func() {
        w, _ := session.StdinPipe()
        defer w.Close()
        // SCP header
        fmt.Fprintln(w, "C0644", stat.Size(), filepath.Base(remotePath))
        io.Copy(w, file)
        fmt.Fprint(w, "\x00")
    }()

    if err := session.Run("/usr/bin/scp -t " + remotePath); err != nil {
        return "", fmt.Errorf("yükleme başarısız: %v", err)
    }

    return fmt.Sprintf("✅ Dosya başarıyla yüklendi: %s -> %s:%s", localPath, host, remotePath), nil
}