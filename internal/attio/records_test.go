package attio

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateRecordSendsRequestAndDecodesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/objects/people/records" {
			t.Fatalf("expected create record path, got %s", r.URL.String())
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("expected bearer token header, got %q", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("expected JSON accept header, got %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("expected JSON content type header, got %q", got)
		}

		var payload struct {
			Data struct {
				Values map[string]any `json:"values"`
			} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.Data.Values["name"] != "Ada Lovelace" {
			t.Fatalf("expected string value, got %#v", payload.Data.Values["name"])
		}
		if payload.Data.Values["follower_count"] != float64(42) {
			t.Fatalf("expected numeric value, got %#v", payload.Data.Values["follower_count"])
		}

		_, _ = w.Write([]byte(`{"data":{"id":{"workspace_id":"workspace-123","object_id":"object-123","record_id":"record-123"},"created_at":"2026-05-07T12:00:00Z","web_url":"https://app.attio.com/acme/person/record-123","values":{"name":"Ada Lovelace"}}}`))
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	record, err := client.CreateRecord(context.Background(), "people", map[string]any{
		"name":           "Ada Lovelace",
		"follower_count": 42,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if record.ID.WorkspaceID != "workspace-123" || record.ID.ObjectID != "object-123" || record.ID.RecordID != "record-123" {
		t.Fatalf("unexpected record ID: %#v", record.ID)
	}
	if record.WebURL == "" || record.Values["name"] != "Ada Lovelace" {
		t.Fatalf("unexpected record decode: %#v", record)
	}
}

func TestCreateRecordEscapesObjectIdentifier(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/objects/custom%20object/records" {
			t.Fatalf("expected escaped path, got %s", r.URL.EscapedPath())
		}
		_, _ = w.Write([]byte(`{"data":{"id":{"record_id":"record-123"}}}`))
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if _, err := client.CreateRecord(context.Background(), "custom object", map[string]any{"name": "Acme"}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestCreateRecordReturnsUsefulAPIErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantBody   string
	}{
		{
			name:       "validation",
			statusCode: http.StatusBadRequest,
			body:       `{"error":"name is required"}`,
			wantBody:   "name is required",
		},
		{
			name:       "permission",
			statusCode: http.StatusForbidden,
			body:       `{"error":"missing record_permission:read-write for test-token"}`,
			wantBody:   "missing record_permission:read-write",
		},
		{
			name:       "rate limit",
			statusCode: http.StatusTooManyRequests,
			body:       `{"error":"rate limit exceeded"}`,
			wantBody:   "rate limit exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, tt.body, tt.statusCode)
			}))
			defer server.Close()

			client := NewClient("test-token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
			_, err := client.CreateRecord(context.Background(), "people", map[string]any{"name": "Ada"})

			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected APIError, got %T: %v", err, err)
			}
			if apiErr.StatusCode != tt.statusCode {
				t.Fatalf("expected status %d, got %d", tt.statusCode, apiErr.StatusCode)
			}
			if !strings.Contains(apiErr.Body, tt.wantBody) {
				t.Fatalf("expected body to contain %q, got %q", tt.wantBody, apiErr.Body)
			}
			if strings.Contains(apiErr.Body, "test-token") || strings.Contains(apiErr.Error(), "test-token") {
				t.Fatalf("expected token to be redacted, got %q", apiErr.Error())
			}
		})
	}
}

func TestCreateRecordPermissionErrorsAreClassified(t *testing.T) {
	for _, statusCode := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"error":"missing scope"}`, statusCode)
		}))

		client := NewClient("test-token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
		_, err := client.CreateRecord(context.Background(), "people", map[string]any{"name": "Ada"})
		server.Close()

		if !IsPermissionError(err) {
			t.Fatalf("expected status %d to be classified as permission error, got %v", statusCode, err)
		}
	}
}
