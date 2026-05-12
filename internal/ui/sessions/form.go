package sessions

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
	fName = iota
	fHost
	fPort
	fUser
	fCredential // key file path or blank
	fPassword   // masked; only shown for password/key_ring
	fNotes
	fCount
)

var authMethods = []store.AuthMethod{
	store.AuthAgent,
	store.AuthPassword,
	store.AuthKeyFile,
	store.AuthKeyRing,
}

var authLabels = map[store.AuthMethod]string{
	store.AuthAgent:    "SSH Agent   (uses ssh-agent socket)",
	store.AuthPassword: "Password    (saved to OS keyring)",
	store.AuthKeyFile:  "Key File    (path to private key)",
	store.AuthKeyRing:  "Key+Keyring (key file + passphrase in keyring)",
}

// FormModel handles new session creation and editing.
type FormModel struct {
	inputs     [fCount]textinput.Model
	authIdx    int // index into authMethods
	focused    int // which field has focus (0-fCount, plus one virtual auth field)
	editID     int64
	width      int
	err        string
	submitted  bool
}

const authFieldIdx = fCount // virtual field index for auth method selector

func NewFormModel(width int) FormModel {
	m := FormModel{width: width}

	labels := [fCount]string{
		fName:       "Name",
		fHost:       "Host",
		fPort:       "Port",
		fUser:       "User",
		fCredential: "Key File Path",
		fPassword:   "Password / Passphrase",
		fNotes:      "Notes",
	}
	placeholders := [fCount]string{
		fName:       "prod-web-01",
		fHost:       "192.168.1.100",
		fPort:       "22",
		fUser:       "root",
		fCredential: "~/.ssh/id_ed25519",
		fPassword:   "",
		fNotes:      "optional notes",
	}

	for i := 0; i < fCount; i++ {
		t := textinput.New()
		t.Placeholder = placeholders[i]
		t.Prompt = "  "
		t.CharLimit = 256
		if i == fPassword {
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '•'
		}
		_ = labels[i] // used in View
		m.inputs[i] = t
	}
	m.inputs[fPort].SetValue("22")
	m.inputs[fName].Focus()
	return m
}

// LoadSession pre-fills the form from an existing session.
func (m *FormModel) LoadSession(s store.Session) {
	m.editID = s.ID
	m.inputs[fName].SetValue(s.Name)
	m.inputs[fHost].SetValue(s.Host)
	m.inputs[fPort].SetValue(strconv.Itoa(s.Port))
	m.inputs[fUser].SetValue(s.User)
	m.inputs[fCredential].SetValue(s.CredentialID)
	m.inputs[fNotes].SetValue(s.Notes)
	// Find auth method index
	for i, a := range authMethods {
		if a == s.AuthMethod {
			m.authIdx = i
			break
		}
	}
}

func (m *FormModel) SetWidth(w int) { m.width = w }

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

func (m FormModel) Init() tea.Cmd { return textinput.Blink }

func (m FormModel) Update(msg tea.Msg) (FormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, formKeys.cancel):
			return m, func() tea.Msg { return FormCancelledMsg{} }

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
			if m.focused == authFieldIdx {
				switch msg.String() {
				case "right", "l":
					m.authIdx = (m.authIdx + 1) % len(authMethods)
				case "left", "h":
					m.authIdx = (m.authIdx - 1 + len(authMethods)) % len(authMethods)
				}
				return m, nil
			}
		}
	}

	// Route to the focused input
	if m.focused < fCount {
		var cmd tea.Cmd
		m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m FormModel) View() string {
	var b strings.Builder

	title := "  New Session"
	if m.editID != 0 {
		title = "  Edit Session"
	}
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED")).Render(title))
	b.WriteString("\n\n")

	fieldLabels := [fCount]string{
		fName:       "Name",
		fHost:       "Host",
		fPort:       "Port",
		fUser:       "User",
		fCredential: "Key File Path",
		fPassword:   "Password / Passphrase",
		fNotes:      "Notes",
	}

	for i := 0; i < fCount; i++ {
		// Skip credential/password fields that don't apply to current auth method
		auth := authMethods[m.authIdx]
		if i == fCredential && auth == store.AuthAgent || i == fCredential && auth == store.AuthPassword {
			continue
		}
		if i == fPassword && auth != store.AuthPassword && auth != store.AuthKeyRing {
			continue
		}

		label := fieldLabels[i]
		focused := m.focused == i
		b.WriteString(m.renderField(label, m.inputs[i].View(), focused))

		// Insert auth selector after User field
		if i == fUser {
			b.WriteString(m.renderAuthSelector())
		}
	}

	if m.err != "" {
		b.WriteString("\n  ")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render("✗ " + m.err))
		b.WriteString("\n")
	}

	b.WriteString("\n  ")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(
		"tab/↓ next field  shift+tab/↑ prev  ctrl+s save  esc cancel",
	))

	return b.String()
}

