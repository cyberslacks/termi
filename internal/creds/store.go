package creds

import (
	"context"
	"fmt"

	"github.com/cyberslacks/termi/internal/store"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type ResolvedCred struct {
	Method     store.AuthMethod
	Password   string
	Signer     ssh.Signer
	AgentConn  agent.ExtendedAgent
}

type Store interface {
	Resolve(ctx context.Context, s store.Session) (ResolvedCred, error)
	StorePassword(sessionID int64, password string) error
	DeletePassword(sessionID int64) error
}

type credStore struct {
	keyring *keyringBackend
	keyfile *keyfileBackend
	agentB  *agentBackend
}

func New() Store {
	return &credStore{
		keyring: &keyringBackend{},
		keyfile: &keyfileBackend{},
		agentB:  &agentBackend{},
	}
}

func (c *credStore) Resolve(ctx context.Context, s store.Session) (ResolvedCred, error) {
	switch s.AuthMethod {
	case store.AuthPassword:
		pw, err := c.keyring.Get(s.ID)
		if err != nil {
			return ResolvedCred{}, fmt.Errorf("get password: %w", err)
		}
		return ResolvedCred{Method: store.AuthPassword, Password: pw}, nil

	case store.AuthKeyRing:
		pw, err := c.keyring.Get(s.ID)
		if err != nil {
			return ResolvedCred{}, fmt.Errorf("get keyring passphrase: %w", err)
		}
		signer, err := c.keyfile.Load(s.CredentialID, pw)
		if err != nil {
			return ResolvedCred{}, fmt.Errorf("load key: %w", err)
		}
		return ResolvedCred{Method: store.AuthKeyRing, Signer: signer}, nil

	case store.AuthKeyFile:
		signer, err := c.keyfile.Load(s.CredentialID, "")
		if err != nil {
			return ResolvedCred{}, fmt.Errorf("load key file: %w", err)
		}
		return ResolvedCred{Method: store.AuthKeyFile, Signer: signer}, nil

	case store.AuthAgent:
		ag, err := c.agentB.Connect()
		if err != nil {
			return ResolvedCred{}, fmt.Errorf("connect ssh-agent: %w", err)
		}
		return ResolvedCred{Method: store.AuthAgent, AgentConn: ag}, nil

	default:
		return ResolvedCred{}, fmt.Errorf("unknown auth method: %s", s.AuthMethod)
	}
}

func (c *credStore) StorePassword(sessionID int64, password string) error {
	return c.keyring.Set(sessionID, password)
}

func (c *credStore) DeletePassword(sessionID int64) error {
	return c.keyring.Delete(sessionID)
}
