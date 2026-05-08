package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecordsImportPlansUpsertAndAvoidsWriteEndpoint(t *testing.T) {
	csvPath := writeImportCSV(t, "people.csv", "email_addresses,name\nada@example.com,Ada Lovelace\n")
	writeEndpointCalled := false
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"id":{"object_id":"object-people"},"api_slug":"people","singular_noun":"Person","plural_noun":"People"}]}`))
		case "/objects/people/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"email_addresses","type":"email-address","is_writable":true,"is_unique":true},{"api_slug":"name","type":"personal-name","is_writable":true}]}`))
		case "/objects/people/records":
			writeEndpointCalled = true
			t.Fatalf("write endpoint should not be called in import dry-run")
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if writeEndpointCalled {
		t.Fatal("write endpoint was called")
	}
	for _, expected := range []string{
		"DRY RUN",
		"Mode: upsert",
		"Matching attribute: email_addresses",
		"valid",
		"ada@example.com",
	} {
		assertContains(t, output, expected)
	}
}

func TestRecordsImportApplyCreateModeWritesPlannedRows(t *testing.T) {
	csvPath := writeImportCSV(t, "companies.csv", "name,domains\nExample Co,example.com\n")
	writeCalled := false
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"id":{"object_id":"object-company"},"api_slug":"companies","singular_noun":"Company","plural_noun":"Companies"}]}`))
			return
		case "/objects/companies/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"name","type":"text","is_writable":true,"is_required":true},{"api_slug":"domains","type":"domain","is_writable":true}]}`))
			return
		case "/objects/companies/records":
			writeCalled = true
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}

		var payload struct {
			Data struct {
				Values map[string]any `json:"values"`
			} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.Data.Values["name"] != "Example Co" || payload.Data.Values["domains"] != "example.com" {
			t.Fatalf("unexpected values payload: %#v", payload.Data.Values)
		}
		_, _ = w.Write([]byte(`{"data":{"id":{"object_id":"object-company","record_id":"record-company-1"}}}`))
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "companies", csvPath, "--mode", "create", "--apply")
	if err != nil {
		t.Fatalf("expected no error, got %v\n%s", err, output)
	}
	if !writeCalled {
		t.Fatal("expected write endpoint to be called")
	}
	assertContains(t, output, "APPLY")
	assertContains(t, output, "Mode: create")
	assertContains(t, output, "record-company-1")
	assertContains(t, output, "succeeded")
}

func TestRecordsImportApplyUpsertModePropagatesIdentity(t *testing.T) {
	csvPath := writeImportCSV(t, "people-upsert.csv", "email_addresses,name\nada@example.com,Ada Lovelace\n")
	writeCalled := false
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"id":{"object_id":"object-people"},"api_slug":"people","singular_noun":"Person","plural_noun":"People"}]}`))
			return
		case "/objects/people/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"email_addresses","type":"email-address","is_writable":true,"is_unique":true},{"api_slug":"name","type":"personal-name","is_writable":true}]}`))
			return
		case "/objects/people/records":
			writeCalled = true
			if r.Method != http.MethodPut {
				t.Fatalf("expected PUT, got %s", r.Method)
			}
			if got := r.URL.Query().Get("matching_attribute"); got != "email_addresses" {
				t.Fatalf("expected matching_attribute=email_addresses, got %q", got)
			}
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}

		var payload struct {
			Data struct {
				Values map[string]any `json:"values"`
			} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.Data.Values["email_addresses"] != "ada@example.com" {
			t.Fatalf("unexpected values payload: %#v", payload.Data.Values)
		}
		_, _ = w.Write([]byte(`{"data":{"id":{"object_id":"object-people","record_id":"record-person-1"},"status":"updated","created":false}}`))
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--apply")
	if err != nil {
		t.Fatalf("expected no error, got %v\n%s", err, output)
	}
	if !writeCalled {
		t.Fatal("expected write endpoint to be called")
	}
	assertContains(t, output, "Mode: upsert")
	assertContains(t, output, "Matching attribute: email_addresses")
	assertContains(t, output, "record-person-1")
	assertContains(t, output, "updated")
}

