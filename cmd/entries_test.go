package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"atcli/internal/attio"

	"github.com/zalando/go-keyring"
)

func TestEntriesAddTableOutput(t *testing.T) {
	writeCalled := false
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lists":
			_, _ = w.Write([]byte(`{"data":[{"id":{"list_id":"list-sales"},"api_slug":"sales","name":"Sales Pipeline","parent_object":["people"]}]}`))
			return
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"id":{"object_id":"object-people"},"api_slug":"people","singular_noun":"Person","plural_noun":"People"}]}`))
			return
		case "/lists/sales/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"stage","is_writable":true},{"api_slug":"priority","type":"number","is_writable":true}]}`))
			return
		case "/lists/sales/entries":
			writeCalled = true
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
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
		if payload.Data.ParentRecordID != "record-people" || payload.Data.ParentObject != "people" {
			t.Fatalf("unexpected parent payload: %#v", payload.Data)
		}
		if payload.Data.EntryValues["stage"] != "Qualified" || payload.Data.EntryValues["priority"] != float64(3) {
			t.Fatalf("unexpected entry values: %#v", payload.Data.EntryValues)
		}

		_, _ = w.Write([]byte(`{"data":{"id":{"list_id":"list-sales","entry_id":"entry-sales"},"parent_record_id":"record-people","parent_object":"people","created_at":"2026-05-08T12:00:00Z","status":"created","created":true}}`))
	}))

	output, err := executeTestCommand(t, newEntriesCommand(), "add", "sales", "--parent-object", "people", "--parent-record-id", "record-people", "--set", "stage=Qualified", "--set-json", "priority=3")
	if err != nil {
		t.Fatalf("expected no error, got %v\n%s", err, output)
	}
	if !writeCalled {
		t.Fatal("expected write endpoint to be called")
	}

	assertContains(t, output, "sales")
	assertContains(t, output, "list-sales")
	assertContains(t, output, "entry-sales")
	assertContains(t, output, "Sales Pipeline")
	assertContains(t, output, "Person")
	assertContains(t, output, "created")
}

func TestEntriesAddDryRunPayloadPreviewAvoidsClient(t *testing.T) {
	oldNewAttioClient := newAttioClient
	newAttioClient = func(token string) *attio.Client {
		t.Fatalf("dry run should not create an Attio client with token %q", token)
		return nil
	}
	t.Cleanup(func() {
		newAttioClient = oldNewAttioClient
	})

	output, err := executeTestCommand(t, newEntriesCommand(), "add", "sales", "--parent-object", "people", "--parent-record-id", "record-people", "--set", "stage=Qualified", "--dry-run")
	if err != nil {
		t.Fatalf("expected no error, got %v\n%s", err, output)
	}

	assertContains(t, output, "DRY RUN")
	assertContains(t, output, "no write endpoint called")
	assertContains(t, output, `"parent_record_id": "record-people"`)
	assertContains(t, output, `"parent_object": "people"`)
	assertContains(t, output, `"entry_values"`)
	assertContains(t, output, `"stage": "Qualified"`)
}

func TestEntriesAddRequiresParentFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "parent object",
			args: []string{"add", "sales", "--parent-record-id", "record-people"},
			want: "--parent-object is required",
		},
		{
			name: "parent record",
			args: []string{"add", "sales", "--parent-object", "people"},
			want: "--parent-record-id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := executeTestCommand(t, newEntriesCommand(), tt.args...)
			if err == nil {
				t.Fatal("expected missing parent flag error")
			}
			assertErrorContains(t, err, tt.want)
		})
	}
}

