package broadcast

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jtfrow/termi/internal/ssh"
)

type broadcastState int

const (
	stateSelect  broadcastState = iota // picking sessions
	stateCommand                       // typing command
	stateResults                       // viewing per-host output
)

// Model is the broadcast screen — select sessions, type a command, see results.
type Model struct {
	sessions []ssh.ActiveSessionInfo
	selected map[int]bool // keyed by TabIndex
	cursor   int

	cmdInput textinput.Model
	results  []ssh.BroadcastResult
	detailIdx int // which result row is expanded

	state  broadcastState
	width  int
	height int
	err    string
}

func New(width, height int) Model {
	ti := textinput.New()
	ti.Placeholder = "command to broadcast to all selected sessions…"
	ti.Prompt = "$ "
	ti.CharLimit = 1024
	return Model{
		selected:  make(map[int]bool),
		cmdInput:  ti,
		width:     width,
		height:    height,
	}
}

// SetSessions refreshes the session list and resets selection state.
func (m *Model) SetSessions(sessions []ssh.ActiveSessionInfo) {
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].SessionName < sessions[j].SessionName
	})
	m.sessions = sessions
	m.selected = make(map[int]bool)
	m.state = stateSelect
	m.cursor = 0
	m.results = nil
	m.err = ""
}

func (m *Model) SetSize(w, h int) { m.width = w; m.height = h }

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case stateSelect:
			return m.updateSelect(msg)
		case stateCommand:
			return m.updateCommand(msg)
		case stateResults:
			return m.updateResults(msg)
		}
	case BroadcastDoneMsg:
		m.results = msg.Results
		m.state = stateResults
		m.detailIdx = -1
		return m, nil
	}
	return m, nil
}

func (m Model) updateSelect(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m, func() tea.Msg { return BroadcastCancelledMsg{} }
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.sessions)-1 {
			m.cursor++
		}
	case " ":
		if len(m.sessions) > 0 {
			idx := m.sessions[m.cursor].TabIndex
			m.selected[idx] = !m.selected[idx]
		}
	case "a":
		// select all / deselect all
		allSelected := m.allSelected()
		for _, s := range m.sessions {
			m.selected[s.TabIndex] = !allSelected
		}
	case "enter":
		if !m.hasSelection() {
			m.err = "Select at least one session (space to toggle, a = all)"
			return m, nil
		}
		m.err = ""
		m.cmdInput.SetValue("")
		m.cmdInput.Focus()
		m.state = stateCommand
		return m, textinput.Blink
	}
	return m, nil
}

func (m Model) updateCommand(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.cmdInput.Blur()
		m.state = stateSelect
		return m, nil
	case "enter":
		cmd := strings.TrimSpace(m.cmdInput.Value())
		if cmd == "" {
			m.err = "Command cannot be empty"
			return m, nil
		}
		m.err = ""
		tabs := m.selectedTabs()
		return m, func() tea.Msg {
			return RunBroadcastMsg{TabIndexes: tabs, Command: cmd}
		}
	}
	var cmd tea.Cmd
	m.cmdInput, cmd = m.cmdInput.Update(msg)
	return m, cmd
}

func (m Model) updateResults(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return m, func() tea.Msg { return BroadcastCancelledMsg{} }
	case "r":
		// re-broadcast: go back to command screen
		m.state = stateCommand
		m.cmdInput.Focus()
		return m, textinput.Blink
	case "up", "k":
		if m.detailIdx > 0 {
			m.detailIdx--
		} else {
			m.detailIdx = len(m.results) - 1
		}
	case "down", "j":
		if m.detailIdx < len(m.results)-1 {
			m.detailIdx++
		} else {
			m.detailIdx = 0
		}
	case "enter":
		// toggle detail panel
		if m.detailIdx < 0 {
			m.detailIdx = 0
		} else {
			m.detailIdx = -1
		}
	}
	return m, nil
}

// ── View ─────────────────────────────────────────────────────────────────────

var (
	styleTitle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	styleSelected = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true)
	styleActive   = lipgloss.NewStyle().Foreground(lipgloss.Color("#06B6D4")).Bold(true)
	styleMuted    = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	styleError    = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	styleSuccess  = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	styleFailure  = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	styleHint     = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	styleBorder   = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#374151")).
			Padding(0, 1)
)

