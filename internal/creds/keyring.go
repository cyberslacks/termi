package creds

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

const keyringService = "termi"

type keyringBackend struct{}

func (k *keyringBackend) key(sessionID int64) string {
	return fmt.Sprintf("session-%d", sessionID)
}

func (k *keyringBackend) Get(sessionID int64) (string, error) {
	pw, err := keyring.Get(keyringService, k.key(sessionID))
	if err != nil {
		return "", fmt.Errorf("keyring get: %w", err)
	}
	return pw, nil
}

func (k *keyringBackend) Set(sessionID int64, password string) error {
	return keyring.Set(keyringService, k.key(sessionID), password)
}

func (k *keyringBackend) Delete(sessionID int64) error {
	return keyring.Delete(keyringService, k.key(sessionID))
}