func TestEntriesAddFallsBackWhenMetadataPermissionMissing(t *testing.T) {
	writeCalled := false
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lists":
			http.Error(w, `{"error":"missing list_configuration:read"}`, http.StatusForbidden)
			return
		case "/lists/sales/entries":
			writeCalled = true
			_, _ = w.Write([]byte(`{"data":{"id":{"entry_id":"entry-sales"},"parent_record_id":"record-people","parent_object":"people"}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
	}))

	output, err := executeTestCommand(t, newEntriesCommand(), "add", "sales", "--parent-object", "people", "--parent-record-id", "record-people", "--set", "custom=kept")
	if err != nil {
		t.Fatalf("expected no error, got %v\n%s", err, output)
	}
	if !writeCalled {
		t.Fatal("expected write endpoint to be called after metadata permission fallback")
	}
	assertContains(t, output, "Metadata unavailable")
	assertContains(t, output, "entry-sales")
}

func TestEntriesAddJSONOutput(t *testing.T) {
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lists":
			_, _ = w.Write([]byte(`{"data":[{"id":{"list_id":"list-sales"},"api_slug":"sales","name":"Sales Pipeline","parent_object":["people"]}]}`))
			return
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"id":{"object_id":"object-people"},"api_slug":"people","singular_noun":"Person","plural_noun":"People"}]}`))
			return
		case "/lists/sales/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"stage","is_writable":true}]}`))
			return
		case "/lists/sales/entries":
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
		_, _ = w.Write([]byte(`{"data":{"id":{"list_id":"list-sales","entry_id":"entry-sales"},"parent_record_id":"record-people","parent_object":"people","created_at":"2026-05-08T12:00:00Z","created":true}}`))
	}))

	output, err := executeTestCommand(t, newEntriesCommand(), "add", "sales", "--parent-object", "people", "--parent-record-id", "record-people", "--set", "stage=Qualified", "--output", "json")
	if err != nil {
		t.Fatalf("expected no error, got %v\n%s", err, output)
	}

	var got struct {
		DryRun              bool `json:"dry_run"`
		WriteEndpointCalled bool `json:"write_endpoint_called"`
		Created             *bool
		List                struct {
			Identifier string `json:"identifier"`
			ListID     string `json:"list_id"`
			Name       string `json:"name"`
		} `json:"list"`
		Parent struct {
			Object       string `json:"object"`
			ObjectID     string `json:"object_id"`
			RecordID     string `json:"record_id"`
			SingularNoun string `json:"singular_noun"`
		} `json:"parent"`
		Entry struct {
			EntryID string `json:"entry_id"`
		} `json:"entry"`
	}
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("decode JSON output: %v\n%s", err, output)
	}
	if got.DryRun || !got.WriteEndpointCalled {
		t.Fatalf("unexpected write markers: %#v", got)
	}
	if got.Created == nil || !*got.Created {
		t.Fatalf("expected created marker, got %#v", got.Created)
	}
	if got.List.Identifier != "sales" || got.List.ListID != "list-sales" || got.List.Name != "Sales Pipeline" {
		t.Fatalf("unexpected list output: %#v", got.List)
	}
	if got.Parent.Object != "people" || got.Parent.ObjectID != "object-people" || got.Parent.RecordID != "record-people" || got.Parent.SingularNoun != "Person" {
		t.Fatalf("unexpected parent output: %#v", got.Parent)
	}
	if got.Entry.EntryID != "entry-sales" {
		t.Fatalf("unexpected entry output: %#v", got.Entry)
	}
}

