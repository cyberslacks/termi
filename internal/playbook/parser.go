package playbook

import (
	"fmt"
	"io"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type rawStep struct {
	Name       string `yaml:"name"`
	Command    string `yaml:"command"`
	Timeout    string `yaml:"timeout"`
	OnError    string `yaml:"on_error"`
	RetryCount int    `yaml:"retry_count"`
}

type rawPlaybook struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Tags        []string    `yaml:"tags"`
	Trusted     bool        `yaml:"trusted"`
	Variables   []Variable  `yaml:"variables"`
	Steps       []rawStep   `yaml:"steps"`
}

func ParseFile(path string) (*Playbook, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	return Parse(f)
}

func Parse(r io.Reader) (*Playbook, error) {
	var raw rawPlaybook
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse playbook yaml: %w", err)
	}
	return convert(raw)
}

func ParseString(s string) (*Playbook, error) {
	var raw rawPlaybook
	if err := yaml.Unmarshal([]byte(s), &raw); err != nil {
		return nil, fmt.Errorf("parse playbook yaml: %w", err)
	}
	return convert(raw)
}

func convert(raw rawPlaybook) (*Playbook, error) {
	if raw.Name == "" {
		return nil, fmt.Errorf("playbook must have a name")
	}
	p := &Playbook{
		Name:        raw.Name,
		Description: raw.Description,
		Tags:        raw.Tags,
		Trusted:     raw.Trusted,
		Variables:   raw.Variables,
	}
	for i, rs := range raw.Steps {
		if rs.Command == "" {
			return nil, fmt.Errorf("step %d missing command", i+1)
		}
		timeout := 30 * time.Second
		if rs.Timeout != "" {
			d, err := time.ParseDuration(rs.Timeout)
			if err != nil {
				return nil, fmt.Errorf("step %d invalid timeout %q: %w", i+1, rs.Timeout, err)
			}
			timeout = d
		}
		onErr := OnErrorAbort
		if rs.OnError != "" {
			switch OnErrorAction(rs.OnError) {
			case OnErrorAbort, OnErrorContinue, OnErrorRetry:
				onErr = OnErrorAction(rs.OnError)
			default:
				return nil, fmt.Errorf("step %d invalid on_error %q", i+1, rs.OnError)
			}
		}
		p.Steps = append(p.Steps, Step{
			Name:       rs.Name,
			Command:    rs.Command,
			Timeout:    timeout,
			OnError:    onErr,
			RetryCount: rs.RetryCount,
		})
	}
	return p, nil
}
