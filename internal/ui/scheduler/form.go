package scheduler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jtfrow/termi/internal/store"
)

const (
	ffName     = iota // job name
	ffCron            // cron expression
	ffSessions        // comma-separated session IDs
	ffCount
)

const (
	fvPlaybook = ffCount     // virtual: playbook selector
	fvMode     = ffCount + 1 // virtual: mode selector
	fvEnabled  = ffCount + 2 // virtual: enabled toggle
	totalFields = ffCount + 3
)

// FormModel handles creating and editing scheduled jobs.
type FormModel struct {
	inputs      [ffCount]textinput.Model
	focused     int
	editID      int64

	// Playbook selector
	playbooks   []store.DBPlaybook
	playbookIdx int

	// Mode / enabled toggles
	mode    store.ApprovalMode
	enabled bool

	width int
	err   string
}

var formKeys = struct {
	next   key.Binding
	prev   key.Binding
	submit key.Binding
	cancel key.Binding
	cycle  key.Binding
}{
	next:   key.NewBinding(key.WithKeys("tab", "down")),
	prev:   key.NewBinding(key.WithKeys("shift+tab", "up")),
	submit: key.NewBinding(key.WithKeys("ctrl+s")),
	cancel: key.NewBinding(key.WithKeys("esc")),
	cycle:  key.NewBinding(key.WithKeys("left", "right", "h", "l")),
}

func NewFormModel(width int, playbooks []store.DBPlaybook) FormModel {
	placeholders := [ffCount]string{
		ffName:     "nightly-updates",
		ffCron:     "0 2 * * *",
		ffSessions: "1,2,3  (comma-separated session IDs)",
	}
	labels := [ffCount]string{
		ffName:     "Name",
		ffCron:     "Cron",
		ffSessions: "Session IDs",
	}
	m := FormModel{
		playbooks: playbooks,
		mode:      store.ApprovalInteractive,
		enabled:   true,
		width:     width,
	}
	for i := 0; i < ffCount; i++ {
		t := textinput.New()
		t.Placeholder = placeholders[i]
		t.Prompt = "  "
		t.CharLimit = 512
		_ = labels[i]
		m.inputs[i] = t
	}
	m.inputs[ffName].Focus()
	return m
}

// LoadJob pre-fills the form for editing.
func (m *FormModel) LoadJob(j store.ScheduledJob) {
	m.editID = j.ID
	m.inputs[ffName].SetValue(j.Name)
	m.inputs[ffCron].SetValue(j.CronExpr)
	ids := make([]string, len(j.SessionIDs))
	for i, id := range j.SessionIDs {
		ids[i] = strconv.FormatInt(id, 10)
	}
	m.inputs[ffSessions].SetValue(strings.Join(ids, ","))
	m.mode = j.Mode
	m.enabled = j.Enabled
	// Find playbook index
	for i, p := range m.playbooks {
		if p.ID == j.PlaybookID {
			m.playbookIdx = i
			break
		}
	}
}

func (m *FormModel) SetWidth(w int) { m.width = w }

func (m FormModel) Init() tea.Cmd { return textinput.Blink }

func (m FormModel) Update(msg tea.Msg) (FormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, formKeys.cancel):
			return m, func() tea.Msg { return JobFormCancelledMsg{} }

		case key.Matches(msg, formKeys.submit):
			if err := m.validate(); err != nil {
				m.err = err.Error()
				return m, nil
			}
			return m, m.submitCmd()

		case key.Matches(msg, formKeys.next):
			m.advanceFocus(1)
			return m, m.focusCmd()

		case key.Matches(msg, formKeys.prev):
			m.advanceFocus(-1)
			return m, m.focusCmd()

		case key.Matches(msg, formKeys.cycle):
			return m.handleCycle(msg)
		}
	}

	// Route to focused text input
	if m.focused < ffCount {
		var cmd tea.Cmd
		m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m FormModel) handleCycle(msg tea.KeyMsg) (FormModel, tea.Cmd) {
	right := msg.String() == "right" || msg.String() == "l"
	switch m.focused {
	case fvPlaybook:
		if len(m.playbooks) == 0 {
			break
		}
		if right {
			m.playbookIdx = (m.playbookIdx + 1) % len(m.playbooks)
		} else {
			m.playbookIdx = (m.playbookIdx - 1 + len(m.playbooks)) % len(m.playbooks)
		}
	case fvMode:
		if m.mode == store.ApprovalInteractive {
			m.mode = store.ApprovalAutonomous
		} else {
			m.mode = store.ApprovalInteractive
		}
	case fvEnabled:
		m.enabled = !m.enabled
	}
	return m, nil
}

