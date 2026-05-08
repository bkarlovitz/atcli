package cmd

import (
	"errors"
	"fmt"

	"atcli/internal/attio"
	"atcli/internal/auth"
)

var newAttioClient = func(token string) *attio.Client {
	return attio.NewClient(token)
}

func loadAttioClient() (*attio.Client, error) {
	token, err := auth.LoadToken()
	if err != nil {
		if errors.Is(err, auth.ErrNotAuthenticated) {
			return nil, errors.New("not authenticated; run `atcli auth` or set ATTIO_ACCESS_TOKEN")
		}
		if errors.Is(err, auth.ErrCredentialStoreUnavailable) {
			return nil, errors.New("could not read the OS credential store; unlock it, run `atcli auth`, or set ATTIO_ACCESS_TOKEN")
		}
		return nil, fmt.Errorf("%w; run `atcli auth` or set ATTIO_ACCESS_TOKEN", err)
	}
	return newAttioClient(token), nil
}