func TestEntriesAddHelp(t *testing.T) {
	output, err := executeTestCommand(t, newEntriesCommand(), "add", "--help")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	for _, expected := range []string{
		"slug or ID",
		"--parent-object",
		"--parent-record-id",
		"--set",
		"--set-json",
		"--dry-run",
		"--output",
		"List entries point at records",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected help to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestEntriesHelpIncludesAdd(t *testing.T) {
	output, err := executeTestCommand(t, newEntriesCommand(), "--help")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	assertContains(t, output, "add")
	assertContains(t, output, "upsert")
}

func TestEntriesAddMissingAuthClassification(t *testing.T) {
	t.Setenv("ATTIO_ACCESS_TOKEN", "")
	keyring.MockInit()
	oldNewAttioClient := newAttioClient
	newAttioClient = func(token string) *attio.Client {
		t.Fatalf("should not create client without credentials: %q", token)
		return nil
	}
	t.Cleanup(func() {
		newAttioClient = oldNewAttioClient
	})

	_, err := executeTestCommand(t, newEntriesCommand(), "add", "sales", "--parent-object", "people", "--parent-record-id", "record-people")
	if err == nil {
		t.Fatal("expected auth error")
	}
	assertErrorContains(t, err, "not authenticated")
	assertErrorContains(t, err, "ATTIO_ACCESS_TOKEN")
}

func TestEntriesUpsertTableOutput(t *testing.T) {
	writeCalled := false
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lists":
			_, _ = w.Write([]byte(`{"data":[{"id":{"list_id":"list-sales"},"api_slug":"sales","name":"Sales Pipeline","parent_object":["people"]}]}`))
			return
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"id":{"object_id":"object-people"},"api_slug":"people","singular_noun":"Person","plural_noun":"People"}]}`))
			return
		case "/lists/sales/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"stage","is_writable":true}]}`))
			return
		case "/lists/sales/entries":
			writeCalled = true
			if r.Method != http.MethodPut {
				t.Fatalf("expected PUT, got %s", r.Method)
			}
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
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
		if payload.Data.ParentRecordID != "record-people" || payload.Data.ParentObject != "people" || payload.Data.EntryValues["stage"] != "Qualified" {
			t.Fatalf("unexpected assert payload: %#v", payload.Data)
		}

		_, _ = w.Write([]byte(`{"data":{"id":{"list_id":"list-sales","entry_id":"entry-sales"},"parent_record_id":"record-people","parent_object":"people","operation":"updated","created":false}}`))
	}))

	output, err := executeTestCommand(t, newEntriesCommand(), "upsert", "sales", "--parent-object", "people", "--parent-record-id", "record-people", "--set", "stage=Qualified")
	if err != nil {
		t.Fatalf("expected no error, got %v\n%s", err, output)
	}
	if !writeCalled {
		t.Fatal("expected write endpoint to be called")
	}
	assertContains(t, output, "entry-sales")
	assertContains(t, output, "updated")
	assertContains(t, output, "Person")
}

func TestEntriesUpsertParentObjectMismatch(t *testing.T) {
	writeCalled := false
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lists":
			_, _ = w.Write([]byte(`{"data":[{"id":{"list_id":"list-sales"},"api_slug":"sales","name":"Sales Pipeline","parent_object":["companies"]}]}`))
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"id":{"object_id":"object-people"},"api_slug":"people","singular_noun":"Person","plural_noun":"People"},{"id":{"object_id":"object-companies"},"api_slug":"companies","singular_noun":"Company","plural_noun":"Companies"}]}`))
		case "/lists/sales/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"stage","is_writable":true}]}`))
		case "/lists/sales/entries":
			writeCalled = true
			t.Fatalf("write endpoint should not be called after parent object mismatch")
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
	}))

	_, err := executeTestCommand(t, newEntriesCommand(), "upsert", "sales", "--parent-object", "people", "--parent-record-id", "record-people", "--set", "stage=Qualified")
	if err == nil {
		t.Fatal("expected parent object mismatch error")
	}
	if writeCalled {
		t.Fatal("write endpoint was called")
	}
	assertErrorContains(t, err, `list "sales" accepts parent object "companies"; got "people"`)
}

