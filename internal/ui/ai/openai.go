package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cyberslacks/termi/internal/config"
)

// OpenAIClient speaks the OpenAI Chat Completions API (streaming SSE).
// Compatible with OpenWebUI, LiteLLM, LocalAI, vLLM, Groq, Together, etc.
type OpenAIClient struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

func NewOpenAIClient(cfg *config.AIConfig) *OpenAIClient {
	if cfg.OpenAIBaseURL == "" {
		return nil
	}
	return &OpenAIClient{
		baseURL: strings.TrimRight(cfg.OpenAIBaseURL, "/"),
		apiKey:  cfg.OpenAIAPIKey,
		model:   cfg.OpenAIModel,
		client:  &http.Client{Timeout: 0}, // streaming — no client-level timeout
	}
}

func (o *OpenAIClient) Available() bool { return o != nil }

// ModelName returns a short label for the UI.
func (o *OpenAIClient) ModelName() string {
	if o == nil {
		return ""
	}
	return o.model
}

// ── request / response types ──────────────────────────────────────────────────

type openAIChatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatReq struct {
	Model    string          `json:"model"`
	Messages []openAIChatMsg `json:"messages"`
	Stream   bool            `json:"stream"`
}

type openAIDelta struct {
	Content string `json:"content"`
}

type openAIChoice struct {
	Delta        openAIDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

type openAIChunk struct {
	Choices []openAIChoice `json:"choices"`
}

// Ask sends the prompt and streams tokens via onChunk.
func (o *OpenAIClient) Ask(ctx context.Context, prompt string, terminalContext string, onChunk func(string)) (string, error) {
	body, err := json.Marshal(openAIChatReq{
		Model: o.model,
		Messages: []openAIChatMsg{
			{Role: "system", Content: buildSystemPrompt(terminalContext)},
			{Role: "user", Content: prompt},
		},
		Stream: true,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai-compatible API returned HTTP %d", resp.StatusCode)
	}

	var fullText strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := line[6:]
		if payload == "[DONE]" {
			break
		}
		var chunk openAIChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 {
			text := chunk.Choices[0].Delta.Content
			if text != "" {
				onChunk(text)
				fullText.WriteString(text)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fullText.String(), fmt.Errorf("openai stream read: %w", err)
	}
	return fullText.String(), nil
}
