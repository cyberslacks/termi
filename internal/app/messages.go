// Package app re-exports msgs types and adds app-internal message types
// that depend on higher-level packages (scheduler, ssh, etc.).
package app

import (
	"github.com/cyberslacks/termi/internal/msgs"
	"github.com/cyberslacks/termi/internal/scheduler"
	"github.com/cyberslacks/termi/internal/ssh"
)

// Re-export shared screen IDs so callers only need to import app.
type ScreenID = msgs.ScreenID

const (
	ScreenDashboard  = msgs.ScreenDashboard
	ScreenSessionList = msgs.ScreenSessionList
	ScreenSessionForm = msgs.ScreenSessionForm
	ScreenTerminal   = msgs.ScreenTerminal
	ScreenBroadcast  = msgs.ScreenBroadcast
	ScreenPlaybooks  = msgs.ScreenPlaybooks
	ScreenScheduler  = msgs.ScreenScheduler
	ScreenAuditLog   = msgs.ScreenAuditLog
	ScreenAIPanel    = msgs.ScreenAIPanel
)

// ApprovalRequestMsg wraps a scheduler.ApprovalRequest for the TUI.
type ApprovalRequestMsg struct {
	Req scheduler.ApprovalRequest
}

// BroadcastResultsMsg carries typed broadcast results.
type BroadcastResultsMsg struct {
	Results []ssh.BroadcastResult
}
