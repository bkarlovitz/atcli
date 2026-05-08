package auth

import (
	"errors"
	"fmt"
	"os"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "atcli"
	keyringUser    = "attio-access-token"

	envToken = "ATTIO_ACCESS_TOKEN"
)

var ErrNotAuthenticated = errors.New("not authenticated")
var ErrCredentialStoreUnavailable = errors.New("credential store unavailable")

func StoreToken(token string) error {
	if err := keyring.Set(keyringService, keyringUser, token); err != nil {
		return fmt.Errorf("%w: %v", ErrCredentialStoreUnavailable, err)
	}
	return nil
}

func LoadToken() (string, error) {
	if token := os.Getenv(envToken); token != "" {
		return token, nil
	}

	token, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrNotAuthenticated
		}
		return "", fmt.Errorf("%w: %v", ErrCredentialStoreUnavailable, err)
	}
	return token, nil
}
