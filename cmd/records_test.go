package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"atcli/internal/attio"

	"github.com/zalando/go-keyring"
)

func TestRecordsCreateTableOutput(t *testing.T) {
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/objects/people/records" {
			t.Fatalf("expected create path, got %s", r.URL.String())
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
			t.Fatalf("unexpected values payload: %#v", payload.Data.Values)
		}

		_, _ = w.Write([]byte(`{"data":{"id":{"object_id":"object-123","record_id":"record-123"},"created_at":"2026-05-07T12:00:00Z","web_url":"https://app.attio.com/acme/person/record-123"}}`))
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "create", "people", "--set", "name=Ada Lovelace")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertContains(t, output, "people")
	assertContains(t, output, "object-123")
	assertContains(t, output, "record-123")
}

func TestRecordsCreateDryRunAvoidsWriteEndpoint(t *testing.T) {
	t.Setenv("ATTIO_ACCESS_TOKEN", "")
	keyring.MockInitWithError(errors.New("dry run should not load credentials"))

	oldNewAttioClient := newAttioClient
	newAttioClient = func(token string) *attio.Client {
		t.Fatalf("dry run should not create an Attio client")
		return nil
	}
	t.Cleanup(func() {
		newAttioClient = oldNewAttioClient
	})

	output, err := executeTestCommand(t, newRecordsCommand(), "create", "people", "--set", "name=Ada Lovelace", "--dry-run")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertContains(t, output, "DRY RUN")
	assertContains(t, output, "no write endpoint called")
	assertContains(t, output, `"values"`)
	assertContains(t, output, `"name": "Ada Lovelace"`)
}

func TestRecordsCreateJSONOutput(t *testing.T) {
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/objects/companies/records" {
			t.Fatalf("expected create path, got %s", r.URL.String())
		}
		_, _ = w.Write([]byte(`{"data":{"id":{"object_id":"object-company","record_id":"record-company"},"web_url":"https://app.attio.com/acme/company/record-company"}}`))
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "create", "companies", "--set-json", `domains=["example.com"]`, "--output", "json")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var got struct {
		DryRun              bool `json:"dry_run"`
		WriteEndpointCalled bool `json:"write_endpoint_called"`
		Object              struct {
			Identifier string `json:"identifier"`
			ObjectID   string `json:"object_id"`
		} `json:"object"`
		Record struct {
			RecordID string `json:"record_id"`
		} `json:"record"`
	}
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("decode JSON output: %v\n%s", err, output)
	}
	if got.DryRun || !got.WriteEndpointCalled {
		t.Fatalf("unexpected write markers: %#v", got)
	}
	if got.Object.Identifier != "companies" || got.Object.ObjectID != "object-company" {
		t.Fatalf("unexpected object output: %#v", got.Object)
	}
	if got.Record.RecordID != "record-company" {
		t.Fatalf("unexpected record output: %#v", got.Record)
	}
}

func TestRecordsCreateMissingAuthClassification(t *testing.T) {
	t.Setenv("ATTIO_ACCESS_TOKEN", "")
	keyring.MockInit()

	_, err := executeTestCommand(t, newRecordsCommand(), "create", "people", "--set", "name=Ada")
	if err == nil {
		t.Fatal("expected auth error")
	}
	assertErrorContains(t, err, "not authenticated")
	assertErrorContains(t, err, "ATTIO_ACCESS_TOKEN")
}

func TestRecordsCreateHelp(t *testing.T) {
	output, err := executeTestCommand(t, newRecordsCommand(), "create", "--help")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	for _, expected := range []string{
		"slug or ID",
		"people",
		"companies",
		"--set",
		"--set-json",
		"--dry-run",
		"--output",
		"does not singularize or pluralize",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected help to contain %q, got:\n%s", expected, output)
		}
	}
}
