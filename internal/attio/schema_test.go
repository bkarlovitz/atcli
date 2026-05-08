package attio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListListsDecodesParentObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lists" {
			t.Fatalf("expected /lists, got %s", r.URL.String())
		}
		_, _ = w.Write([]byte(`{"data":[{"id":{"list_id":"list-123"},"api_slug":"hiring-engineering","name":"Hiring Engineering","parent_object":["people"]}]}`))
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	lists, err := client.ListLists(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(lists) != 1 {
		t.Fatalf("expected one list, got %d", len(lists))
	}
	if lists[0].ID.ListID != "list-123" || lists[0].APISlug != "hiring-engineering" || lists[0].ParentObject[0] != "people" {
		t.Fatalf("unexpected list decode: %#v", lists[0])
	}
}

func TestListObjectAttributesIncludesArchivedQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/objects/people/attributes" {
			t.Fatalf("expected object attributes path, got %s", r.URL.String())
		}
		if got := r.URL.Query().Get("show_archived"); got != "true" {
			t.Fatalf("expected show_archived=true, got %q", got)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":{"attribute_id":"attr-123"},"title":"Email","api_slug":"email_addresses","type":"email-address","is_writable":true,"is_required":false,"is_unique":true,"is_multiselect":true,"is_archived":true}]}`))
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	attributes, err := client.ListObjectAttributes(context.Background(), "people", true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(attributes) != 1 {
		t.Fatalf("expected one attribute, got %d", len(attributes))
	}
	if attributes[0].ID.AttributeID != "attr-123" || !attributes[0].IsWritable || !attributes[0].IsArchived {
		t.Fatalf("unexpected attribute decode: %#v", attributes[0])
	}
}

func TestListListAttributesHandlesMissingOptionalFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lists/pipeline/attributes" {
			t.Fatalf("expected list attributes path, got %s", r.URL.String())
		}
		_, _ = w.Write([]byte(`{"data":[{"id":{"attribute_id":"attr-123"},"api_slug":"stage"}]}`))
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	attributes, err := client.ListListAttributes(context.Background(), "pipeline", false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(attributes) != 1 {
		t.Fatalf("expected one attribute, got %d", len(attributes))
	}
	if attributes[0].APISlug != "stage" || attributes[0].Title != "" || attributes[0].Type != "" || attributes[0].IsEditable != nil {
		t.Fatalf("unexpected optional field handling: %#v", attributes[0])
	}
}
