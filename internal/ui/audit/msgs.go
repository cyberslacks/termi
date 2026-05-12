package audit

import "github.com/cyberslacks/termi/internal/store"

// AuditLoadedMsg is sent when entries are fetched from the DB.
type AuditLoadedMsg struct {
	Entries []store.AuditEntry
	Err     error
}

// AuditCancelledMsg is sent when the user exits the audit log screen.
type AuditCancelledMsg struct{}
