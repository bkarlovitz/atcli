package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"atcli/internal/attio"

	"github.com/spf13/cobra"
)

func TestObjectsListPrintsReturnedNouns(t *testing.T) {
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/objects" {
			t.Fatalf("expected /objects, got %s", r.URL.String())
		}
		_, _ = w.Write([]byte(`{"data":[{"id":{"object_id":"object-people"},"api_slug":"people","singular_noun":"Person","plural_noun":"People"},{"id":{"object_id":"object-widgets"},"api_slug":"custom_widgets"}]}`))
	}))

	output, err := executeTestCommand(t, newObjectsCommand(), "list")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertContains(t, output, "API SLUG")
	assertContains(t, output, "people")
	assertContains(t, output, "object-people")
	assertContains(t, output, "Person")
	assertContains(t, output, "People")
	assertContains(t, output, "custom_widgets")
}

func TestObjectsAttributesFiltersArchivedByDefault(t *testing.T) {
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/objects/people/attributes" {
			t.Fatalf("expected object attributes path, got %s", r.URL.String())
		}
		if got := r.URL.Query().Get("show_archived"); got != "" {
			t.Fatalf("did not expect show_archived query, got %q", got)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":{"attribute_id":"attr-email"},"title":"Email","api_slug":"email_addresses","type":"email-address","is_writable":true,"is_unique":true,"is_multiselect":true},{"id":{"attribute_id":"attr-old"},"title":"Old","api_slug":"old_field","type":"text","is_archived":true}]}`))
	}))

	output, err := executeTestCommand(t, newObjectsCommand(), "attributes", "people")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertContains(t, output, "email_addresses")
	assertContains(t, output, "yes")
	assertNotContains(t, output, "old_field")
}

func TestObjectsAttributesAllIncludesArchived(t *testing.T) {
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/objects/people/attributes" {
			t.Fatalf("expected object attributes path, got %s", r.URL.String())
		}
		if got := r.URL.Query().Get("show_archived"); got != "true" {
			t.Fatalf("expected show_archived=true, got %q", got)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":{"attribute_id":"attr-old"},"title":"Old","api_slug":"old_field","type":"text","is_archived":true}]}`))
	}))

	output, err := executeTestCommand(t, newObjectsCommand(), "attributes", "people", "--all")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertContains(t, output, "old_field")
	assertContains(t, output, "yes")
}

func TestObjectsAttributesHelpExplainsObjectArgument(t *testing.T) {
	output, err := executeTestCommand(t, newObjectsCommand(), "attributes", "--help")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertContains(t, output, "slug or ID")
	assertContains(t, output, "people")
	assertContains(t, output, "companies")
}

func TestListsListPrintsParentObject(t *testing.T) {
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lists" {
			t.Fatalf("expected /lists, got %s", r.URL.String())
		}
		_, _ = w.Write([]byte(`{"data":[{"id":{"list_id":"list-123"},"api_slug":"hiring-engineering","name":"Hiring Engineering","parent_object":["people"]},{"id":{"list_id":"list-456"},"api_slug":"empty-list","name":"No Parent"}]}`))
	}))

	output, err := executeTestCommand(t, newListsCommand(), "list")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertContains(t, output, "hiring-engineering")
	assertContains(t, output, "Hiring Engineering")
	assertContains(t, output, "people")
	assertContains(t, output, "empty-list")
}

func TestListsAttributesPrintsListEntryAttributes(t *testing.T) {
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lists/pipeline/attributes" {
			t.Fatalf("expected list attributes path, got %s", r.URL.String())
		}
		_, _ = w.Write([]byte(`{"data":[{"id":{"attribute_id":"attr-stage"},"title":"Stage","api_slug":"stage","type":"select","is_writable":true,"is_required":true}]}`))
	}))

	output, err := executeTestCommand(t, newListsCommand(), "attributes", "pipeline")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertContains(t, output, "stage")
	assertContains(t, output, "Stage")
	assertContains(t, output, "select")
	assertContains(t, output, "yes")
}

func TestListsAttributesHelpExplainsListArgument(t *testing.T) {
	output, err := executeTestCommand(t, newListsCommand(), "attributes", "--help")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertContains(t, output, "slug or ID")
	assertContains(t, output, "list entries")
}

func attioTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	t.Setenv("ATTIO_ACCESS_TOKEN", "test-token")

	server := httptest.NewServer(handler)
	oldNewAttioClient := newAttioClient
	newAttioClient = func(token string) *attio.Client {
		if token != "test-token" {
			t.Fatalf("expected test token, got %q", token)
		}
		return attio.NewClient(token, attio.WithBaseURL(server.URL), attio.WithHTTPClient(server.Client()))
	}
	t.Cleanup(func() {
		newAttioClient = oldNewAttioClient
		server.Close()
	})
	return server
}

func executeTestCommand(t *testing.T, command *cobra.Command, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&out)
	command.SetArgs(args)

	err := command.Execute()
	return out.String(), err
}

func assertNotContains(t *testing.T, output, unexpected string) {
	t.Helper()
	normalizedOutput := strings.Join(strings.Fields(output), " ")
	normalizedUnexpected := strings.Join(strings.Fields(unexpected), " ")
	if strings.Contains(normalizedOutput, normalizedUnexpected) {
		t.Fatalf("expected output not to contain %q, got:\n%s", unexpected, output)
	}
}
