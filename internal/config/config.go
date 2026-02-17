package config

import (
	"os"
	"gopkg.in/yaml.v3"
)

// Config: Rick Agent V2'nin tüm beyin, kol ve duyularını yöneten ana yapı.
type Config struct {
	App struct {
		Name     string `yaml:"name"`
		Version  string `yaml:"version"`
		Debug    bool   `yaml:"debug"`
		WorkDir  string `yaml:"work_dir"` // Proje dosyalarının olduğu ana dizin
	} `yaml:"app"`

	Brain struct {
		DefaultProvider string `yaml:"default_provider"` // "ollama" veya "claude"
		Ollama struct {
			BaseURL   string `yaml:"base_url"`
			ModelName string `yaml:"model_name"`
		} `yaml:"ollama"`
		Claude struct {
			APIKey    string `yaml:"api_key"`
			ModelName string `yaml:"model_name"`
		} `yaml:"claude"`
		Temperature float64 `yaml:"temperature"`
	} `yaml:"brain"`

	Capabilities struct {
		AllowShell     bool `yaml:"allow_shell"`
		AllowFileSystem bool `yaml:"allow_file_system"`
		AllowPatching  bool `yaml:"allow_patching"`
		AllowGUI       bool `yaml:"allow_gui"`
	} `yaml:"capabilities"`

	Security struct {
		RequireApproval bool     `yaml:"require_approval"` // Her işlem için onay sorsun mu?
		BlockedCommands []string `yaml:"blocked_commands"`
		SandboxEnabled  bool     `yaml:"sandbox_enabled"`
	} `yaml:"security"`

	Communication struct {
		Telegram struct {
			Enabled  bool   `yaml:"enabled"`
			Token    string `yaml:"token"`
			AdminID  int64  `yaml:"admin_id"`
		} `yaml:"telegram"`
		Whatsapp struct {
			Enabled    bool   `yaml:"enabled"`
			AdminPhone string `yaml:"admin_phone"`
		} `yaml:"whatsapp"`
	} `yaml:"communication"`
}

// LoadConfig: Belirtilen yoldaki YAML dosyasını okur ve Config yapısına çevirir.
func LoadConfig(path string) (*Config, error) {
	config := &Config{}
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(file, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}