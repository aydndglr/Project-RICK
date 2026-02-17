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

type SSHRunCommand struct{}

func (c *SSHRunCommand) Name() string { return "ssh_run" }

func (c *SSHRunCommand) Description() string {
	return "Uzak bir sunucuya SSH ile bağlanır ve bir komut çalıştırır."
}

func (c *SSHRunCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"host": map[string]interface{}{
				"type":        "string",
				"description": "Sunucu adresi ve portu (Örn: '192.168.1.10:22').",
			},
			"user": map[string]interface{}{
				"type":        "string",
				"description": "SSH kullanıcı adı.",
			},
			"password": map[string]interface{}{
				"type":        "string",
				"description": "SSH şifresi (Anahtar kullanılmıyorsa zorunludur).",
			},
			"key_path": map[string]interface{}{
				"type":        "string",
				"description": "Özel anahtar (Private Key) dosyasının yolu (Opsiyonel).",
			},
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Uzak sunucuda çalıştırılacak komut.",
			},
		},
		"required": []string{"host", "user", "command"},
	}
}

func (c *SSHRunCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	host, _ := args["host"].(string)
	user, _ := args["user"].(string)
	pass, _ := args["password"].(string)
	keyPath, _ := args["key_path"].(string)
	cmd, _ := args["command"].(string)

	if host == "" || user == "" || cmd == "" {
		return "", fmt.Errorf("eksik parametre: host, user ve command zorunludur")
	}

	config := &ssh.ClientConfig{
		User:            user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

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

	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return "", fmt.Errorf("bağlantı hatası: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("oturum açma hatası: %v", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(cmd); err != nil {
		return fmt.Sprintf("⚠️ Komut hata verdi:\n%s\nHata: %v", stderr.String(), err), nil
	}

	return fmt.Sprintf("✅ Sunucu Yanıtı (%s):\n%s", host, stdout.String()), nil
}

// --- 2. SSH UPLOAD COMMAND ---

type SSHUploadCommand struct {
	BaseDir string
}

func (c *SSHUploadCommand) Name() string { return "ssh_upload" }

func (c *SSHUploadCommand) Description() string {
	return "Yerel bir dosyayı uzak sunucuya SCP protokolü ile yükler."
}

func (c *SSHUploadCommand) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"host": map[string]interface{}{
				"type":        "string",
				"description": "Hedef sunucu adresi (IP:Port).",
			},
			"user": map[string]interface{}{
				"type":        "string",
				"description": "SSH kullanıcı adı.",
			},
			"password": map[string]interface{}{
				"type":        "string",
				"description": "SSH şifresi.",
			},
			"local_path": map[string]interface{}{
				"type":        "string",
				"description": "Yüklenecek yerel dosyanın yolu.",
			},
			"remote_path": map[string]interface{}{
				"type":        "string",
				"description": "Sunucudaki hedef yol (örn: '/home/user/file.txt').",
			},
		},
		"required": []string{"host", "user", "local_path", "remote_path"},
	}
}

func (c *SSHUploadCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	host, _ := args["host"].(string)
	user, _ := args["user"].(string)
	pass, _ := args["password"].(string)
	localPath, _ := args["local_path"].(string)
	remotePath, _ := args["remote_path"].(string)

	fullLocalPath := filepath.Join(c.BaseDir, localPath)
	if filepath.IsAbs(localPath) {
		fullLocalPath = localPath
	}

	file, err := os.Open(fullLocalPath)
	if err != nil {
		return "", fmt.Errorf("yerel dosya okunamadı: %v", err)
	}
	defer file.Close()

	stat, _ := file.Stat()

	config := &ssh.ClientConfig{
		User:            user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return "", err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()
		fmt.Fprintln(w, "C0644", stat.Size(), filepath.Base(remotePath))
		io.Copy(w, file)
		fmt.Fprint(w, "\x00")
	}()

	if err := session.Run("/usr/bin/scp -t " + remotePath); err != nil {
		return "", fmt.Errorf("yükleme başarısız: %v", err)
	}

	return fmt.Sprintf("✅ Dosya başarıyla yüklendi: %s -> %s:%s", localPath, host, remotePath), nil
}