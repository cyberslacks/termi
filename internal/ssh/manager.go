package ssh

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/jtfrow/termi/internal/creds"
	"github.com/jtfrow/termi/internal/store"
)

// ActiveSessionInfo is a snapshot of an open SSH session.
type ActiveSessionInfo struct {
	TabIndex    int
	SessionID   int64
	SessionName string
	Host        string
	ConnectedAt time.Time
}

// Manager handles the lifecycle of all open SSH sessions.
type Manager interface {
	Connect(ctx context.Context, s store.Session, cred creds.ResolvedCred, cfg ConnectConfig) (tabIndex int, err error)
	Disconnect(tabIndex int) error
	SendInput(tabIndex int, data []byte) error
	Resize(tabIndex int, cols, rows int) error
	BroadcastCommand(tabIndexes []int, command string) []BroadcastResult
	RunCommand(ctx context.Context, tabIndex int, cmd string, timeout time.Duration) (string, int, error)
	ActiveSessions() []ActiveSessionInfo
	// SessionStdout returns the PTY stdout reader for a tab (used to start the PTY bridge).
	SessionStdout(tabIndex int) (io.Reader, error)
}

// BroadcastResult holds the outcome of a single session in a broadcast.
type BroadcastResult struct {
	TabIndex    int
	SessionID   int64
	SessionName string
	Output      string
	ExitCode    int
	Err         error
}

type entry struct {
	sess        *Session
	sessionID   int64
	sessionName string
	host        string
	connectedAt time.Time
}

type manager struct {
	mu      sync.RWMutex
	tabs    map[int]*entry
	nextTab int
}

func NewManager() Manager {
	return &manager{tabs: make(map[int]*entry)}
}

func (m *manager) Connect(ctx context.Context, s store.Session, cred creds.ResolvedCred, cfg ConnectConfig) (int, error) {
	sess, err := Connect(ctx, s, cred, cfg)
	if err != nil {
		return -1, err
	}

	m.mu.Lock()
	tabIndex := m.nextTab
	m.nextTab++
	m.tabs[tabIndex] = &entry{
		sess:        sess,
		sessionID:   s.ID,
		sessionName: s.Name,
		host:        s.Host,
		connectedAt: time.Now(),
	}
	m.mu.Unlock()
	return tabIndex, nil
}

func (m *manager) Disconnect(tabIndex int) error {
	m.mu.Lock()
	e, ok := m.tabs[tabIndex]
	if ok {
		delete(m.tabs, tabIndex)
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("tab %d not found", tabIndex)
	}
	return e.sess.Close()
}

func (m *manager) SendInput(tabIndex int, data []byte) error {
	m.mu.RLock()
	e, ok := m.tabs[tabIndex]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("tab %d not found", tabIndex)
	}
	_, err := e.sess.Write(data)
	return err
}

func (m *manager) Resize(tabIndex int, cols, rows int) error {
	m.mu.RLock()
	e, ok := m.tabs[tabIndex]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("tab %d not found", tabIndex)
	}
	return e.sess.Resize(cols, rows)
}

func (m *manager) BroadcastCommand(tabIndexes []int, command string) []BroadcastResult {
	type result struct {
		idx int
		BroadcastResult
	}
	ch := make(chan result, len(tabIndexes))

	m.mu.RLock()
	for _, idx := range tabIndexes {
		e, ok := m.tabs[idx]
		if !ok {
			ch <- result{idx: idx, BroadcastResult: BroadcastResult{TabIndex: idx, Err: fmt.Errorf("tab %d not found", idx)}}
			continue
		}
		// Copy pointer for goroutine
		tabIdx := idx
		ent := e
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			out, code, err := ent.sess.RunCommand(ctx, command, 5*time.Minute)
			ch <- result{idx: tabIdx, BroadcastResult: BroadcastResult{
				TabIndex:    tabIdx,
				SessionID:   ent.sessionID,
				SessionName: ent.sessionName,
				Output:      out,
				ExitCode:    code,
				Err:         err,
			}}
		}()
	}
	m.mu.RUnlock()

	var results []BroadcastResult
	for range tabIndexes {
		r := <-ch
		results = append(results, r.BroadcastResult)
	}
	return results
}

func (m *manager) RunCommand(ctx context.Context, tabIndex int, cmd string, timeout time.Duration) (string, int, error) {
	m.mu.RLock()
	e, ok := m.tabs[tabIndex]
	m.mu.RUnlock()
	if !ok {
		return "", -1, fmt.Errorf("tab %d not found", tabIndex)
	}
	return e.sess.RunCommand(ctx, cmd, timeout)
}

func (m *manager) ActiveSessions() []ActiveSessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ActiveSessionInfo, 0, len(m.tabs))
	for idx, e := range m.tabs {
		out = append(out, ActiveSessionInfo{
			TabIndex:    idx,
			SessionID:   e.sessionID,
			SessionName: e.sessionName,
			Host:        e.host,
			ConnectedAt: e.connectedAt,
		})
	}
	return out
}

func (m *manager) SessionStdout(tabIndex int) (io.Reader, error) {
	m.mu.RLock()
	e, ok := m.tabs[tabIndex]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("tab %d not found", tabIndex)
	}
	return e.sess.Stdout(), nil
}
