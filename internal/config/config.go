package config

import "time"

type Config struct {
	DataDir string `mapstructure:"data_dir"`

	SSH SSHConfig `mapstructure:"ssh"`
	AI  AIConfig  `mapstructure:"ai"`
	UI  UIConfig  `mapstructure:"ui"`
}

type SSHConfig struct {
	ConnectTimeout  time.Duration `mapstructure:"connect_timeout"`
	KeepaliveInterval time.Duration `mapstructure:"keepalive_interval"`
	DefaultPort     int           `mapstructure:"default_port"`
}

type AIConfig struct {
	AnthropicAPIKey string `mapstructure:"anthropic_api_key"`
	OllamaHost      string `mapstructure:"ollama_host"`
	OllamaModel     string `mapstructure:"ollama_model"`
	ClaudeModel     string `mapstructure:"claude_model"`
	ContextLines    int    `mapstructure:"context_lines"` // lines of terminal output sent to AI

	// OpenAI-compatible endpoint (OpenWebUI, LiteLLM, LocalAI, vLLM, etc.)
	OpenAIBaseURL string `mapstructure:"openai_base_url"`
	OpenAIAPIKey  string `mapstructure:"openai_api_key"`
	OpenAIModel   string `mapstructure:"openai_model"`
}

type UIConfig struct {
	EscapeKey   string `mapstructure:"escape_key"` // prefix key, default ctrl+b
	Theme       string `mapstructure:"theme"`
	Scrollback  int    `mapstructure:"scrollback"` // lines of terminal scrollback
}
