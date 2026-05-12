package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/cyberslacks/termi/internal/creds"
	"github.com/cyberslacks/termi/internal/store"
	gossh "golang.org/x/crypto/ssh"
)

// Session wraps an active SSH connection and PTY.
type Session struct {
	ID         int64 // store.Session.ID
	client     *gossh.Client
	session    *gossh.Session
	stdin      io.WriteCloser
	stdout     io.Reader
	stderr     io.Reader
	cols, rows int
}

// Connect dials the SSH server and opens an interactive PTY shell.
func Connect(ctx context.Context, s store.Session, cred creds.ResolvedCred, cfg ConnectConfig) (*Session, error) {
	authMethods, err := buildAuthMethods(cred)
	if err != nil {
		return nil, err
	}

	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	sshCfg := &gossh.ClientConfig{
		User:            s.User,
		Auth:            authMethods,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), // TODO: known_hosts
		Timeout:         cfg.ConnectTimeout,
	}

	var conn net.Conn
	if s.JumpHostID != nil {
		conn, err = dialViaJump(ctx, s, sshCfg, cfg)
		if err != nil {
			return nil, fmt.Errorf("jump dial: %w", err)
		}
	} else {
		d := net.Dialer{Timeout: cfg.ConnectTimeout}
		conn, err = d.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("dial %s: %w", addr, err)
		}
	}

	c, chans, reqs, err := gossh.NewClientConn(conn, addr, sshCfg)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh handshake: %w", err)
	}
	client := gossh.NewClient(c, chans, reqs)

	sess, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("new session: %w", err)
	}

	cols := cfg.InitCols
	rows := cfg.InitRows
	if cols == 0 {
		cols = 220
	}
	if rows == 0 {
		rows = 50
	}

	modes := gossh.TerminalModes{
		gossh.ECHO:          1,
		gossh.TTY_OP_ISPEED: 14400,
		gossh.TTY_OP_OSPEED: 14400,
	}
	if err := sess.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		sess.Close()
		client.Close()
		return nil, fmt.Errorf("request pty: %w", err)
	}

	stdin, err := sess.StdinPipe()
	if err != nil {
		sess.Close()
		client.Close()
		return nil, err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		sess.Close()
		client.Close()
		return nil, err
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		sess.Close()
		client.Close()
		return nil, err
	}

	if err := sess.Shell(); err != nil {
		sess.Close()
		client.Close()
		return nil, fmt.Errorf("start shell: %w", err)
	}

	return &Session{
		ID:      s.ID,
		client:  client,
		session: sess,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		cols:    cols,
		rows:    rows,
	}, nil
}

// Write sends bytes to the remote shell's stdin.
func (s *Session) Write(data []byte) (int, error) {
	return s.stdin.Write(data)
}

// Stdout returns the reader for PTY output (stdout + stderr merged by PTY).
func (s *Session) Stdout() io.Reader { return s.stdout }

// Stderr returns the stderr reader (usually empty with PTY).
func (s *Session) Stderr() io.Reader { return s.stderr }

// Resize sends a window change request to the remote PTY.
func (s *Session) Resize(cols, rows int) error {
	s.cols = cols
	s.rows = rows
	return s.session.WindowChange(rows, cols)
}

// RunCommand executes a non-interactive command (for automation / playbooks).
// Returns stdout+stderr output and exit code.
func (s *Session) RunCommand(ctx context.Context, cmd string, timeout time.Duration) (string, int, error) {
	sess, err := s.client.NewSession()
	if err != nil {
		return "", -1, err
	}
	defer sess.Close()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	out := make(chan []byte, 1)
	errOut := make(chan error, 1)

	go func() {
		b, err := sess.CombinedOutput(cmd)
		out <- b
		errOut <- err
	}()

	select {
	case <-ctx.Done():
		sess.Signal(gossh.SIGKILL)
		return "", -1, ctx.Err()
	case b := <-out:
		err := <-errOut
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*gossh.ExitError); ok {
				exitCode = exitErr.ExitStatus()
				return string(b), exitCode, nil
			}
			return string(b), -1, err
		}
		return string(b), exitCode, nil
	}
}

// Close disconnects the SSH session and client.
func (s *Session) Close() error {
	s.stdin.Close()
	s.session.Close()
	return s.client.Close()
}

type ConnectConfig struct {
	ConnectTimeout time.Duration
	InitCols       int
	InitRows       int
	// JumpSessions is populated by the manager for bastion lookups
	JumpSessions map[int64]*store.Session
	JumpCreds    map[int64]creds.ResolvedCred
}

func buildAuthMethods(cred creds.ResolvedCred) ([]gossh.AuthMethod, error) {
	switch cred.Method {
	case store.AuthPassword:
		return []gossh.AuthMethod{gossh.Password(cred.Password)}, nil
	case store.AuthKeyFile, store.AuthKeyRing:
		return []gossh.AuthMethod{gossh.PublicKeys(cred.Signer)}, nil
	case store.AuthAgent:
		return []gossh.AuthMethod{gossh.PublicKeysCallback(cred.AgentConn.Signers)}, nil
	default:
		return nil, fmt.Errorf("unknown auth method: %s", cred.Method)
	}
}

func dialViaJump(ctx context.Context, target store.Session, targetCfg *gossh.ClientConfig, cfg ConnectConfig) (net.Conn, error) {
	jumpSess, ok := cfg.JumpSessions[*target.JumpHostID]
	if !ok {
		return nil, fmt.Errorf("jump host session %d not found", *target.JumpHostID)
	}
	jumpCred, ok := cfg.JumpCreds[*target.JumpHostID]
	if !ok {
		return nil, fmt.Errorf("jump host cred %d not found", *target.JumpHostID)
	}

	jumpAuthMethods, err := buildAuthMethods(jumpCred)
	if err != nil {
		return nil, err
	}
	jumpCfg := &gossh.ClientConfig{
		User:            jumpSess.User,
		Auth:            jumpAuthMethods,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         cfg.ConnectTimeout,
	}

	jumpAddr := fmt.Sprintf("%s:%d", jumpSess.Host, jumpSess.Port)
	d := net.Dialer{Timeout: cfg.ConnectTimeout}
	jumpTCPConn, err := d.DialContext(ctx, "tcp", jumpAddr)
	if err != nil {
		return nil, fmt.Errorf("dial jump host %s: %w", jumpAddr, err)
	}

	jumpClientConn, chans, reqs, err := gossh.NewClientConn(jumpTCPConn, jumpAddr, jumpCfg)
	if err != nil {
		jumpTCPConn.Close()
		return nil, fmt.Errorf("ssh handshake to jump host: %w", err)
	}
	jumpClient := gossh.NewClient(jumpClientConn, chans, reqs)

	targetAddr := fmt.Sprintf("%s:%d", target.Host, target.Port)
	conn, err := jumpClient.Dial("tcp", targetAddr)
	if err != nil {
		jumpClient.Close()
		return nil, fmt.Errorf("dial target via jump: %w", err)
	}
	return conn, nil
}

