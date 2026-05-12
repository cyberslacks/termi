package audit

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jtfrow/termi/internal/store"
)

type actorFilter int

const (
	filterAll       actorFilter = iota
	filterUser
	filterAgent
	filterScheduler
)

var filterLabels = []string{"all", "user", "agent", "scheduler"}

// Model is the audit log browser — a scrollable table of command history.
type Model struct {
	repo    *store.AuditRepo
	entries []store.AuditEntry
	cursor  int
	offset  int  // top visible row index
	detail  bool // show expanded detail for cursor row
	filter  actorFilter
	width   int
	height  int
	loaded  bool
	err     string
}

func New(repo *store.AuditRepo, width, height int) Model {
	return Model{repo: repo, width: width, height: height}
}

func (m *Model) SetSize(w, h int) { m.width = w; m.height = h }

// LoadCmd fetches the latest 200 audit entries with the current filter.
func LoadCmd(repo *store.AuditRepo, filter actorFilter) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		f := store.AuditFilter{Limit: 200}
		if filter != filterAll {
			actor := store.ActorType(filterLabels[filter])
			f.Actor = &actor
		}
		entries, err := repo.Query(ctx, f)
		return AuditLoadedMsg{Entries: entries, Err: err}
	}
}

func (m Model) Init() tea.Cmd {
	return LoadCmd(m.repo, m.filter)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case AuditLoadedMsg:
		if msg.Err != nil {
			m.err = msg.Err.Error()
			return m, nil
		}
		m.entries = msg.Entries
		m.loaded = true
		m.err = ""
		m.cursor = 0
		m.offset = 0
		m.detail = false
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return m, func() tea.Msg { return AuditCancelledMsg{} }
		case "r":
			m.loaded = false
			return m, LoadCmd(m.repo, m.filter)
		case "f":
			m.filter = (m.filter + 1) % 4
			m.loaded = false
			return m, LoadCmd(m.repo, m.filter)
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.scrollTo(m.cursor)
			}
			m.detail = false
		case "down", "j":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
				m.scrollTo(m.cursor)
			}
			m.detail = false
		case "enter":
			m.detail = !m.detail
		case "g":
			m.cursor = 0
			m.offset = 0
			m.detail = false
		case "G":
			if len(m.entries) > 0 {
				m.cursor = len(m.entries) - 1
				m.scrollTo(m.cursor)
				m.detail = false
			}
		}
	}
	return m, nil
}

func (m *Model) scrollTo(row int) {
	visible := m.visibleRows()
	if row < m.offset {
		m.offset = row
	} else if row >= m.offset+visible {
		m.offset = row - visible + 1
	}
}

func (m Model) visibleRows() int {
	h := m.height - 6
	if m.detail {
		h -= 8
	}
	if h < 1 {
		h = 1
	}
	return h
}

// ── View ─────────────────────────────────────────────────────────────────────

var (
	styleTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	styleHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9CA3AF")).Background(lipgloss.Color("#111827"))
	styleRow    = lipgloss.NewStyle().Foreground(lipgloss.Color("#D1D5DB"))
	styleCursor = lipgloss.NewStyle().Foreground(lipgloss.Color("#F9FAFB")).Background(lipgloss.Color("#374151"))
	styleMuted  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	styleFail   = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	styleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#374151")).
			Padding(0, 1)
)

func (m Model) View() string {
	var b strings.Builder

	filterLabel := filterLabels[m.filter]
	b.WriteString(styleTitle.Render(fmt.Sprintf("  Audit Log  [filter: %s]", filterLabel)) + "\n\n")

	if m.err != "" {
		b.WriteString(styleFail.Render("  Error: "+m.err) + "\n")
		b.WriteString(styleMuted.Render("  r to retry  esc back"))
		return b.String()
	}
	if !m.loaded {
		b.WriteString(styleMuted.Render("  Loading…"))
		return b.String()
	}
	if len(m.entries) == 0 {
		b.WriteString(styleMuted.Render("  No entries yet.") + "\n\n")
		b.WriteString(styleMuted.Render("  r refresh  f cycle filter  esc back"))
		return b.String()
	}

	// Column widths
	wTime := 16
	wSession := 14
	wActor := 7
	wExit := 5
	wCmd := m.width - wTime - wSession - wActor - wExit - 12
	if wCmd < 10 {
		wCmd = 10
	}

	// Header
	header := fmt.Sprintf("  %-*s  %-*s  %-*s  %-*s  %-*s",
		wTime, "Time",
		wSession, "Session",
		wActor, "Actor",
		wCmd, "Command",
		wExit, "Exit",
	)
	b.WriteString(styleHeader.Width(m.width).Render(header) + "\n")

	// Rows
	visible := m.visibleRows()
	end := m.offset + visible
	if end > len(m.entries) {
		end = len(m.entries)
	}

	for i := m.offset; i < end; i++ {
		e := m.entries[i]
		row := renderRow(e, wTime, wSession, wActor, wCmd, wExit)
		if i == m.cursor {
			b.WriteString(styleCursor.Width(m.width).Render(row) + "\n")
		} else {
			b.WriteString(styleRow.Render(row) + "\n")
		}
	}

	// Detail panel for cursor row
	if m.detail && m.cursor < len(m.entries) {
		b.WriteString("\n")
		b.WriteString(renderDetail(m.entries[m.cursor], m.width))
		b.WriteString("\n")
	}

	total := len(m.entries)
	b.WriteString(styleMuted.Render(fmt.Sprintf("\n  %d–%d of %d  ·  j/k navigate  enter expand  f filter  r refresh  esc back", m.offset+1, end, total)))
	return b.String()
}

func renderRow(e store.AuditEntry, wTime, wSession, wActor, wCmd, wExit int) string {
	ts := e.ExecutedAt.Format("01-02 15:04:05")
	session := truncate(e.SessionName, wSession)
	actor := actorStr(e)
	cmd := truncate(e.Command, wCmd)

	exitStr := "-"
	if e.ExitCode != nil {
		exitStr = fmt.Sprintf("%d", *e.ExitCode)
	}

	return fmt.Sprintf("  %-*s  %-*s  %-*s  %-*s  %-*s",
		wTime, ts,
		wSession, session,
		wActor, actor,
		wCmd, cmd,
		wExit, exitStr,
	)
}

func actorStr(e store.AuditEntry) string {
	switch e.Actor {
	case store.ActorUser:
		return "user"
	case store.ActorAgent:
		return "agent"
	case store.ActorScheduler:
		return "sched"
	}
	return string(e.Actor)
}

func renderDetail(e store.AuditEntry, width int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("  Session:  %s (%s)\n", e.SessionName, e.Host))
	b.WriteString(fmt.Sprintf("  Command:  %s\n", e.Command))
	b.WriteString(fmt.Sprintf("  Actor:    %s  %s\n", e.Actor, e.ActorDetail))
	b.WriteString(fmt.Sprintf("  Time:     %s\n", e.ExecutedAt.Format(time.RFC3339)))
	if e.ExitCode != nil {
		b.WriteString(fmt.Sprintf("  Exit:     %d\n", *e.ExitCode))
	}
	if e.OutputSnippet != "" {
		b.WriteString("  Output:\n")
		b.WriteString(styleBorder.Width(width - 4).Render(e.OutputSnippet))
	}
	return b.String()
}

func truncate(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return s[:n-1] + "…"
}
