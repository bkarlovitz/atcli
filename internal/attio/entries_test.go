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

func TestCreateListEntrySendsRequestAndDecodesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.EscapedPath() != "/lists/sales%20pipeline/entries" {
			t.Fatalf("expected escaped create entry path, got %s", r.URL.EscapedPath())
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
				ParentRecordID string         `json:"parent_record_id"`
				ParentObject   string         `json:"parent_object"`
				EntryValues    map[string]any `json:"entry_values"`
			} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.Data.ParentRecordID != "record-123" || payload.Data.ParentObject != "people" {
			t.Fatalf("unexpected parent reference: %#v", payload.Data)
		}
		if payload.Data.EntryValues["status"] != "Qualified" {
			t.Fatalf("unexpected entry values: %#v", payload.Data.EntryValues)
		}

		_, _ = w.Write([]byte(`{"data":{"id":{"workspace_id":"workspace-123","list_id":"list-123","entry_id":"entry-123"},"parent_record_id":"record-123","parent_object":"people","created_at":"2026-05-08T12:00:00Z","entry_values":{"status":[{"value":"Qualified"}]},"status":"created","created":true}}`))
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	result, err := client.CreateListEntry(context.Background(), "sales pipeline", ListEntryWrite{
		ParentRecordID: "record-123",
		ParentObject:   "people",
		EntryValues: map[string]any{
			"status": "Qualified",
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Entry.ID.WorkspaceID != "workspace-123" || result.Entry.ID.ListID != "list-123" || result.Entry.ID.EntryID != "entry-123" {
		t.Fatalf("unexpected entry ID: %#v", result.Entry.ID)
	}
	if result.Entry.ParentRecordID != "record-123" || result.Entry.ParentObject != "people" {
		t.Fatalf("unexpected parent decode: %#v", result.Entry)
	}
	if result.Entry.CreatedAt == "" || result.Entry.EntryValues["status"] == nil {
		t.Fatalf("unexpected entry decode: %#v", result.Entry)
	}
	if result.Outcome != "created" {
		t.Fatalf("expected created outcome, got %q", result.Outcome)
	}
	if result.Created == nil || !*result.Created {
		t.Fatalf("expected created marker, got %#v", result.Created)
	}
}

func TestAssertListEntrySendsRequestAndDecodesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/lists/list-123/entries" {
			t.Fatalf("expected assert entry path, got %s", r.URL.String())
		}

		var payload struct {
			Data struct {
				ParentRecordID string         `json:"parent_record_id"`
				ParentObject   string         `json:"parent_object"`
				EntryValues    map[string]any `json:"entry_values"`
			} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.Data.ParentRecordID != "record-123" || payload.Data.ParentObject != "people" {
			t.Fatalf("unexpected parent reference: %#v", payload.Data)
		}
		if payload.Data.EntryValues["priority"] != float64(3) {
			t.Fatalf("unexpected entry values: %#v", payload.Data.EntryValues)
		}

		_, _ = w.Write([]byte(`{"data":{"id":{"workspace_id":"workspace-123","list_id":"list-123","entry_id":"entry-123"},"parent_record_id":"record-123","parent_object":"people","created_at":"2026-05-08T12:00:00Z","entry_values":{"priority":[{"value":3}]},"operation":"updated","created":false}}`))
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	result, err := client.AssertListEntry(context.Background(), "list-123", ListEntryWrite{
		ParentRecordID: "record-123",
		ParentObject:   "people",
		EntryValues: map[string]any{
			"priority": 3,
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Entry.ID.EntryID != "entry-123" || result.Entry.ID.ListID != "list-123" {
		t.Fatalf("unexpected entry identity: %#v", result.Entry.ID)
	}
	if result.Outcome != "updated" {
		t.Fatalf("expected updated outcome, got %q", result.Outcome)
	}
	if result.Created == nil || *result.Created {
		t.Fatalf("expected created=false marker, got %#v", result.Created)
	}
}

func TestListEntryWritesRequireParentReferences(t *testing.T) {
	client := NewClient("test-token")
	tests := []struct {
		name  string
		list  string
		write ListEntryWrite
		want  string
	}{
		{
			name: "list",
			write: ListEntryWrite{
				ParentRecordID: "record-123",
				ParentObject:   "people",
			},
			want: "list is required",
		},
		{
			name: "parent object",
			list: "list-123",
			write: ListEntryWrite{
				ParentRecordID: "record-123",
			},
			want: "parent object is required",
		},
		{
			name: "parent record",
			list: "list-123",
			write: ListEntryWrite{
				ParentObject: "people",
			},
			want: "parent record ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.CreateListEntry(context.Background(), tt.list, tt.write)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, err)
			}
		})
	}
}

func TestCreateListEntryReturnsUsefulAPIErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantBody   string
	}{
		{
			name:       "permission",
			statusCode: http.StatusForbidden,
			body:       `{"error":"missing list_entry:read-write for test-token"}`,
			wantBody:   "missing list_entry:read-write",
		},
		{
			name:       "validation",
			statusCode: http.StatusBadRequest,
			body:       `{"error":"entry_values.status is required"}`,
			wantBody:   "entry_values.status is required",
		},
		{
			name:       "duplicate",
			statusCode: http.StatusConflict,
			body:       `{"error":"DUPLICATE_VALUE: unique attribute conflict"}`,
			wantBody:   "DUPLICATE_VALUE",
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
			_, err := client.CreateListEntry(context.Background(), "sales", ListEntryWrite{
				ParentRecordID: "record-123",
				ParentObject:   "people",
				EntryValues:    map[string]any{"status": "Qualified"},
			})

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

func TestAssertListEntryReturnsMultipleMatchAndRateLimitStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		retryAfter string
		wantBody   string
		wantRetry  bool
	}{
		{
			name:       "multiple match",
			statusCode: http.StatusBadRequest,
			body:       `{"error":"MULTIPLE_MATCH_RESULTS: multiple list entries match this parent"}`,
			wantBody:   "MULTIPLE_MATCH_RESULTS",
		},
		{
			name:       "rate limit retry after",
			statusCode: http.StatusTooManyRequests,
			body:       `{"error":"rate limit exceeded"}`,
			retryAfter: "3",
			wantBody:   "rate limit exceeded",
			wantRetry:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.retryAfter != "" {
					w.Header().Set("Retry-After", tt.retryAfter)
				}
				http.Error(w, tt.body, tt.statusCode)
			}))
			defer server.Close()

			client := NewClient("test-token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
			_, err := client.AssertListEntry(context.Background(), "sales", ListEntryWrite{
				ParentRecordID: "record-123",
				ParentObject:   "people",
				EntryValues:    map[string]any{"status": "Qualified"},
			})

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
			if apiErr.HasRetryAfter != tt.wantRetry {
				t.Fatalf("expected retry marker %v, got %v", tt.wantRetry, apiErr.HasRetryAfter)
			}
		})
	}
}
