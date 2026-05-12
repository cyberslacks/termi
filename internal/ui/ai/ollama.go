package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cyberslacks/termi/internal/config"
)

type OllamaClient struct {
	host   string
	model  string
	client *http.Client
}

func NewOllamaClient(cfg *config.AIConfig) *OllamaClient {
	return &OllamaClient{
		host:  strings.TrimRight(cfg.OllamaHost, "/"),
		model: cfg.OllamaModel,
		client: &http.Client{Timeout: 0}, // streaming — no timeout on client level
	}
}

func (o *OllamaClient) Available(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.host+"/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

type ollamaChatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatReq struct {
	Model    string          `json:"model"`
	Messages []ollamaChatMsg `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ollamaChatChunk struct {
	Message ollamaChatMsg `json:"message"`
	Done    bool          `json:"done"`
}

// Ask sends a prompt to Ollama and streams response chunks via onChunk.
func (o *OllamaClient) Ask(ctx context.Context, prompt string, terminalContext string, onChunk func(string)) (string, error) {
	body, err := json.Marshal(ollamaChatReq{
		Model: o.model,
		Messages: []ollamaChatMsg{
			{Role: "system", Content: buildSystemPrompt(terminalContext)},
			{Role: "user", Content: prompt},
		},
		Stream: true,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.host+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned %d", resp.StatusCode)
	}

	var fullText strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var chunk ollamaChatChunk
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}
		if chunk.Message.Content != "" {
			onChunk(chunk.Message.Content)
			fullText.WriteString(chunk.Message.Content)
		}
		if chunk.Done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return fullText.String(), fmt.Errorf("ollama stream read: %w", err)
	}
	return fullText.String(), nil
}
