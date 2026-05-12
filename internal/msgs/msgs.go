// Package msgs contains all Bubbletea tea.Msg types used across the app.
// It has no imports from other internal packages, breaking import cycles.
package msgs

import (
	"time"
)

// ScreenID identifies TUI screens.
type ScreenID int

const (
	ScreenDashboard ScreenID = iota
	ScreenSessionList
	ScreenSessionForm
	ScreenTerminal
	ScreenBroadcast
	ScreenPlaybooks
	ScreenScheduler
	ScreenAuditLog
	ScreenAIPanel
)

// --- Navigation ---

type NavigateMsg struct{ Screen ScreenID }

// --- SSH lifecycle ---

type ConnectRequestMsg struct {
	SessionID int64
}

type SessionConnectedMsg struct {
	TabIndex    int
	SessionID   int64
	SessionName string
	Host        string
}

type SessionDisconnectedMsg struct {
	TabIndex  int
	SessionID int64
	Err       error
}

type SessionErrorMsg struct {
	TabIndex int
	Err      error
}

// --- PTY I/O (hot path) ---

type TermOutputMsg struct {
	TabIndex int
	Data     []byte
}

type TermResizeMsg struct {
	Width, Height int
}

// --- Broadcast ---

type BroadcastCompleteMsg struct {
	TabIndexes []int
	Command    string
	// Results is []BroadcastResult but we avoid importing ssh here;
	// callers cast via interface.
	Results interface{}
}

// --- Playbook ---

type PlaybookRunMsg struct {
	PlaybookID int64
	TabIndexes []int
	Variables  map[string]string
}

type PlaybookStepDoneMsg struct {
	StepIndex int
	StepName  string
	TabIndex  int
	ExitCode  int
	Output    string
	Err       error
}

type PlaybookCompleteMsg struct {
	PlaybookID int64
	Success    bool
}

// --- Approval ---

type ApprovalRequestMsg struct {
	JobID       int64
	JobName     string
	PlaybookID  int64
	PlanSummary string
	ExpiresAt   time.Time
	// ResponseCh is chan<- bool but typed as interface{} to avoid importing scheduler here.
	ResponseCh interface{}
}

type ApprovalResponseMsg struct {
	JobID    int64
	Approved bool
}

// --- AI ---

type AIStreamChunkMsg struct {
	Chunk string
}

type AIResponseDoneMsg struct {
	FullText       string
	PlaybookYAML   string // non-empty if AI generated a playbook
}

// --- Scheduler ---

type SchedulerJobFiredMsg struct {
	JobID   int64
	JobName string
	FiredAt time.Time
}