func TestRecordsImportApplyRetriesRateLimitWithRetryAfter(t *testing.T) {
	sleeps := captureImportRateLimitSleeps(t)
	csvPath := writeImportCSV(t, "retry-success.csv", "email_addresses,name\nada@example.com,Ada Lovelace\ncharles@example.com,Charles Babbage\n")
	attemptsByEmail := map[string]int{}
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"people"}]}`))
			return
		case "/objects/people/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"email_addresses","type":"email-address","is_writable":true,"is_unique":true},{"api_slug":"name","type":"personal-name","is_writable":true}]}`))
			return
		case "/objects/people/records":
			if r.Method != http.MethodPut {
				t.Fatalf("expected PUT, got %s", r.Method)
			}
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}

		var payload struct {
			Data struct {
				Values map[string]any `json:"values"`
			} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		email, _ := payload.Data.Values["email_addresses"].(string)
		attemptsByEmail[email]++
		if email == "charles@example.com" && attemptsByEmail[email] == 1 {
			w.Header().Set("Retry-After", "2")
			http.Error(w, `{"error":"rate limited"}`, http.StatusTooManyRequests)
			return
		}
		created := email == "charles@example.com"
		status := "updated"
		recordID := "record-ada"
		if created {
			status = "created"
			recordID = "record-charles"
		}
		_, _ = fmt.Fprintf(w, `{"data":{"id":{"record_id":%q},"status":%q,"created":%t}}`, recordID, status, created)
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--apply")
	if err != nil {
		t.Fatalf("expected no error, got %v\n%s", err, output)
	}
	if attemptsByEmail["ada@example.com"] != 1 || attemptsByEmail["charles@example.com"] != 2 {
		t.Fatalf("unexpected attempts by email: %#v", attemptsByEmail)
	}
	if len(*sleeps) != 1 || (*sleeps)[0] != 2*time.Second {
		t.Fatalf("expected one Retry-After sleep of 2s, got %#v", *sleeps)
	}
	assertContains(t, output, "record-ada")
	assertContains(t, output, "record-charles")
	assertContains(t, output, "Rows: 2 (succeeded: 2, failed: 0, created: 1, updated: 1)")
}

func TestRecordsImportApplyRateLimitExhaustionReportsFailure(t *testing.T) {
	sleeps := captureImportRateLimitSleeps(t)
	csvPath := writeImportCSV(t, "retry-exhausted.csv", "email_addresses,name\nada@example.com,Ada Lovelace\ncharles@example.com,Charles Babbage\n")
	attemptsByEmail := map[string]int{}
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"people"}]}`))
			return
		case "/objects/people/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"email_addresses","type":"email-address","is_writable":true,"is_unique":true},{"api_slug":"name","type":"personal-name","is_writable":true}]}`))
			return
		case "/objects/people/records":
			if r.Method != http.MethodPut {
				t.Fatalf("expected PUT, got %s", r.Method)
			}
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}

		var payload struct {
			Data struct {
				Values map[string]any `json:"values"`
			} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		email, _ := payload.Data.Values["email_addresses"].(string)
		attemptsByEmail[email]++
		if email == "charles@example.com" {
			http.Error(w, `{"error":"still rate limited"}`, http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"id":{"record_id":"record-ada"},"status":"updated","created":false}}`))
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--apply")
	if err == nil {
		t.Fatalf("expected apply failure, got nil\n%s", output)
	}
	if attemptsByEmail["ada@example.com"] != 1 || attemptsByEmail["charles@example.com"] != maxImportRateLimitRetries+1 {
		t.Fatalf("unexpected attempts by email: %#v", attemptsByEmail)
	}
	wantSleeps := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond}
	if len(*sleeps) != len(wantSleeps) {
		t.Fatalf("expected sleeps %#v, got %#v", wantSleeps, *sleeps)
	}
	for i, want := range wantSleeps {
		if (*sleeps)[i] != want {
			t.Fatalf("expected sleep %d to be %s, got %s", i, want, (*sleeps)[i])
		}
	}
	assertContains(t, output, "record-ada")
	assertContains(t, output, "failed")
	assertContains(t, output, "rate limit")
	assertContains(t, output, "Rows: 2 (succeeded: 1, failed: 1, created: 0, updated: 1)")
}

func TestRecordsImportApplyReusesPlanValidationBeforeWrites(t *testing.T) {
	csvPath := writeImportCSV(t, "invalid-apply.csv", "email_addresses,name\nada@example.com,\n")
	writeCalled := false
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"people"}]}`))
		case "/objects/people/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"email_addresses","type":"email-address","is_writable":true,"is_unique":true},{"api_slug":"name","type":"personal-name","is_writable":true,"is_required":true}]}`))
		case "/objects/people/records":
			writeCalled = true
			t.Fatalf("write endpoint should not be called after planned row validation failure")
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--apply")
	if err == nil {
		t.Fatalf("expected apply failure, got nil\n%s", output)
	}
	if writeCalled {
		t.Fatal("write endpoint was called")
	}
	assertContains(t, output, "failed")
	assertContains(t, output, `missing required attribute "name"`)
}

