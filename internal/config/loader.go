package config

import (
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

func Load(dataDir string) (*Config, error) {
	v := viper.New()

	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".local", "share", "termi")
	}

	cfgDir := filepath.Join(os.Getenv("HOME"), ".config", "termi")
	v.AddConfigPath(cfgDir)
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	setDefaults(v, dataDir)

	v.SetEnvPrefix("TERMI")
	v.AutomaticEnv()

	// Map standard env vars directly
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		v.Set("ai.anthropic_api_key", key)
	}
	if host := os.Getenv("OLLAMA_HOST"); host != "" {
		v.Set("ai.ollama_host", host)
	}
	if url := os.Getenv("OPENAI_BASE_URL"); url != "" {
		v.Set("ai.openai_base_url", url)
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		v.Set("ai.openai_api_key", key)
	}

	_ = v.ReadInConfig() // missing config file is fine

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func setDefaults(v *viper.Viper, dataDir string) {
	v.SetDefault("data_dir", dataDir)

	v.SetDefault("ssh.connect_timeout", 10*time.Second)
	v.SetDefault("ssh.keepalive_interval", 30*time.Second)
	v.SetDefault("ssh.default_port", 22)

	v.SetDefault("ai.ollama_host", "http://localhost:11434")
	v.SetDefault("ai.ollama_model", "llama3")
	v.SetDefault("ai.claude_model", "claude-sonnet-4-6")
	v.SetDefault("ai.context_lines", 100)
	v.SetDefault("ai.openai_model", "gpt-4o")

	v.SetDefault("ui.escape_key", "ctrl+b")
	v.SetDefault("ui.theme", "dark")
	v.SetDefault("ui.scrollback", 5000)
}
