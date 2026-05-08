package attio

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientUsesCustomBaseURLAndHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/objects" {
			t.Fatalf("expected /objects, got %s", r.URL.String())
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("expected bearer token header, got %q", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("expected JSON accept header, got %q", got)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":{"object_id":"object-123"},"api_slug":"people","singular_noun":"Person","plural_noun":"People"}]}`))
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	objects, err := client.ListObjects(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(objects) != 1 {
		t.Fatalf("expected one object, got %d", len(objects))
	}
	if objects[0].APISlug != "people" || objects[0].ID.ObjectID != "object-123" {
		t.Fatalf("unexpected object decode: %#v", objects[0])
	}
}

func TestClientReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"missing scope"}`, http.StatusForbidden)
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	_, err := client.ListObjects(context.Background())

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.Body, "missing scope") {
		t.Fatalf("expected error body to be preserved, got %q", apiErr.Body)
	}
}