func TestRecordsImportPlansCreate(t *testing.T) {
	csvPath := writeImportCSV(t, "companies.csv", "name,domains\nExample Co,example.com\n")
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"companies","singular_noun":"Company","plural_noun":"Companies"}]}`))
		case "/objects/companies/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"name","type":"text","is_writable":true,"is_required":true},{"api_slug":"domains","type":"domain","is_writable":true}]}`))
		case "/objects/companies/records":
			t.Fatalf("write endpoint should not be called in import dry-run")
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "companies", csvPath, "--mode", "create")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	assertContains(t, output, "Mode: create")
	assertContains(t, output, "valid")
	assertNotContains(t, output, "Matching attribute")
}

func TestRecordsImportUnknownColumnError(t *testing.T) {
	csvPath := writeImportCSV(t, "unknown.csv", "nickname\nAda\n")
	attioTestServer(t, metadataValidationHandler(t, `[{"api_slug":"name","is_writable":true}]`))

	_, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--mode", "create")
	if err == nil {
		t.Fatal("expected unknown attribute error")
	}
	assertErrorContains(t, err, `unknown attribute "nickname"`)
}

func TestRecordsImportReportsMissingRequiredValues(t *testing.T) {
	csvPath := writeImportCSV(t, "missing-required.csv", "name,email_addresses\n,ada@example.com\n")
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"people"}]}`))
		case "/objects/people/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"name","type":"personal-name","is_writable":true,"is_required":true},{"api_slug":"email_addresses","type":"email-address","is_writable":true,"is_unique":true}]}`))
		case "/objects/people/records":
			t.Fatalf("write endpoint should not be called in import dry-run")
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath)
	if err != nil {
		t.Fatalf("expected invalid row plan without command error, got %v", err)
	}
	assertContains(t, output, "invalid")
	assertContains(t, output, `missing required attribute "name"`)
}

func TestRecordsImportTableOutputSummarizesPlan(t *testing.T) {
	csvPath := writeImportCSV(t, "summary.csv", "email_addresses,name,notes\nada@example.com,Ada,\ncharles@example.com,Charles,Keep\n")
	attioTestServer(t, importMetadataHandler(t))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	for _, expected := range []string{
		"Rows: 2 (valid: 2, invalid: 0)",
		"Skipped empty cells: 1",
		"Warnings: 0",
		"Sample planned rows: first 2 of 2",
		"ROW",
		"STATUS",
	} {
		assertContains(t, output, expected)
	}
}

