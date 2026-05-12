package ai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cyberslacks/termi/internal/config"
	"github.com/cyberslacks/termi/internal/ssh"
)

// ── backend selector ─────────────────────────────────────────────────────────

type backend int

const (
	backendClaude backend = iota
	backendOllama
	backendOpenAI
)

// ── streaming message types (internal to this package) ───────────────────────

type streamChunkMsg struct {
	epoch int
	text  string
	done  bool
	err   error
	next  tea.Cmd // nil when done
}

// ── ChatMessage ───────────────────────────────────────────────────────────────

type ChatMessage struct {
	Role    string // "user" | "assistant"
	Content string
	IsErr   bool
	At      time.Time
}

// ── Panel model ───────────────────────────────────────────────────────────────

type Model struct {
	claude  *ClaudeClient
	ollama  *OllamaClient
	openai  *OpenAIClient
	cfg     *config.AIConfig

	backend  backend
	history  []ChatMessage
	streamBuf strings.Builder
	streaming bool
	epoch    int // incremented per stream; stale chunks are discarded

	input textinput.Model

	activeSessions []ssh.ActiveSessionInfo
	termOutput     string // recent terminal lines

	savedPlaybooks []string // paths of .yml files we saved this session
	scrollOffset   int
	width, height  int
}

// AIPanelCancelledMsg is sent when the user leaves the panel.
type AIPanelCancelledMsg struct{}

// AnsiblePlaybookSavedMsg is sent after a playbook is written to disk.
type AnsiblePlaybookSavedMsg struct{ Path string }

func NewModel(cfg *config.AIConfig, width, height int) Model {
	ti := textinput.New()
	ti.Placeholder = "Ask anything… or 'write a playbook to…'"
	ti.Prompt = "> "
	ti.CharLimit = 2048
	ti.Focus()

	claude := NewClaudeClient(cfg)
	ollama := NewOllamaClient(cfg)
	openai := NewOpenAIClient(cfg)

	// Pick the first available backend in priority order
	b := backendOllama
	if openai.Available() {
		b = backendOpenAI
	}
	if claude.Available() {
		b = backendClaude
	}

	return Model{
		claude:  claude,
		ollama:  ollama,
		openai:  openai,
		cfg:     cfg,
		backend: b,
		input:   ti,
		width:   width,
		height:  height,
	}
}

func (m *Model) SetSize(w, h int) { m.width = w; m.height = h }

