package sessions

import "github.com/cyberslacks/termi/internal/store"

// ConnectRequestMsg is sent when the user selects a session to connect to.
type ConnectRequestMsg struct{ Session store.Session }

// OpenFormMsg is sent when the user wants to create or edit a session.
// Session is nil for a new session, non-nil for editing an existing one.
type OpenFormMsg struct{ Session *store.Session }

// SaveSessionMsg is sent when the form is submitted.
type SaveSessionMsg struct {
	Session  store.Session
	Password string // non-empty when auth=password or auth=key_ring
}

// DeleteSessionMsg is sent when the user confirms deleting a session.
type DeleteSessionMsg struct{ ID int64 }

// FormCancelledMsg is sent when the form is dismissed without saving.
type FormCancelledMsg struct{}

// SessionsLoadedMsg carries freshly loaded sessions and groups from the DB.
type SessionsLoadedMsg struct {
	Sessions []store.Session
	Groups   []store.Group
	Err      error
}
