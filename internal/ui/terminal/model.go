package terminal

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cyberslacks/termi/internal/msgs"
	"github.com/cyberslacks/termi/internal/ssh"
	"github.com/cyberslacks/termi/pkg/vtparser"
)

// Model holds the state for one terminal tab.
type Model struct {
	TabIndex  int
	SessionID int64
	Name      string
	Host      string

	screen  *vtparser.Screen
	sshMgr  ssh.Manager
	focused bool
	width   int
	height  int

	// scrollback offset: 0 = live view, positive = scrolled back N lines
	scrollOffset int
}

func New(tabIndex int, sessionID int64, name, host string, sshMgr ssh.Manager, width, height int) Model {
	return Model{
		TabIndex:  tabIndex,
		SessionID: sessionID,
		Name:      name,
		Host:      host,
		screen:    vtparser.NewScreen(width, height),
		sshMgr:    sshMgr,
		focused:   true,
		width:     width,
		height:    height,
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case msgs.TermOutputMsg:
		if msg.TabIndex != m.TabIndex {
			return m, nil
		}
		m.screen.Write(msg.Data) //nolint:errcheck
		return m, nil

	case msgs.TermResizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.screen.Resize(msg.Width, msg.Height)
		_ = m.sshMgr.Resize(m.TabIndex, msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}
		switch msg.Type {
		case tea.KeyPgUp:
			m.scrollOffset += m.height / 2
		case tea.KeyPgDown:
			if m.scrollOffset > 0 {
				m.scrollOffset -= m.height / 2
				if m.scrollOffset < 0 {
					m.scrollOffset = 0
				}
			}
		default:
			m.scrollOffset = 0
			if b := keyToSSHBytes(msg); len(b) > 0 {
				_ = m.sshMgr.SendInput(m.TabIndex, b)
			}
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) SetFocused(focused bool) { m.focused = focused }
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.screen.Resize(w, h)
}

// RecentOutput returns the last n visible lines of the terminal as plain text.
// Used to inject context into the AI panel.
func (m Model) RecentOutput(n int) string {
	return m.screen.RecentText(n)
}
