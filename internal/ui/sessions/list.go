package sessions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jtfrow/termi/internal/store"
)

// sessionItem wraps store.Session to satisfy list.Item.
type sessionItem struct {
	session store.Session
	active  bool // currently connected
}

func (s sessionItem) Title() string {
	prefix := "  "
	if s.active {
		prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render("● ")
	}
	return prefix + s.session.Name
}

func (s sessionItem) Description() string {
	user := s.session.User
	host := s.session.Host
	port := s.session.Port
	auth := string(s.session.AuthMethod)

	var lastConn string
	if s.session.LastConnected != nil {
		lastConn = "  last: " + humanDuration(time.Since(*s.session.LastConnected))
	}

	addr := fmt.Sprintf("%s@%s:%d", user, host, port)
	return fmt.Sprintf("%s  [%s]%s", addr, auth, lastConn)
}

func (s sessionItem) FilterValue() string {
	return s.session.Name + " " + s.session.Host + " " + s.session.User
}

// listKeyMap defines extra keybindings for the session list.
type listKeyMap struct {
	connect key.Binding
	newSess key.Binding
	edit    key.Binding
	delete  key.Binding
	back    key.Binding
}

func newListKeyMap() listKeyMap {
	return listKeyMap{
		connect: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "connect")),
		newSess: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
		edit:    key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		delete:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		back:    key.NewBinding(key.WithKeys("esc", "backspace"), key.WithHelp("esc", "back")),
	}
}

// ListModel is the session browser screen.
type ListModel struct {
	list        list.Model
	keys        listKeyMap
	activeTabs  map[int64]bool // session IDs currently connected
	width       int
	height      int
	loaded      bool
	confirmDel  *store.Session // non-nil = showing delete confirmation
	err         string
}

func NewListModel(width, height int) ListModel {
	keys := newListKeyMap()

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#F9FAFB")).
		Background(lipgloss.Color("#7C3AED")).
		Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#C4B5FD")).
		Background(lipgloss.Color("#7C3AED"))

	l := list.New(nil, delegate, width, height-4)
	l.Title = "Sessions"
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{keys.connect, keys.newSess, keys.edit, keys.delete}
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{keys.connect, keys.newSess, keys.edit, keys.delete, keys.back}
	}

	return ListModel{
		list:       l,
		keys:       keys,
		activeTabs: make(map[int64]bool),
		width:      width,
		height:     height,
	}
}

// LoadCmd returns a tea.Cmd that fetches sessions from the DB.
func LoadCmd(repo *store.SessionRepo) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		sessions, err := repo.List(ctx)
		if err != nil {
			return SessionsLoadedMsg{Err: err}
		}
		groups, err := repo.ListGroups(ctx)
		if err != nil {
			return SessionsLoadedMsg{Err: err}
		}
		return SessionsLoadedMsg{Sessions: sessions, Groups: groups}
	}
}

func (m ListModel) Init() tea.Cmd { return nil }

func (m ListModel) Update(msg tea.Msg) (ListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case SessionsLoadedMsg:
		if msg.Err != nil {
			m.err = msg.Err.Error()
			return m, nil
		}
		items := make([]list.Item, len(msg.Sessions))
		for i, s := range msg.Sessions {
			items[i] = sessionItem{session: s, active: m.activeTabs[s.ID]}
		}
		cmd := m.list.SetItems(items)
		m.loaded = true
		m.err = ""
		return m, cmd

	case tea.KeyMsg:
		// Delete confirmation
		if m.confirmDel != nil {
			switch msg.String() {
			case "y", "Y":
				id := m.confirmDel.ID
				m.confirmDel = nil
				return m, func() tea.Msg { return DeleteSessionMsg{ID: id} }
			default:
				m.confirmDel = nil
				return m, nil
			}
		}

		switch {
		case key.Matches(msg, m.keys.back):
			return m, func() tea.Msg { return FormCancelledMsg{} }

		case key.Matches(msg, m.keys.newSess):
			return m, func() tea.Msg { return OpenFormMsg{Session: nil} }

		case key.Matches(msg, m.keys.edit):
			if sel, ok := m.selectedSession(); ok {
				s := sel // copy
				return m, func() tea.Msg { return OpenFormMsg{Session: &s} }
			}

		case key.Matches(msg, m.keys.delete):
			if sel, ok := m.selectedSession(); ok {
				m.confirmDel = &sel
				return m, nil
			}

		case key.Matches(msg, m.keys.connect):
			if sel, ok := m.selectedSession(); ok {
				return m, func() tea.Msg { return ConnectRequestMsg{Session: sel} }
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m ListModel) View() string {
	if m.err != "" {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render("  Error: " + m.err)
	}
	if !m.loaded {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("  Loading sessions…")
	}

	view := m.list.View()

	if m.confirmDel != nil {
		overlay := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F59E0B")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#F59E0B")).
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

func (m *ListModel) SetActiveTabs(ids map[int64]bool) {
	m.activeTabs = ids
}

func (m ListModel) selectedSession() (store.Session, bool) {
	item, ok := m.list.SelectedItem().(sessionItem)
	if !ok {
		return store.Session{}, false
	}
	return item.session, true
}

func humanDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// groupName builds a display string for group membership.
func groupName(groups []store.Group, id *int64) string {
	if id == nil {
		return ""
	}
	for _, g := range groups {
		if g.ID == *id {
			return g.Name
		}
	}
	return ""
}

// ensure groupName is used (it's referenced by the item delegate via the group list)
var _ = strings.Contains
var _ = groupName