func TestEntriesUpsertMissingMetadataFallback(t *testing.T) {
	writeCalled := false
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lists":
			http.Error(w, `{"error":"missing list_configuration:read"}`, http.StatusForbidden)
			return
		case "/lists/sales/entries":
			writeCalled = true
			if r.Method != http.MethodPut {
				t.Fatalf("expected PUT, got %s", r.Method)
			}
			_, _ = w.Write([]byte(`{"data":{"id":{"entry_id":"entry-sales"},"parent_record_id":"record-people","parent_object":"people","created":true}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
	}))

	output, err := executeTestCommand(t, newEntriesCommand(), "upsert", "sales", "--parent-object", "people", "--parent-record-id", "record-people", "--set", "custom=kept")
	if err != nil {
		t.Fatalf("expected no error, got %v\n%s", err, output)
	}
	if !writeCalled {
		t.Fatal("expected write endpoint to be called after metadata permission fallback")
	}
	assertContains(t, output, "Metadata unavailable")
	assertContains(t, output, "entry-sales")
}

func TestEntriesUpsertDryRunPayloadPreview(t *testing.T) {
	oldNewAttioClient := newAttioClient
	newAttioClient = func(token string) *attio.Client {
		t.Fatalf("dry run should not create an Attio client with token %q", token)
		return nil
	}
	t.Cleanup(func() {
		newAttioClient = oldNewAttioClient
	})

	output, err := executeTestCommand(t, newEntriesCommand(), "upsert", "sales", "--parent-object", "people", "--parent-record-id", "record-people", "--set-json", `stage=["Qualified"]`, "--dry-run")
	if err != nil {
		t.Fatalf("expected no error, got %v\n%s", err, output)
	}
	assertContains(t, output, "DRY RUN")
	assertContains(t, output, `"parent_record_id": "record-people"`)
	assertContains(t, output, `"stage"`)
	assertContains(t, output, `"Qualified"`)
}

func TestEntriesUpsertJSONOutput(t *testing.T) {
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lists":
			_, _ = w.Write([]byte(`{"data":[{"id":{"list_id":"list-sales"},"api_slug":"sales","name":"Sales Pipeline","parent_object":["people"]}]}`))
			return
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"id":{"object_id":"object-people"},"api_slug":"people","singular_noun":"Person","plural_noun":"People"}]}`))
			return
		case "/lists/sales/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"stage","is_writable":true}]}`))
			return
		case "/lists/sales/entries":
			if r.Method != http.MethodPut {
				t.Fatalf("expected PUT, got %s", r.Method)
			}
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
		_, _ = w.Write([]byte(`{"data":{"id":{"list_id":"list-sales","entry_id":"entry-sales"},"parent_record_id":"record-people","parent_object":"people","created_at":"2026-05-08T12:00:00Z","outcome":"updated","created":false}}`))
	}))

	output, err := executeTestCommand(t, newEntriesCommand(), "upsert", "sales", "--parent-object", "people", "--parent-record-id", "record-people", "--set", "stage=Qualified", "--output", "json")
	if err != nil {
		t.Fatalf("expected no error, got %v\n%s", err, output)
	}

	var got struct {
		DryRun              bool   `json:"dry_run"`
		WriteEndpointCalled bool   `json:"write_endpoint_called"`
		Outcome             string `json:"outcome"`
		Created             *bool  `json:"created"`
		Entry               struct {
			EntryID string `json:"entry_id"`
		} `json:"entry"`
	}
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("decode JSON output: %v\n%s", err, output)
	}
	if got.DryRun || !got.WriteEndpointCalled || got.Outcome != "updated" {
		t.Fatalf("unexpected write output: %#v", got)
	}
	if got.Created == nil || *got.Created {
		t.Fatalf("expected created=false, got %#v", got.Created)
	}
	if got.Entry.EntryID != "entry-sales" {
		t.Fatalf("unexpected entry output: %#v", got.Entry)
	}
}

func TestEntriesUpsertHelp(t *testing.T) {
	output, err := executeTestCommand(t, newEntriesCommand(), "upsert", "--help")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	for _, expected := range []string{
		"slug or ID",
		"--parent-object",
		"--parent-record-id",
		"--set",
		"--set-json",
		"--dry-run",
		"--output",
		"parent record",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected help to contain %q, got:\n%s", expected, output)
		}
	}
}