func (m FormModel) View() string {
	var b strings.Builder

	title := "  New Scheduled Job"
	if m.editID != 0 {
		title = "  Edit Scheduled Job"
	}
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED")).Render(title))
	b.WriteString("\n\n")

	labels := [ffCount]string{ffName: "Name", ffCron: "Cron Expression", ffSessions: "Session IDs"}
	for i := 0; i < ffCount; i++ {
		b.WriteString(m.renderField(labels[i], m.inputs[i].View(), m.focused == i))
	}

	// Playbook selector
	b.WriteString(m.renderSelector("Playbook", m.playbookLabel(), m.focused == fvPlaybook, "← → to change"))
	// Mode selector
	b.WriteString(m.renderSelector("Mode", string(m.mode), m.focused == fvMode, "← → to toggle"))
	// Enabled toggle
	enabledStr := "yes"
	if !m.enabled {
		enabledStr = "no"
	}
	b.WriteString(m.renderSelector("Enabled", enabledStr, m.focused == fvEnabled, "← → to toggle"))

	if m.err != "" {
		b.WriteString("\n  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render("✗ "+m.err) + "\n")
	}

	b.WriteString("\n  ")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(
		"tab/↓ next  shift+tab/↑ prev  ctrl+s save  esc cancel"))
	return b.String()
}

func (m FormModel) playbookLabel() string {
	if len(m.playbooks) == 0 {
		return "(no playbooks — create one first)"
	}
	return m.playbooks[m.playbookIdx].Name
}

func (m FormModel) renderField(label, inputView string, focused bool) string {
	ls := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Width(20)
	if focused {
		ls = ls.Foreground(lipgloss.Color("#06B6D4")).Bold(true)
	}
	return fmt.Sprintf("  %s  %s\n", ls.Render(label), inputView)
}

func (m FormModel) renderSelector(label, value string, focused bool, hint string) string {
	ls := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Width(20)
	if focused {
		ls = ls.Foreground(lipgloss.Color("#06B6D4")).Bold(true)
	}
	display := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F9FAFB")).
		Background(lipgloss.Color("#374151")).
		Padding(0, 1).
		Render("◀ " + value + " ▶")
	hintStr := ""
	if focused {
		hintStr = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("  " + hint)
	}
	return fmt.Sprintf("  %s  %s%s\n", ls.Render(label), display, hintStr)
}

func (m *FormModel) advanceFocus(delta int) {
	if m.focused < ffCount {
		m.inputs[m.focused].Blur()
	}
	m.focused = (m.focused + delta + totalFields) % totalFields
	if m.focused < ffCount {
		m.inputs[m.focused].Focus()
	}
}

func (m FormModel) focusCmd() tea.Cmd {
	if m.focused < ffCount {
		return m.inputs[m.focused].Focus()
	}
	return nil
}

func (m FormModel) validate() error {
	if strings.TrimSpace(m.inputs[ffName].Value()) == "" {
		return fmt.Errorf("Name is required")
	}
	if strings.TrimSpace(m.inputs[ffCron].Value()) == "" {
		return fmt.Errorf("Cron expression is required")
	}
	if len(m.playbooks) == 0 {
		return fmt.Errorf("No playbooks available — create a playbook first")
	}
	return nil
}

func (m FormModel) submitCmd() tea.Cmd {
	pb := m.playbooks[m.playbookIdx]
	sessionIDs := parseSessionIDs(m.inputs[ffSessions].Value())
	j := store.ScheduledJob{
		ID:         m.editID,
		Name:       strings.TrimSpace(m.inputs[ffName].Value()),
		CronExpr:   strings.TrimSpace(m.inputs[ffCron].Value()),
		PlaybookID: pb.ID,
		SessionIDs: sessionIDs,
		Mode:       m.mode,
		Enabled:    m.enabled,
	}
	return func() tea.Msg { return SaveJobMsg{Job: j} }
}

func parseSessionIDs(s string) []int64 {
	var out []int64
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if id, err := strconv.ParseInt(part, 10, 64); err == nil && id > 0 {
			out = append(out, id)
		}
	}
	return out
}
