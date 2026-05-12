package playbook

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// AnsibleTarget holds per-host SSH connection details for inventory generation.
type AnsibleTarget struct {
	Name     string // inventory alias (e.g. session name)
	Host     string
	Port     int
	User     string
	KeyPath  string // path to private key; empty when using agent or password
	Password string // requires sshpass on the local machine
}

// AnsibleLine is one output line from ansible-playbook, or a terminal sentinel.
type AnsibleLine struct {
	Text string
	Done bool
	Err  error
	Code int // exit code, set when Done == true
}

// Executor wraps ansible-playbook.
type Executor struct {
	bin string
}

func NewExecutor() *Executor {
	bin, err := exec.LookPath("ansible-playbook")
	if err != nil {
		bin = "ansible-playbook" // report error at run time
	}
	return &Executor{bin: bin}
}

// Run executes a playbook and streams combined stdout+stderr line by line.
// The returned channel is closed after the Done sentinel is sent.
func (e *Executor) Run(ctx context.Context, playbookPath string, targets []AnsibleTarget, vars map[string]string) (<-chan AnsibleLine, error) {
	if len(targets) == 0 {
		return nil, fmt.Errorf("no targets provided")
	}

	invPath, err := writeInventory(targets)
	if err != nil {
		return nil, fmt.Errorf("write inventory: %w", err)
	}

	args := []string{"-i", invPath, playbookPath}
	if len(vars) > 0 {
		b, _ := json.Marshal(vars)
		args = append(args, "--extra-vars", string(b))
	}

	pr, pw := io.Pipe()
	cmd := exec.CommandContext(ctx, e.bin, args...)
	cmd.Stdout = pw
	cmd.Stderr = pw

	ch := make(chan AnsibleLine, 256)

	go func() {
		defer os.Remove(invPath)
		defer close(ch)
		defer pr.Close()

		if err := cmd.Start(); err != nil {
			pw.Close()
			ch <- AnsibleLine{Err: fmt.Errorf("start ansible-playbook: %w", err), Done: true}
			return
		}

		// Read lines concurrently while the process runs
		done := make(chan struct{})
		go func() {
			defer close(done)
			scanner := bufio.NewScanner(pr)
			for scanner.Scan() {
				ch <- AnsibleLine{Text: scanner.Text()}
			}
		}()

		waitErr := cmd.Wait()
		pw.Close() // signal EOF to scanner goroutine
		<-done     // wait for all lines to be sent

		code := 0
		if waitErr != nil {
			if exitErr, ok := waitErr.(*exec.ExitError); ok {
				code = exitErr.ExitCode()
			} else {
				ch <- AnsibleLine{Err: waitErr, Done: true}
				return
			}
		}
		ch <- AnsibleLine{Done: true, Code: code}
	}()

	return ch, nil
}

// writeInventory creates a temp .ini inventory file and returns its path.
func writeInventory(targets []AnsibleTarget) (string, error) {
	f, err := os.CreateTemp("", "termi-inventory-*.ini")
	if err != nil {
		return "", err
	}
	defer f.Close()

	fmt.Fprintln(f, "[all]")
	for _, t := range targets {
		name := strings.ReplaceAll(t.Name, " ", "_")
		if name == "" {
			name = t.Host
		}
		parts := []string{
			name,
			fmt.Sprintf("ansible_host=%s", t.Host),
			fmt.Sprintf("ansible_port=%d", t.Port),
			fmt.Sprintf("ansible_user=%s", t.User),
		}
		if t.KeyPath != "" {
			parts = append(parts, fmt.Sprintf("ansible_ssh_private_key_file=%s", t.KeyPath))
		} else if t.Password != "" {
			parts = append(parts, fmt.Sprintf("ansible_password=%s", t.Password))
		}
		// Suppress interactive host-key prompts
		parts = append(parts, `ansible_ssh_common_args='-o StrictHostKeyChecking=accept-new'`)
		fmt.Fprintln(f, strings.Join(parts, " "))
	}
	return f.Name(), nil
}
