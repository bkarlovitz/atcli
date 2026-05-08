package auth

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestLoadTokenReturnsNotAuthenticatedWhenTokenMissing(t *testing.T) {
	t.Setenv(envToken, "")
	keyring.MockInit()

	_, err := LoadToken()
	if !errors.Is(err, ErrNotAuthenticated) {
		t.Fatalf("expected ErrNotAuthenticated, got %v", err)
	}
}

func TestLoadTokenReturnsEnvTokenFirst(t *testing.T) {
	t.Setenv(envToken, "env-token")
	keyring.MockInitWithError(errors.New("keyring should not be read"))

	token, err := LoadToken()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token != "env-token" {
		t.Fatalf("expected env token, got %q", token)
	}
}

func TestLoadTokenWrapsCredentialStoreFailures(t *testing.T) {
	t.Setenv(envToken, "")
	keyring.MockInitWithError(errors.New("locked"))

	_, err := LoadToken()
	if !errors.Is(err, ErrCredentialStoreUnavailable) {
		t.Fatalf("expected ErrCredentialStoreUnavailable, got %v", err)
	}
}
