package config

import (
	"os"
	"gopkg.in/yaml.v3"
)

type Config struct {
	App struct {
		Name         string `yaml:"name"`
		ActivePrompt string `yaml:"active_prompt"`
		Version      string `yaml:"version"`
		TimeoutMinutes int `yaml:"timeout_minutes" mapstructure:"timeout_minutes"`
		Debug        bool   `yaml:"debug"`
		WorkDir      string `yaml:"work_dir"`
	} `yaml:"app"`

	Security struct {
		Level        string `yaml:"level"`         // god_mode, standard, restricted
		AutoPatching bool   `yaml:"auto_patching"` // Kendi kodunu tamir etme
	} `yaml:"security"`

	Brain struct {
		Primary struct {
			Provider    string  `yaml:"provider"`
			BaseURL     string  `yaml:"base_url"`
			ModelName   string  `yaml:"model_name"`
			Temperature float64 `yaml:"temperature"`
			NumCtx      int     `yaml:"num_ctx"`
		} `yaml:"primary"`

		Secondary struct {
			Enabled   bool   `yaml:"enabled"`
			Provider  string `yaml:"provider"`
			BaseURL   string `yaml:"base_url"`
			ModelName string `yaml:"model_name"`
		} `yaml:"secondary"`

		APIKeys struct {
			OpenAI    string `yaml:"openai"`
			Gemini    string `yaml:"gemini"`
			Anthropic string `yaml:"anthropic"`
		} `yaml:"api_keys"`
	} `yaml:"brain"`

	Communication struct {
		Whatsapp struct {
			Enabled      bool   `yaml:"enabled"`
			AdminPhone   string `yaml:"admin_phone"`
			DatabasePath string `yaml:"database_path"`
		} `yaml:"whatsapp"`
	} `yaml:"communication"`
}

// Load: Config dosyasını okur
func Load(path string) (*Config, error) {
	config := &Config{}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}