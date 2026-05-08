package auth

import (
	"fmt"
	"os"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "atcli"
	keyringUser    = "attio-access-token"

	envToken = "ATTIO_ACCESS_TOKEN"
)

func StoreToken(token string) error {
	if err := keyring.Set(keyringService, keyringUser, token); err != nil {
		return fmt.Errorf("store token in OS credential store: %w", err)
	}
	return nil
}

func LoadToken() (string, error) {
	if token := os.Getenv(envToken); token != "" {
		return token, nil
	}

	token, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		return "", fmt.Errorf("load token from OS credential store: %w", err)
	}
	return token, nil
}