// SetContext is called by app.go before switching to this screen.
func (m *Model) SetContext(sessions []ssh.ActiveSessionInfo, termOutput string) {
	m.activeSessions = sessions
	m.termOutput = termOutput
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case streamChunkMsg:
		if msg.epoch != m.epoch {
			return m, nil // stale stream — discard
		}
		if msg.done {
			m.streaming = false
			content := m.streamBuf.String()
			m.streamBuf.Reset()
			entry := ChatMessage{Role: "assistant", Content: content, At: time.Now()}
			if msg.err != nil {
				entry.Content = "Error: " + msg.err.Error()
				entry.IsErr = true
			}
			m.history = append(m.history, entry)
			m.scrollToBottom()
			return m, nil
		}
		m.streamBuf.WriteString(msg.text)
		m.scrollToBottom()
		return m, msg.next

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.streaming {
				m.epoch++ // cancel current stream
				m.streaming = false
				m.streamBuf.Reset()
				m.history = append(m.history, ChatMessage{
					Role:    "assistant",
					Content: "(cancelled)",
					At:      time.Now(),
				})
				return m, nil
			}
			return m, func() tea.Msg { return AIPanelCancelledMsg{} }

		case "ctrl+c":
			if m.streaming {
				m.epoch++
				m.streaming = false
				m.streamBuf.Reset()
				return m, nil
			}

		case "ctrl+b":
			m.backend = m.nextBackend()
			return m, nil

		case "ctrl+p":
			// Save last assistant response as Ansible playbook
			return m, m.saveLastPlaybook()

		case "ctrl+k":
			// Clear history
			m.history = nil
			m.scrollOffset = 0
			return m, nil

		case "pgup":
			m.scrollOffset += m.contentHeight() / 2
			return m, nil

		case "pgdown":
			if m.scrollOffset > 0 {
				m.scrollOffset -= m.contentHeight() / 2
				if m.scrollOffset < 0 {
					m.scrollOffset = 0
				}
			}
			return m, nil

		case "enter":
			if m.streaming {
				return m, nil
			}
			prompt := strings.TrimSpace(m.input.Value())
			if prompt == "" {
				return m, nil
			}
			m.input.SetValue("")
			m.history = append(m.history, ChatMessage{Role: "user", Content: prompt, At: time.Now()})
			m.scrollToBottom()
			m.streaming = true
			m.epoch++
			return m, m.streamCmd(prompt)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// ── streaming ─────────────────────────────────────────────────────────────────

type streamItem struct {
	text string
	done bool
	err  error
}

func (m Model) streamCmd(prompt string) tea.Cmd {
	epoch := m.epoch
	ch := make(chan streamItem, 128)

	termCtx := BuildContext(m.activeSessions, nil, m.termOutput)

	go func() {
		defer close(ch)
		var ask func(context.Context, string, string, func(string)) (string, error)

		switch m.backend {
		case backendClaude:
			if !m.claude.Available() {
				ch <- streamItem{err: fmt.Errorf("Claude not configured — set ANTHROPIC_API_KEY"), done: true}
				return
			}
			ask = m.claude.Ask
		case backendOpenAI:
			if !m.openai.Available() {
				ch <- streamItem{err: fmt.Errorf("OpenAI-compatible endpoint not configured — set OPENAI_BASE_URL"), done: true}
				return
			}
			ask = m.openai.Ask
		default:
			if m.ollama == nil {
				ch <- streamItem{err: fmt.Errorf("Ollama not configured"), done: true}
				return
			}
			ask = m.ollama.Ask
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		_, err := ask(ctx, prompt, termCtx, func(chunk string) {
			ch <- streamItem{text: chunk}
		})
		ch <- streamItem{done: true, err: err}
	}()

	return nextChunk(epoch, ch)
}

func nextChunk(epoch int, ch <-chan streamItem) tea.Cmd {
	return func() tea.Msg {
		item, ok := <-ch
		if !ok {
			return streamChunkMsg{epoch: epoch, done: true}
		}
		if item.done || item.err != nil {
			return streamChunkMsg{epoch: epoch, done: true, err: item.err}
		}
		return streamChunkMsg{
			epoch: epoch,
			text:  item.text,
			next:  nextChunk(epoch, ch),
		}
	}
}

// ── playbook save ─────────────────────────────────────────────────────────────

func (m Model) saveLastPlaybook() tea.Cmd {
	// Find last assistant message that contains a yaml code block
	for i := len(m.history) - 1; i >= 0; i-- {
		msg := m.history[i]
		if msg.Role != "assistant" {
			continue
		}
		yamlSrc := extractYAMLBlock(msg.Content)
		if yamlSrc == "" {
			continue
		}
		// Found one — write to disk
		src := yamlSrc
		return func() tea.Msg {
			dir := os.Getenv("HOME")
			if dir == "" {
				dir = "."
			}
			pbDir := filepath.Join(dir, ".local", "share", "termi", "playbooks")
			_ = os.MkdirAll(pbDir, 0o750)
			f, err := os.CreateTemp(pbDir, "playbook-*.yml")
			if err != nil {
				return streamChunkMsg{done: true, err: err}
			}
			_, err = f.WriteString(src)
			f.Close()
			if err != nil {
				return streamChunkMsg{done: true, err: err}
			}
			return AnsiblePlaybookSavedMsg{Path: f.Name()}
		}
	}
	return nil
}

func extractYAMLBlock(text string) string {
	start := strings.Index(text, "```yaml")
	if start == -1 {
		start = strings.Index(text, "```yml")
	}
	if start == -1 {
		return ""
	}
	rest := text[start:]
	nl := strings.Index(rest, "\n")
	if nl == -1 {
		return ""
	}
	rest = rest[nl+1:]
	end := strings.Index(rest, "```")
	if end == -1 {
		return rest // no closing fence — return anyway
	}
	return strings.TrimSpace(rest[:end])
}

// ── view ──────────────────────────────────────────────────────────────────────

var (
	stylePanelTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	styleUser        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#06B6D4"))
	styleAssistant   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A78BFA"))
	stylePanelMuted  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	stylePanelErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	stylePanelBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#374151")).
				Padding(0, 1)
)

func (m Model) View() string {
	var b strings.Builder

	availCount := 0
	for _, be := range []backend{backendClaude, backendOpenAI, backendOllama} {
		if m.backendAvailable(be) {
			availCount++
		}
	}
	label := m.backendLabel()
	if availCount > 1 {
		label += "  ctrl+b=next"
	}

	b.WriteString(stylePanelTitle.Render(fmt.Sprintf("  AI Panel  [%s]", label)) + "\n")

	chatH := m.contentHeight()
	lines := m.renderHistory()

	// Apply scroll: we show the last chatH lines, offset by scrollOffset
	start := len(lines) - chatH - m.scrollOffset
	if start < 0 {
		start = 0
	}
	end := start + chatH
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[start:end]

	for _, l := range visible {
		b.WriteString(l + "\n")
	}

	// Stream in progress indicator
	if m.streaming {
		live := m.streamBuf.String()
		// Show last few lines of the live buffer
		liveLines := strings.Split(live, "\n")
		maxLive := 4
		if len(liveLines) > maxLive {
			liveLines = liveLines[len(liveLines)-maxLive:]
		}
		for _, ll := range liveLines {
			b.WriteString(stylePanelMuted.Render("  " + ll) + "\n")
		}
		b.WriteString(stylePanelMuted.Render("  ▌") + "\n")
	}

	// Input box
	b.WriteString("\n")
	b.WriteString(stylePanelBorder.Width(m.width - 4).Render(m.input.View()))
	b.WriteString("\n")
	b.WriteString(stylePanelMuted.Render("  enter send  ctrl+p save playbook  ctrl+k clear  ctrl+b swap backend  pgup/dn scroll  esc back"))

	return b.String()
}

func (m Model) renderHistory() []string {
	var lines []string
	w := m.width - 4
	if w < 10 {
		w = 10
	}
	for _, msg := range m.history {
		switch msg.Role {
		case "user":
			lines = append(lines, styleUser.Render("  you  ")+stylePanelMuted.Render(msg.At.Format("15:04")))
			for _, l := range wrap(msg.Content, w) {
				lines = append(lines, "  "+l)
			}
			lines = append(lines, "")
		case "assistant":
			label := styleAssistant.Render("  ai   ")
			if msg.IsErr {
				label = stylePanelErr.Render("  ai   ")
			}
			lines = append(lines, label+stylePanelMuted.Render(msg.At.Format("15:04")))
			for _, l := range wrap(msg.Content, w) {
				lines = append(lines, "  "+l)
			}
			lines = append(lines, "")
		}
	}
	return lines
}

func (m Model) contentHeight() int {
	// tabbar(1) + title(1) + input(3) + hints(1) + padding(2)
	h := m.height - 8
	if m.streaming {
		h -= 6
	}
	if h < 3 {
		h = 3
	}
	return h
}

func (m *Model) scrollToBottom() {
	m.scrollOffset = 0
}

// nextBackend cycles to the next available backend.
func (m Model) nextBackend() backend {
	order := []backend{backendClaude, backendOpenAI, backendOllama}
	cur := m.backend
	for i, b := range order {
		if b == cur {
			for j := 1; j <= len(order); j++ {
				next := order[(i+j)%len(order)]
				if m.backendAvailable(next) {
					return next
				}
			}
			break
		}
	}
	return cur
}

func (m Model) backendAvailable(b backend) bool {
	switch b {
	case backendClaude:
		return m.claude.Available()
	case backendOpenAI:
		return m.openai.Available()
	case backendOllama:
		return m.ollama != nil
	}
	return false
}

func (m Model) backendLabel() string {
	switch m.backend {
	case backendClaude:
		return "claude:" + m.cfg.ClaudeModel
	case backendOpenAI:
		label := "openai"
		if m.openai != nil && m.openai.ModelName() != "" {
			label = m.openai.ModelName()
		}
		return label
	default:
		return "ollama:" + m.cfg.OllamaModel
	}
}

// wrap breaks s into lines of at most width runes.
func wrap(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	var out []string
	for _, line := range strings.Split(s, "\n") {
		for len([]rune(line)) > width {
			out = append(out, string([]rune(line)[:width]))
			line = string([]rune(line)[width:])
		}
		out = append(out, line)
	}
	return out
}