func (m FormModel) renderField(label, inputView string, focused bool) string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Width(22)
	if focused {
		labelStyle = labelStyle.Foreground(lipgloss.Color("#06B6D4")).Bold(true)
	}
	return fmt.Sprintf("  %s  %s\n", labelStyle.Render(label), inputView)
}

func (m FormModel) renderAuthSelector() string {
	focused := m.focused == authFieldIdx
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Width(22)
	if focused {
		labelStyle = labelStyle.Foreground(lipgloss.Color("#06B6D4")).Bold(true)
	}

	auth := authMethods[m.authIdx]
	display := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F9FAFB")).
		Background(lipgloss.Color("#374151")).
		Padding(0, 1).
		Render("◀ " + authLabels[auth] + " ▶")

	hint := ""
	if focused {
		hint = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("  ← → to change")
	}

	return fmt.Sprintf("  %s  %s%s\n", labelStyle.Render("Auth Method"), display, hint)
}

func (m *FormModel) advanceFocus(delta int) {
	totalFields := fCount + 1 // +1 for the auth selector

	// Blurs current field if it's a real input
	if m.focused < fCount {
		m.inputs[m.focused].Blur()
	}

	// Compute next valid field, skipping hidden ones
	for {
		m.focused = (m.focused + delta + totalFields) % totalFields
		if m.focused < fCount && m.isFieldVisible(m.focused) {
			break
		}
		if m.focused == authFieldIdx {
			break
		}
	}

	if m.focused < fCount {
		m.inputs[m.focused].Focus()
	}
}

func (m FormModel) isFieldVisible(i int) bool {
	auth := authMethods[m.authIdx]
	switch i {
	case fCredential:
		return auth == store.AuthKeyFile || auth == store.AuthKeyRing
	case fPassword:
		return auth == store.AuthPassword || auth == store.AuthKeyRing
	}
	return true
}

func (m FormModel) focusCmd() tea.Cmd {
	if m.focused < fCount {
		return m.inputs[m.focused].Focus()
	}
	return nil
}

func (m FormModel) validate() error {
	if strings.TrimSpace(m.inputs[fName].Value()) == "" {
		return fmt.Errorf("Name is required")
	}
	if strings.TrimSpace(m.inputs[fHost].Value()) == "" {
		return fmt.Errorf("Host is required")
	}
	if strings.TrimSpace(m.inputs[fUser].Value()) == "" {
		return fmt.Errorf("User is required")
	}
	port, err := strconv.Atoi(m.inputs[fPort].Value())
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("Port must be 1–65535")
	}
	auth := authMethods[m.authIdx]
	if (auth == store.AuthKeyFile || auth == store.AuthKeyRing) &&
		strings.TrimSpace(m.inputs[fCredential].Value()) == "" {
		return fmt.Errorf("Key file path is required for %s", auth)
	}
	return nil
}

func (m FormModel) submitCmd() tea.Cmd {
	port, _ := strconv.Atoi(m.inputs[fPort].Value())
	if port == 0 {
		port = 22
	}
	auth := authMethods[m.authIdx]

	s := store.Session{
		ID:           m.editID,
		Name:         strings.TrimSpace(m.inputs[fName].Value()),
		Host:         strings.TrimSpace(m.inputs[fHost].Value()),
		Port:         port,
		User:         strings.TrimSpace(m.inputs[fUser].Value()),
		AuthMethod:   auth,
		CredentialID: strings.TrimSpace(m.inputs[fCredential].Value()),
		Notes:        strings.TrimSpace(m.inputs[fNotes].Value()),
	}

	pw := m.inputs[fPassword].Value()

	return func() tea.Msg {
		return SaveSessionMsg{Session: s, Password: pw}
	}
}
