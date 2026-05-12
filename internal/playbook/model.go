package playbook

import "time"

type OnErrorAction string

const (
	OnErrorAbort    OnErrorAction = "abort"
	OnErrorContinue OnErrorAction = "continue"
	OnErrorRetry    OnErrorAction = "retry"
)

type Variable struct {
	Name     string `yaml:"name"`
	Default  string `yaml:"default"`
	Required bool   `yaml:"required"`
}

type Step struct {
	Name       string        `yaml:"name"`
	Command    string        `yaml:"command"`
	Timeout    time.Duration `yaml:"timeout"`
	OnError    OnErrorAction `yaml:"on_error"`
	RetryCount int           `yaml:"retry_count"`
}

type Playbook struct {
	ID          int64  // populated after DB save
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Tags        []string `yaml:"tags"`
	Trusted     bool   `yaml:"trusted"`
	Variables   []Variable `yaml:"variables"`
	Steps       []Step `yaml:"steps"`
}
