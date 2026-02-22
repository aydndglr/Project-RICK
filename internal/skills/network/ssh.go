package network

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SSHSession: Aktif bağlantıyı, tünelleri ve akıllı shell'i tutar
type SSHSession struct {
	Client       *ssh.Client
	SFTPClient   *sftp.Client
	ShellSession *ssh.Session
	Stdin        io.WriteCloser
	Stdout       io.Reader
	Host         string
	User         string
	Password     string
	LastOutput   strings.Builder // Çıktıları biriktirdiğimiz yer
	LastActive   time.Time       // Son aktivite zamanı
	mu           sync.Mutex      // Eşzamanlı erişim güvenliği için
}

var (
	sshSessions = make(map[string]*SSHSession)
	sshMu       sync.Mutex
)

type SSHTool struct{}

func (t *SSHTool) Name() string { return "ssh_tool" }

func (t *SSHTool) Description() string {
	return "Uzak sunucu yönetimi. 'connect' ile tünel açar, 'exec' ile açık olan tünelden komut gönderir (Sudo destekler). 'terminal' komutu ile mevcut ekran çıktısını verir."
}

func (t *SSHTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action":   map[string]interface{}{"type": "string", "enum": []string{"connect", "exec", "upload", "download", "close", "terminal"}},
			"host":     map[string]interface{}{"type": "string", "description": "Sunucu IP/Host"},
			"user":     map[string]interface{}{"type": "string", "description": "Kullanıcı adı"},
			"password": map[string]interface{}{"type": "string", "description": "SSH ve Sudo şifresi"},
			"key_path": map[string]interface{}{"type": "string", "description": "PEM/Key yolu"},
			"command":  map[string]interface{}{"type": "string", "description": "Gönderilecek komut (exec için)"},
			"local":    map[string]interface{}{"type": "string", "description": "Yerel dosya yolu"},
			"remote":   map[string]interface{}{"type": "string", "description": "Uzak dosya yolu"},
		},
		"required": []string{"action", "host"},
	}
}

