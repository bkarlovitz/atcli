package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const introspectURL = "https://app.attio.com/oauth/introspect"

type Introspection struct {
	Active        bool   `json:"active"`
	Scope         string `json:"scope"`
	TokenType     string `json:"token_type"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	WorkspaceSlug string `json:"workspace_slug"`
}

func Introspect(ctx context.Context, token string) (*Introspection, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, introspectURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create introspection request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send introspection request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if len(body) > 0 {
			return nil, fmt.Errorf("Attio returned %s: %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("Attio returned %s", resp.Status)
	}

	var info Introspection
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode introspection response: %w", err)
	}

	return &info, nil
}
