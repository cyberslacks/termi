package ai

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/jtfrow/termi/internal/config"
)

type ClaudeClient struct {
	client *anthropic.Client
	model  string
}

func NewClaudeClient(cfg *config.AIConfig) *ClaudeClient {
	if cfg.AnthropicAPIKey == "" {
		return nil
	}
	client := anthropic.NewClient(option.WithAPIKey(cfg.AnthropicAPIKey))
	return &ClaudeClient{
		client: client,
		model:  cfg.ClaudeModel,
	}
}

func (c *ClaudeClient) Available() bool { return c != nil }

// Ask sends a prompt and streams the response via the provided chunk callback.
func (c *ClaudeClient) Ask(ctx context.Context, prompt string, terminalContext string, onChunk func(string)) (string, error) {
	systemPrompt := buildSystemPrompt(terminalContext)

	stream := c.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     anthropic.F(c.model),
		MaxTokens: anthropic.F(int64(4096)),
		System: anthropic.F([]anthropic.TextBlockParam{
			anthropic.NewTextBlock(systemPrompt),
		}),
		Messages: anthropic.F([]anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		}),
	})

	var fullText string
	for stream.Next() {
		event := stream.Current()
		switch e := event.AsUnion().(type) {
		case anthropic.ContentBlockDeltaEvent:
			if delta, ok := e.Delta.AsUnion().(anthropic.TextDelta); ok {
				onChunk(delta.Text)
				fullText += delta.Text
			}
		}
	}
	if err := stream.Err(); err != nil {
		return fullText, fmt.Errorf("claude stream: %w", err)
	}
	return fullText, nil
}

func buildSystemPrompt(terminalCtx string) string {
	base := `You are termi's AI assistant — an expert in Linux system administration, DevOps, and Ansible automation.
You help users manage remote Linux servers via SSH.

termi uses Ansible for automation. When asked to automate something, write a standard Ansible playbook:

` + "```yaml" + `
---
- name: Describe what this play does
  hosts: all
  become: true   # add when root access is needed
  tasks:
    - name: Install nginx
      ansible.builtin.package:
        name: nginx
        state: present
` + "```" + `

Rules:
- Always use fully qualified module names (ansible.builtin.*)
- Prefer idempotent modules (package, service, copy, template, user) over raw shell/command
- Use shell/command only when no module fits, and add changed_when: false when output is read-only
- Include handlers for service restarts triggered by config changes
- Explain what the playbook does before the code block
- After generating a playbook, remind the user they can press ctrl+p to save it to disk`

	if terminalCtx != "" {
		base += "\n\nCurrent terminal context:\n```\n" + terminalCtx + "\n```"
	}
	return base
}
