package playbooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jtfrow/termi/internal/store"
)

// ── List view ─────────────────────────────────────────────────────────────────

type pbItem struct{ pb store.DBPlaybook }

func (p pbItem) Title() string {
	trust := ""
	if p.pb.TrustedAt != nil {
		trust = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render(" ✓trusted")
	}
	return p.pb.Name + trust
}

func (p pbItem) Description() string {
	desc := p.pb.Description
	if p.pb.FilePath != "" {
		desc += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(p.pb.FilePath)
	}
	return desc
}

func (p pbItem) FilterValue() string { return p.pb.Name + " " + p.pb.Description }

type listKeys struct {
	run    key.Binding
	new    key.Binding
	edit   key.Binding
	delete key.Binding
	trust  key.Binding
	back   key.Binding
}

func newListKeys() listKeys {
	return listKeys{
		run:    key.NewBinding(key.WithKeys("enter", "r"), key.WithHelp("enter", "run")),
		new:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
		edit:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		delete: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		trust:  key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "trust")),
		back:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

// ListModel is the playbook browser.
type ListModel struct {
	list       list.Model
	keys       listKeys
	width      int
	height     int
	loaded     bool
	confirmDel *store.DBPlaybook
	err        string
}

func NewListModel(width, height int) ListModel {
	keys := newListKeys()
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#F9FAFB")).
		Background(lipgloss.Color("#7C3AED")).
		Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#C4B5FD")).
		Background(lipgloss.Color("#7C3AED"))

	l := list.New(nil, delegate, width, height-4)
	l.Title = "Ansible Playbooks"
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{keys.run, keys.new, keys.edit, keys.delete, keys.trust}
	}

	return ListModel{list: l, keys: keys, width: width, height: height}
}

// LoadCmd fetches playbooks from the DB.
func LoadCmd(repo *store.PlaybookRepo) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		pbs, err := repo.List(ctx)
		return PlaybooksLoadedMsg{Playbooks: pbs, Err: err}
	}
}

func (m ListModel) Init() tea.Cmd { return nil }

