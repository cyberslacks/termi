package ai

import (
	"fmt"
	"strings"

	"github.com/jtfrow/termi/internal/ssh"
)

// BuildContext assembles context for the AI from active sessions.
func BuildContext(sessions []ssh.ActiveSessionInfo, selectedTabs []int, terminalOutput string) string {
	if len(sessions) == 0 && terminalOutput == "" {
		return ""
	}

	var sb strings.Builder

	if len(sessions) > 0 {
		sb.WriteString("Active SSH sessions:\n")
		selectedSet := make(map[int]bool, len(selectedTabs))
		for _, t := range selectedTabs {
			selectedSet[t] = true
		}
		for _, s := range sessions {
			marker := " "
			if selectedSet[s.TabIndex] {
				marker = "*"
			}
			sb.WriteString(fmt.Sprintf("  [%s] tab=%d name=%q host=%s (connected %s ago)\n",
				marker, s.TabIndex, s.SessionName, s.Host,
				"N/A")) // time since connection could be computed
		}
		if len(selectedTabs) > 0 {
			sb.WriteString("  (* = selected for broadcast/playbook)\n")
		}
	}

	if terminalOutput != "" {
		sb.WriteString("\nRecent terminal output (focused tab):\n")
		sb.WriteString(terminalOutput)
	}

	return sb.String()
}
