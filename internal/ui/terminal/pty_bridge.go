package terminal

import (
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cyberslacks/termi/internal/msgs"
)

// StartBridge launches a goroutine that reads PTY output and sends TermOutputMsg
// to the Bubbletea program. It exits when the reader closes, sending SessionDisconnectedMsg.
func StartBridge(tabIndex int, sessionID int64, stdout io.Reader, p *tea.Program) {
	go runBridge(tabIndex, sessionID, stdout, p)
}

func runBridge(tabIndex int, sessionID int64, stdout io.Reader, p *tea.Program) {
	buf := make([]byte, 4096)
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			p.Send(msgs.TermOutputMsg{TabIndex: tabIndex, Data: data})
		}
		if err != nil {
			p.Send(msgs.SessionDisconnectedMsg{
				TabIndex:  tabIndex,
				SessionID: sessionID,
				Err:       filterEOF(err),
			})
			return
		}
	}
}

func filterEOF(err error) error {
	if err == io.EOF {
		return nil
	}
	return err
}
