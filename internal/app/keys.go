package app

import "github.com/charmbracelet/bubbles/key"

// GlobalKeyMap defines app-wide keybindings.
// The prefix key (Ctrl+B by default) is consumed by the root model;
// the next key is routed as a termi command rather than forwarded to SSH.
type GlobalKeyMap struct {
	Quit        key.Binding
	Help        key.Binding
	SessionList key.Binding
	NewSession  key.Binding
	Broadcast   key.Binding
	Playbooks   key.Binding
	Scheduler   key.Binding
	AuditLog    key.Binding
	AIPanel     key.Binding
	NextTab     key.Binding
	PrevTab     key.Binding
	CloseTab    key.Binding
	Tab1        key.Binding
	Tab2        key.Binding
	Tab3        key.Binding
	Tab4        key.Binding
	Tab5        key.Binding
}

var Keys = GlobalKeyMap{
	Quit:        key.NewBinding(key.WithKeys("ctrl+c", "q"), key.WithHelp("q", "quit")),
	Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	SessionList: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sessions")),
	NewSession:  key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new session")),
	Broadcast:   key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "broadcast")),
	Playbooks:   key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "playbooks")),
	Scheduler:   key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "scheduler")),
	AuditLog:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "audit log")),
	AIPanel:     key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "AI panel")),
	NextTab:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next tab")),
	PrevTab:     key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev tab")),
	CloseTab:    key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "close tab")),
	Tab1:        key.NewBinding(key.WithKeys("1")),
	Tab2:        key.NewBinding(key.WithKeys("2")),
	Tab3:        key.NewBinding(key.WithKeys("3")),
	Tab4:        key.NewBinding(key.WithKeys("4")),
	Tab5:        key.NewBinding(key.WithKeys("5")),
}