func (m ListModel) Update(msg tea.Msg) (ListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case PlaybooksLoadedMsg:
		if msg.Err != nil {
			m.err = msg.Err.Error()
			return m, nil
		}
		items := make([]list.Item, len(msg.Playbooks))
		for i, p := range msg.Playbooks {
			items[i] = pbItem{pb: p}
		}
		cmd := m.list.SetItems(items)
		m.loaded = true
		m.err = ""
		return m, cmd

	case tea.KeyMsg:
		if m.confirmDel != nil {
			switch msg.String() {
			case "y", "Y":
				id := m.confirmDel.ID
				m.confirmDel = nil
				return m, func() tea.Msg { return DeletePlaybookMsg{ID: id} }
			default:
				m.confirmDel = nil
				return m, nil
			}
		}

		switch {
		case key.Matches(msg, m.keys.back):
			return m, func() tea.Msg { return PlaybookCancelledMsg{} }

		case key.Matches(msg, m.keys.new):
			return m, func() tea.Msg { return OpenPlaybookFormMsg{Playbook: nil} }

		case key.Matches(msg, m.keys.edit):
			if p, ok := m.selected(); ok {
				cp := p
				return m, func() tea.Msg { return OpenPlaybookFormMsg{Playbook: &cp} }
			}

		case key.Matches(msg, m.keys.delete):
			if p, ok := m.selected(); ok {
				m.confirmDel = &p
				return m, nil
			}

		case key.Matches(msg, m.keys.run):
			if p, ok := m.selected(); ok {
				cp := p
				// Ask app.go to open the session picker and run
				return m, func() tea.Msg { return RunPlaybookMsg{Playbook: cp} }
			}

		case key.Matches(msg, m.keys.trust):
			if p, ok := m.selected(); ok {
				cp := p
				return m, func() tea.Msg { return trustPlaybookMsg{ID: cp.ID} }
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

type trustPlaybookMsg struct{ ID int64 }

func (m ListModel) View() string {
	if m.err != "" {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render("  Error: " + m.err)
	}
	if !m.loaded {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("  Loading…")
	}
	view := m.list.View()
	if m.confirmDel != nil {
		overlay := lipgloss.NewStyle().
			Bold(true).Foreground(lipgloss.Color("#F59E0B")).
			Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#F59E0B")).
			Padding(1, 3).
			Render(fmt.Sprintf("Delete %q? [y] yes  [any] cancel", m.confirmDel.Name))
		view = lipgloss.JoinVertical(lipgloss.Left, view, "", overlay)
	}
	return view
}

func (m *ListModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.list.SetSize(w, h-4)
}

func (m ListModel) selected() (store.DBPlaybook, bool) {
	item, ok := m.list.SelectedItem().(pbItem)
	if !ok {
		return store.DBPlaybook{}, false
	}
	return item.pb, true
}

// ── Create/edit form ──────────────────────────────────────────────────────────

const (
	pfName = iota
	pfDesc
	pfPath
	pfCount
)

// FormModel handles creating and editing Ansible playbook entries.
type FormModel struct {
	inputs [pfCount]textinput.Model
	editID int64
	width  int
	err    string
}

func NewFormModel(width int) FormModel {
	labels := [pfCount]string{pfName: "Name", pfDesc: "Description", pfPath: "Playbook Path"}
	placeholders := [pfCount]string{
		pfName: "install-nginx",
		pfDesc: "Install and configure nginx",
		pfPath: "~/.local/share/termi/playbooks/install-nginx.yml",
	}
	m := FormModel{width: width}
	for i := 0; i < pfCount; i++ {
		t := textinput.New()
		t.Placeholder = placeholders[i]
		t.Prompt = "  "
		t.CharLimit = 512
		_ = labels[i]
		m.inputs[i] = t
	}
	m.inputs[pfName].Focus()
	return m
}

func (m *FormModel) LoadPlaybook(p store.DBPlaybook) {
	m.editID = p.ID
	m.inputs[pfName].SetValue(p.Name)
	m.inputs[pfDesc].SetValue(p.Description)
	m.inputs[pfPath].SetValue(p.FilePath)
}

func (m *FormModel) SetWidth(w int) { m.width = w }

func (m FormModel) Init() tea.Cmd { return textinput.Blink }

func (m FormModel) Update(msg tea.Msg) (FormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return PlaybookFormCancelledMsg{} }
		case "ctrl+s", "enter":
			if msg.String() == "enter" && m.inputs[pfPath].Focused() {
				// fall through to submit only from last field
			} else if msg.String() == "enter" {
				m.advanceFocus(1)
				return m, m.focusCmd()
			}
			if err := m.validate(); err != nil {
				m.err = err.Error()
				return m, nil
			}
			return m, m.submitCmd()
		case "tab", "down":
			m.advanceFocus(1)
			return m, m.focusCmd()
		case "shift+tab", "up":
			m.advanceFocus(-1)
			return m, m.focusCmd()
		}
	}
	focused := m.focusedIdx()
	var cmd tea.Cmd
	m.inputs[focused], cmd = m.inputs[focused].Update(msg)
	return m, cmd
}

func (m *FormModel) advanceFocus(d int) {
	cur := m.focusedIdx()
	m.inputs[cur].Blur()
	next := (cur + d + pfCount) % pfCount
	m.inputs[next].Focus()
}

func (m FormModel) focusedIdx() int {
	for i := 0; i < pfCount; i++ {
		if m.inputs[i].Focused() {
			return i
		}
	}
	return 0
}

func (m FormModel) focusCmd() tea.Cmd {
	return m.inputs[m.focusedIdx()].Focus()
}