func (m Model) View() string {
	switch m.state {
	case stateSelect:
		return m.viewSelect()
	case stateCommand:
		return m.viewCommand()
	case stateResults:
		return m.viewResults()
	}
	return ""
}

func (m Model) viewSelect() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("  Broadcast") + "\n\n")

	if len(m.sessions) == 0 {
		b.WriteString(styleMuted.Render("  No active sessions. Connect to sessions first.") + "\n")
		b.WriteString("\n" + styleHint.Render("  esc — back"))
		return b.String()
	}

	b.WriteString(styleMuted.Render(fmt.Sprintf("  %d session(s) active  ·  %d selected\n\n", len(m.sessions), m.countSelected())))

	for i, s := range m.sessions {
		check := "[ ]"
		nameStyle := lipgloss.NewStyle()
		if m.selected[s.TabIndex] {
			check = styleSelected.Render("[✓]")
			nameStyle = styleSelected
		}
		cursor := "  "
		if i == m.cursor {
			cursor = styleActive.Render("▶ ")
		}
		since := time.Since(s.ConnectedAt).Truncate(time.Second)
		line := fmt.Sprintf("%s%s %s  %s  connected %s",
			cursor, check,
			nameStyle.Render(s.SessionName),
			styleMuted.Render(s.Host),
			styleMuted.Render(since.String()))
		b.WriteString("  " + line + "\n")
	}

	if m.err != "" {
		b.WriteString("\n  " + styleError.Render("✗ "+m.err) + "\n")
	}

	b.WriteString("\n  " + styleHint.Render("space select  a all/none  enter next  esc back"))
	return b.String()
}

func (m Model) viewCommand() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("  Broadcast") + "\n\n")

	selected := m.countSelected()
	b.WriteString(styleMuted.Render(fmt.Sprintf("  Sending to %d session(s)\n\n", selected)))
	b.WriteString("  " + m.cmdInput.View() + "\n")

	if m.err != "" {
		b.WriteString("\n  " + styleError.Render("✗ "+m.err) + "\n")
	}

	b.WriteString("\n  " + styleHint.Render("enter send  esc back to selection"))
	return b.String()
}

func (m Model) viewResults() string {
	var b strings.Builder
	cmd := m.cmdInput.Value()
	b.WriteString(styleTitle.Render("  Broadcast Results") + "\n")
	b.WriteString(styleMuted.Render("  $ "+cmd) + "\n\n")

	for i, r := range m.results {
		status := styleSuccess.Render("✓")
		if r.Err != nil || r.ExitCode != 0 {
			status = styleFailure.Render("✗")
		}
		exitStr := fmt.Sprintf("exit %d", r.ExitCode)
		if r.Err != nil {
			exitStr = "error"
		}
		header := fmt.Sprintf("  %s  %-20s  %-20s  %s",
			status,
			r.SessionName,
			r.Output[:min(len(r.Output), 40)],
			styleMuted.Render(exitStr),
		)
		if i == m.detailIdx {
			header = styleActive.Render("▶") + header[1:]
		}
		b.WriteString(header + "\n")

		if i == m.detailIdx {
			out := r.Output
			if r.Err != nil {
				out = r.Err.Error()
			}
			if out == "" {
				out = "(no output)"
			}
			b.WriteString(styleBorder.Width(m.width - 6).Render(out) + "\n")
		}
	}

	b.WriteString("\n  " + styleHint.Render("j/k navigate  enter expand  r re-broadcast  esc back"))
	return b.String()
}

func (m Model) hasSelection() bool {
	for _, v := range m.selected {
		if v {
			return true
		}
	}
	return false
}

func (m Model) allSelected() bool {
	if len(m.sessions) == 0 {
		return false
	}
	for _, s := range m.sessions {
		if !m.selected[s.TabIndex] {
			return false
		}
	}
	return true
}

func (m Model) countSelected() int {
	n := 0
	for _, v := range m.selected {
		if v {
			n++
		}
	}
	return n
}

func (m Model) selectedTabs() []int {
	var out []int
	for idx, sel := range m.selected {
		if sel {
			out = append(out, idx)
		}
	}
	sort.Ints(out)
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