func (t *SSHTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)
	host, _ := args["host"].(string)
	user, _ := args["user"].(string)
	password, _ := args["password"].(string)

	logger.Info("🛠️ SSH Aksiyonu: [%s] -> %s", strings.ToUpper(action), host)

	sshMu.Lock()
	session, exists := sshSessions[host]
	sshMu.Unlock()

	// --- 1. BAĞLANTIYI KAPATMA ---
	if action == "close" {
		if exists {
			logger.Action("🔌 %s tüneli ve shell oturumu kapatılıyor...", host)
			if session.ShellSession != nil { session.ShellSession.Close() }
			if session.SFTPClient != nil { session.SFTPClient.Close() }
			session.Client.Close()
			sshMu.Lock()
			delete(sshSessions, host)
			sshMu.Unlock()
			return "🔌 Bağlantı tamamen kapatıldı.", nil
		}
		return "⚠️ Kapatılacak aktif bir bağlantı yok.", nil
	}

	// --- 2. BAĞLANTI VE SARMALANMIŞ SHELL KURULUMU ---
	if !exists {
		logger.Action("📡 Yeni interaktif tünel inşa ediliyor: %s@%s", user, host)
		keyPath, _ := args["key_path"].(string)

		var auth []ssh.AuthMethod
		if keyPath != "" {
			key, err := os.ReadFile(keyPath)
			if err != nil { return "", err }
			signer, err := ssh.ParsePrivateKey(key)
			if err != nil { return "", err }
			auth = append(auth, ssh.PublicKeys(signer))
		} else {
			auth = append(auth, ssh.Password(password))
		}

		config := &ssh.ClientConfig{
			User: user,
			Auth: auth,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         15 * time.Second,
		}

		client, err := ssh.Dial("tcp", host+":22", config)
		if err != nil { return "", fmt.Errorf("Bağlantı Hatası: %v", err) }

		// 🐚 İNTERAKTİF SHELL BAŞLATMA
		shellSess, err := client.NewSession()
		if err != nil { return "", err }

		modes := ssh.TerminalModes{
			ssh.ECHO:          0,
			ssh.TTY_OP_ISPEED: 14400,
			ssh.TTY_OP_OSPEED: 14400,
		}
		if err := shellSess.RequestPty("xterm", 80, 40, modes); err != nil {
			return "", err
		}

		stdin, _ := shellSess.StdinPipe()
		stdout, _ := shellSess.StdoutPipe()

		if err := shellSess.Shell(); err != nil {
			return "", err
		}

		sftpClient, _ := sftp.NewClient(client)
		session = &SSHSession{
			Client:       client,
			SFTPClient:   sftpClient,
			ShellSession: shellSess,
			Stdin:        stdin,
			Stdout:       stdout,
			Host:         host,
			User:         user,
			Password:     password,
			LastActive:   time.Now(),
		}

		// 👂 ARKA PLANDA DİNLEME VE SUDO YÖNETİMİ
		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := scanner.Text()
				
				session.mu.Lock()
				session.LastOutput.WriteString(line + "\n")
				session.mu.Unlock()

				logger.Info("📺 [SSH-%s] %s", host, line)

				// Sudo Şifre Yakalayıcı
				lowerLine := strings.ToLower(line)
				if strings.Contains(lowerLine, "password") || strings.Contains(lowerLine, "parola") {
					logger.Info("🔑 Şifre isteniyor, gönderiliyor...")
					fmt.Fprintln(stdin, session.Password)
				}
			}
		}()

		sshMu.Lock()
		sshSessions[host] = session
		sshMu.Unlock()
		logger.Success("🚀 Sunucu sarmalandı ve tünel açık.")
	}

	session.LastActive = time.Now()

	// --- 3. EYLEMLER ---
	switch action {
	case "terminal":
		session.mu.Lock()
		output := session.LastOutput.String()
		session.mu.Unlock()
		return fmt.Sprintf("📖 [TERMİNAL GÖRÜNÜMÜ - %s]\n%s", host, output), nil

	case "exec":
		cmdStr, _ := args["command"].(string)
		
		session.mu.Lock()
		session.LastOutput.Reset()
		session.mu.Unlock()

		logger.Action("💻 Komut gönderiliyor: %s", cmdStr)
		fmt.Fprintln(session.Stdin, cmdStr)

		// Çıktının gelmesi için kısa bir bekleme (Sudo vb. etkileşimler için)
		time.Sleep(2 * time.Second) 
		
		session.mu.Lock()
		res := session.LastOutput.String()
		session.mu.Unlock()

		return fmt.Sprintf("💻 [%s] Sonuç:\n%s", host, res), nil

	case "upload":
		localPath, _ := args["local"].(string)
		remotePath, _ := args["remote"].(string)
		if strings.HasSuffix(remotePath, "/") {
			remotePath = filepath.Join(remotePath, filepath.Base(localPath))
		}
		src, _ := os.Open(localPath)
		defer src.Close()
		dst, _ := session.SFTPClient.Create(remotePath)
		defer dst.Close()
		io.Copy(dst, src)
		return fmt.Sprintf("📤 Yüklendi: %s", remotePath), nil

	case "download":
		localPath, _ := args["local"].(string)
		remotePath, _ := args["remote"].(string)
		if info, err := os.Stat(localPath); err == nil && info.IsDir() {
			localPath = filepath.Join(localPath, filepath.Base(remotePath))
		}
		src, _ := session.SFTPClient.Open(remotePath)
		defer src.Close()
		dst, _ := os.Create(localPath)
		defer dst.Close()
		io.Copy(dst, src)
		return fmt.Sprintf("📥 İndirildi: %s", localPath), nil
	}

	return "✅ İşlem tamamlandı.", nil
}