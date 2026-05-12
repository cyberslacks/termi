package creds

import (
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

type keyfileBackend struct{}

func (k *keyfileBackend) Load(path, passphrase string) (ssh.Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key file %s: %w", path, err)
	}

	if passphrase != "" {
		signer, err := ssh.ParsePrivateKeyWithPassphrase(data, []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("parse encrypted key: %w", err)
		}
		return signer, nil
	}

	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("parse key: %w", err)
	}
	return signer, nil
}