func TestRecordsImportJSONLOutputShape(t *testing.T) {
	csvPath := writeImportCSV(t, "jsonl.csv", "email_addresses,name,notes\nada@example.com,Ada,\n")
	attioTestServer(t, importMetadataHandler(t))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--output", "jsonl")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one JSONL event, got %d:\n%s", len(lines), output)
	}
	var event struct {
		Type              string         `json:"type"`
		RowNumber         int            `json:"row_number"`
		Mode              string         `json:"mode"`
		Object            string         `json:"object"`
		MatchingAttribute string         `json:"matching_attribute"`
		MatchDefaulted    bool           `json:"match_defaulted"`
		Values            map[string]any `json:"values"`
		SkippedEmpty      []struct {
			CSVColumn string `json:"csv_column"`
			Attribute string `json:"attribute"`
		} `json:"skipped_empty"`
		Valid               bool   `json:"valid"`
		ValidationStatus    string `json:"validation_status"`
		MetadataAvailable   bool   `json:"metadata_available"`
		WriteEndpointCalled bool   `json:"write_endpoint_called"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("decode JSONL event: %v\n%s", err, output)
	}
	if event.Type != "row" || event.RowNumber != 2 || event.Mode != "upsert" || event.Object != "people" {
		t.Fatalf("unexpected JSONL event identity: %#v", event)
	}
	if event.MatchingAttribute != "email_addresses" || !event.MatchDefaulted {
		t.Fatalf("unexpected match fields: %#v", event)
	}
	if event.Values["email_addresses"] != "ada@example.com" || event.Values["name"] != "Ada" {
		t.Fatalf("unexpected values: %#v", event.Values)
	}
	if len(event.SkippedEmpty) != 1 || event.SkippedEmpty[0].Attribute != "notes" {
		t.Fatalf("unexpected skipped empty cells: %#v", event.SkippedEmpty)
	}
	if !event.Valid || event.ValidationStatus != "valid" {
		t.Fatalf("unexpected validation status: %#v", event)
	}
	if !event.MetadataAvailable || event.WriteEndpointCalled {
		t.Fatalf("unexpected metadata/write markers: %#v", event)
	}
	assertNotContains(t, output, "record_id")
}

func TestRecordsImportJSONLIncludesWarningsWithoutInventedMetadata(t *testing.T) {
	csvPath := writeImportCSV(t, "fallback-jsonl.csv", "external_id,name\nwidget-1,Widget\n")
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			http.Error(w, `{"error":"missing object_configuration:read"}`, http.StatusForbidden)
		case "/objects/custom_widgets/records":
			t.Fatalf("write endpoint should not be called in import dry-run")
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "custom_widgets", csvPath, "--match", "external_id", "--output", "jsonl")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var event struct {
		Object            string   `json:"object"`
		Warnings          []string `json:"warnings"`
		MetadataAvailable bool     `json:"metadata_available"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &event); err != nil {
		t.Fatalf("decode JSONL event: %v\n%s", err, output)
	}
	if event.Object != "custom_widgets" || event.MetadataAvailable {
		t.Fatalf("unexpected missing-metadata event: %#v", event)
	}
	if len(event.Warnings) != 1 || !strings.Contains(event.Warnings[0], "Metadata unavailable") {
		t.Fatalf("expected metadata warning, got %#v", event.Warnings)
	}
	assertNotContains(t, output, "object_id")
	assertNotContains(t, output, "record_id")
}

func TestRecordsImportNonWritableAttributeError(t *testing.T) {
	csvPath := writeImportCSV(t, "non-writable.csv", "name\nAda\n")
	attioTestServer(t, metadataValidationHandler(t, `[{"api_slug":"name","is_writable":false}]`))

	_, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--mode", "create")
	if err == nil {
		t.Fatal("expected non-writable attribute error")
	}
	assertErrorContains(t, err, `attribute "name" is not writable`)
}

func TestRecordsImportMetadataPermissionFallback(t *testing.T) {
	csvPath := writeImportCSV(t, "fallback.csv", "external_id,name\nwidget-1,Widget\n")
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			http.Error(w, `{"error":"missing object_configuration:read"}`, http.StatusForbidden)
		case "/objects/custom_widgets/records":
			t.Fatalf("write endpoint should not be called in import dry-run")
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "custom_widgets", csvPath, "--match", "external_id")
	if err != nil {
		t.Fatalf("expected metadata fallback plan, got %v", err)
	}
	assertContains(t, output, "Metadata unavailable")
	assertContains(t, output, "match uniqueness validation skipped")
	assertContains(t, output, "valid")
}

func TestRecordsImportHelp(t *testing.T) {
	output, err := executeTestCommand(t, newRecordsCommand(), "import", "--help")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	for _, expected := range []string{
		"slug or ID",
		"--map",
		"--ignore",
		"--apply",
		"--set",
		"--set-json",
		"--mode",
		"--output",
		"--match",
		"--multi-sep",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected help to contain %q, got:\n%s", expected, output)
		}
	}
}

func importMetadataHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"people","singular_noun":"Person","plural_noun":"People"}]}`))
		case "/objects/people/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"email_addresses","type":"email-address","is_writable":true,"is_unique":true},{"api_slug":"name","type":"personal-name","is_writable":true,"is_required":true},{"api_slug":"notes","type":"text","is_writable":true}]}`))
		case "/objects/people/records":
			t.Fatalf("write endpoint should not be called in import dry-run")
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
	}
}

func writeImportCSV(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write import CSV fixture: %v", err)
	}
	return path
}

func captureImportRateLimitSleeps(t *testing.T) *[]time.Duration {
	t.Helper()
	sleeps := make([]time.Duration, 0)
	oldSleep := importRateLimitSleep
	importRateLimitSleep = func(_ context.Context, delay time.Duration) error {
		sleeps = append(sleeps, delay)
		return nil
	}
	t.Cleanup(func() {
		importRateLimitSleep = oldSleep
	})
	return &sleeps
}
