package broadcast

import "github.com/jtfrow/termi/internal/ssh"

// RunBroadcastMsg is sent by the model when the user confirms a broadcast.
// app.go catches it, executes the command, then sends back BroadcastDoneMsg.
type RunBroadcastMsg struct {
	TabIndexes []int
	Command    string
}

// BroadcastDoneMsg is routed back to the model after app.go completes execution.
type BroadcastDoneMsg struct {
	Command string
	Results []ssh.BroadcastResult
}

// BroadcastCancelledMsg is sent when the user presses esc to leave the screen.
type BroadcastCancelledMsg struct{}
