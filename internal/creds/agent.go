package creds

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/crypto/ssh/agent"
)

type agentBackend struct{}

func (a *agentBackend) Connect() (agent.ExtendedAgent, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK not set — is ssh-agent running?")
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("dial ssh-agent: %w", err)
	}
	return agent.NewClient(conn), nil
}