func (m FormModel) validate() error {
	if strings.TrimSpace(m.inputs[pfName].Value()) == "" {
		return fmt.Errorf("Name is required")
	}
	if strings.TrimSpace(m.inputs[pfPath].Value()) == "" {
		return fmt.Errorf("Playbook path is required")
	}
	return nil
}

func (m FormModel) submitCmd() tea.Cmd {
	p := store.DBPlaybook{
		ID:          m.editID,
		Name:        strings.TrimSpace(m.inputs[pfName].Value()),
		Description: strings.TrimSpace(m.inputs[pfDesc].Value()),
		FilePath:    strings.TrimSpace(m.inputs[pfPath].Value()),
	}
	return func() tea.Msg { return SavePlaybookMsg{Playbook: p} }
}

func (m FormModel) View() string {
	title := "  New Playbook"
	if m.editID != 0 {
		title = "  Edit Playbook"
	}

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED")).Render(title) + "\n\n")

	labels := [pfCount]string{pfName: "Name", pfDesc: "Description", pfPath: "Playbook .yml Path"}
	for i := 0; i < pfCount; i++ {
		ls := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Width(22)
		if m.inputs[i].Focused() {
			ls = ls.Foreground(lipgloss.Color("#06B6D4")).Bold(true)
		}
		b.WriteString(fmt.Sprintf("  %s  %s\n", ls.Render(labels[i]), m.inputs[i].View()))
	}

	if m.err != "" {
		b.WriteString("\n  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render("✗ "+m.err) + "\n")
	}

	b.WriteString("\n  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(
		"tab/↓ next  shift+tab/↑ prev  ctrl+s save  esc cancel"))
	return b.String()
}

// ── Run view ──────────────────────────────────────────────────────────────────

// RunView shows live ansible-playbook output.
type RunView struct {
	playbook store.DBPlaybook
	lines    []string
	done     bool
	code     int
	err      error
	scroll   int
	width    int
	height   int
}

func NewRunView(pb store.DBPlaybook, width, height int) RunView {
	return RunView{playbook: pb, width: width, height: height}
}

func (v *RunView) SetSize(w, h int) { v.width = w; v.height = h }

func (v RunView) Update(msg tea.Msg) (RunView, tea.Cmd) {
	switch msg := msg.(type) {
	case PlaybookOutputLineMsg:
		if msg.Text != "" {
			v.lines = append(v.lines, msg.Text)
			v.scroll = 0 // stay at bottom
		}
		if msg.Done {
			v.done = true
			v.code = msg.Code
			v.err = msg.Err
		}
		return v, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			if v.done {
				return v, func() tea.Msg { return PlaybookCancelledMsg{} }
			}
		case "pgup":
			v.scroll += v.visibleLines()
		case "pgdown":
			v.scroll -= v.visibleLines()
			if v.scroll < 0 {
				v.scroll = 0
			}
		}
	}
	return v, nil
}

func (v RunView) visibleLines() int {
	h := v.height - 6
	if h < 1 {
		h = 1
	}
	return h
}

func (v RunView) View() string {
	var b strings.Builder

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED")).
		Render(fmt.Sprintf("  Running: %s", v.playbook.Name))
	b.WriteString(title + "\n\n")

	visible := v.visibleLines()
	start := len(v.lines) - visible - v.scroll
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > len(v.lines) {
		end = len(v.lines)
	}

	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#D1D5DB"))
	for _, l := range v.lines[start:end] {
		b.WriteString("  " + muted.Render(l) + "\n")
	}

	if !v.done {
		b.WriteString("\n  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("▌ running…  pgup/dn scroll"))
	} else {
		var status string
		if v.err != nil {
			status = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render(fmt.Sprintf("  ✗ error: %v", v.err))
		} else if v.code != 0 {
			status = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render(fmt.Sprintf("  ✗ exited %d", v.code))
		} else {
			status = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render("  ✓ completed successfully")
		}
		b.WriteString("\n" + status + "\n")
		b.WriteString("  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("esc back"))
	}
	return b.String()
}
