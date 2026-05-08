package cmd

import (
	"context"
	"encoding/csv"
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

func TestRecordsImportApplyJSONLOutputIncludesRowsAndSummary(t *testing.T) {
	csvPath := writeImportCSV(t, "apply-jsonl.csv", "email_addresses,name\nada@example.com,Ada Lovelace\ncharles@example.com,Charles Babbage\n")
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"people"}]}`))
			return
		case "/objects/people/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"email_addresses","type":"email-address","is_writable":true,"is_unique":true},{"api_slug":"name","type":"personal-name","is_writable":true}]}`))
			return
		case "/objects/people/records":
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}

		email := decodeImportRequestEmail(t, r)
		switch email {
		case "ada@example.com":
			_, _ = w.Write([]byte(`{"data":{"id":{"record_id":"record-ada"},"status":"updated","created":false}}`))
		case "charles@example.com":
			_, _ = w.Write([]byte(`{"data":{"id":{"record_id":"record-charles"},"status":"created","created":true}}`))
		default:
			t.Fatalf("unexpected email %q", email)
		}
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--apply", "--output", "jsonl")
	if err != nil {
		t.Fatalf("expected no error, got %v\n%s", err, output)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected two row events and one summary event, got %d:\n%s", len(lines), output)
	}

	var firstRow struct {
		Type                string `json:"type"`
		RowNumber           int    `json:"row_number"`
		Mode                string `json:"mode"`
		Object              string `json:"object"`
		MatchingAttribute   string `json:"matching_attribute"`
		RecordID            string `json:"record_id"`
		Status              string `json:"status"`
		WriteEndpointCalled bool   `json:"write_endpoint_called"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &firstRow); err != nil {
		t.Fatalf("decode first row event: %v\n%s", err, output)
	}
	if firstRow.Type != "row" || firstRow.RowNumber != 2 || firstRow.Mode != "upsert" || firstRow.Object != "people" {
		t.Fatalf("unexpected first row event identity: %#v", firstRow)
	}
	if firstRow.MatchingAttribute != "email_addresses" || firstRow.RecordID != "record-ada" || firstRow.Status != "updated" || !firstRow.WriteEndpointCalled {
		t.Fatalf("unexpected first row event result: %#v", firstRow)
	}

	var summary struct {
		Type      string `json:"type"`
		Planned   int    `json:"planned"`
		Succeeded int    `json:"succeeded"`
		Failed    int    `json:"failed"`
		Skipped   int    `json:"skipped"`
		Created   int    `json:"created"`
		Updated   int    `json:"updated"`
		ElapsedMS int64  `json:"elapsed_ms"`
		Records   []struct {
			RowNumber int    `json:"row_number"`
			RecordID  string `json:"record_id"`
			Status    string `json:"status"`
		} `json:"records"`
	}
	if err := json.Unmarshal([]byte(lines[2]), &summary); err != nil {
		t.Fatalf("decode summary event: %v\n%s", err, output)
	}
	if summary.Type != "summary" || summary.Planned != 2 || summary.Succeeded != 2 || summary.Failed != 0 || summary.Skipped != 0 {
		t.Fatalf("unexpected summary counts: %#v", summary)
	}
	if summary.Created != 1 || summary.Updated != 1 || summary.ElapsedMS < 0 {
		t.Fatalf("unexpected summary status totals: %#v", summary)
	}
	if len(summary.Records) != 2 || summary.Records[0].RecordID != "record-ada" || summary.Records[1].RecordID != "record-charles" {
		t.Fatalf("unexpected summary records: %#v", summary.Records)
	}
}

func TestRecordsImportApplyTableOutputHandlesMissingCreateUpdateStatus(t *testing.T) {
	csvPath := writeImportCSV(t, "missing-status.csv", "email_addresses,name\nada@example.com,Ada Lovelace\n")
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"people"}]}`))
		case "/objects/people/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"email_addresses","type":"email-address","is_writable":true,"is_unique":true},{"api_slug":"name","type":"personal-name","is_writable":true}]}`))
		case "/objects/people/records":
			_, _ = w.Write([]byte(`{"data":{"id":{"record_id":"record-no-status"}}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--apply")
	if err != nil {
		t.Fatalf("expected no error, got %v\n%s", err, output)
	}
	assertContains(t, output, "record-no-status")
	assertContains(t, output, "succeeded")
	assertContains(t, output, "Rows: 1 planned, 1 succeeded, 0 failed, 0 skipped, 0 created, 0 updated")
	assertContains(t, output, "Elapsed:")
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
	assertContains(t, output, "Rows: 2 planned, 2 succeeded, 0 failed, 0 skipped, 1 created, 1 updated")
	assertContains(t, output, "Elapsed:")
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
	assertContains(t, output, "Rows: 2 planned, 1 succeeded, 1 failed, 0 skipped, 0 created, 1 updated")
	assertContains(t, output, "Elapsed:")
}

func TestRecordsImportApplyStopsAfterWriteFailureByDefault(t *testing.T) {
	csvPath := writeImportCSV(t, "stop-on-write-failure.csv", "email_addresses,name\nada@example.com,Ada Lovelace\nbad@example.com,Bad Row\nthird@example.com,Third Row\n")
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
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}

		email := decodeImportRequestEmail(t, r)
		attemptsByEmail[email]++
		switch email {
		case "ada@example.com":
			_, _ = w.Write([]byte(`{"data":{"id":{"record_id":"record-ada"},"status":"updated","created":false}}`))
		case "bad@example.com":
			http.Error(w, `{"error":"invalid row"}`, http.StatusUnprocessableEntity)
		case "third@example.com":
			t.Fatal("third row should not be written after default stop-on-error")
		default:
			t.Fatalf("unexpected email %q", email)
		}
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--apply")
	if err == nil {
		t.Fatalf("expected apply failure, got nil\n%s", output)
	}
	if attemptsByEmail["ada@example.com"] != 1 || attemptsByEmail["bad@example.com"] != 1 || attemptsByEmail["third@example.com"] != 0 {
		t.Fatalf("unexpected attempts by email: %#v", attemptsByEmail)
	}
	assertContains(t, output, "record-ada")
	assertContains(t, output, "failed")
	assertContains(t, output, "skipped after row 3 failed")
	assertContains(t, output, "Rows: 3 planned, 1 succeeded, 1 failed, 1 skipped, 0 created, 1 updated")
	assertNotContains(t, output, "record-third")
}

func TestRecordsImportApplyContinueOnErrorProcessesWriteFailures(t *testing.T) {
	csvPath := writeImportCSV(t, "continue-write-failure.csv", "email_addresses,name\nada@example.com,Ada Lovelace\nbad@example.com,Bad Row\nthird@example.com,Third Row\n")
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
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}

		email := decodeImportRequestEmail(t, r)
		attemptsByEmail[email]++
		switch email {
		case "ada@example.com":
			_, _ = w.Write([]byte(`{"data":{"id":{"record_id":"record-ada"},"status":"updated","created":false}}`))
		case "bad@example.com":
			http.Error(w, `{"error":"invalid row"}`, http.StatusUnprocessableEntity)
		case "third@example.com":
			_, _ = w.Write([]byte(`{"data":{"id":{"record_id":"record-third"},"status":"created","created":true}}`))
		default:
			t.Fatalf("unexpected email %q", email)
		}
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--apply", "--continue-on-error")
	if err == nil {
		t.Fatalf("expected apply failure, got nil\n%s", output)
	}
	if attemptsByEmail["ada@example.com"] != 1 || attemptsByEmail["bad@example.com"] != 1 || attemptsByEmail["third@example.com"] != 1 {
		t.Fatalf("unexpected attempts by email: %#v", attemptsByEmail)
	}
	assertContains(t, output, "record-ada")
	assertContains(t, output, "record-third")
	assertContains(t, output, "failed")
	assertNotContains(t, output, "skipped after")
}

func TestRecordsImportApplyContinueOnErrorProcessesRowsAfterValidationFailure(t *testing.T) {
	csvPath := writeImportCSV(t, "continue-validation-failure.csv", "email_addresses,name\nada@example.com,\ncharles@example.com,Charles Babbage\n")
	writeCount := 0
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"people"}]}`))
		case "/objects/people/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"email_addresses","type":"email-address","is_writable":true,"is_unique":true},{"api_slug":"name","type":"personal-name","is_writable":true,"is_required":true}]}`))
		case "/objects/people/records":
			writeCount++
			email := decodeImportRequestEmail(t, r)
			if email != "charles@example.com" {
				t.Fatalf("expected only second row write, got %q", email)
			}
			_, _ = w.Write([]byte(`{"data":{"id":{"record_id":"record-charles"},"status":"created","created":true}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--apply", "--continue-on-error")
	if err == nil {
		t.Fatalf("expected apply failure, got nil\n%s", output)
	}
	if writeCount != 1 {
		t.Fatalf("expected one write after validation failure, got %d", writeCount)
	}
	assertContains(t, output, `missing required attribute "name"`)
	assertContains(t, output, "record-charles")
}

func TestRecordsImportApplySanitizesRowErrors(t *testing.T) {
	t.Setenv("API_SECRET", "leaked-secret-value")
	csvPath := writeImportCSV(t, "sanitized-error.csv", "email_addresses,name\nada@example.com,Ada Lovelace\n")
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"people"}]}`))
		case "/objects/people/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"email_addresses","type":"email-address","is_writable":true,"is_unique":true},{"api_slug":"name","type":"personal-name","is_writable":true}]}`))
		case "/objects/people/records":
			http.Error(w, `{"error":"bad value leaked-secret-value test-token"}`, http.StatusUnprocessableEntity)
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--apply")
	if err == nil {
		t.Fatalf("expected apply failure, got nil\n%s", output)
	}
	assertContains(t, output, "[redacted]")
	assertNotContains(t, output, "leaked-secret-value")
	assertNotContains(t, output, "test-token")
	assertNotContains(t, err.Error(), "leaked-secret-value")
}

func TestRecordsImportApplyDoesNotWriteErrorsFileWithoutFailures(t *testing.T) {
	csvPath := writeImportCSV(t, "no-errors.csv", "email_addresses,name\nada@example.com,Ada Lovelace\n")
	errorsPath := filepath.Join(t.TempDir(), "errors.csv")
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"people"}]}`))
		case "/objects/people/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"email_addresses","type":"email-address","is_writable":true,"is_unique":true},{"api_slug":"name","type":"personal-name","is_writable":true}]}`))
		case "/objects/people/records":
			_, _ = w.Write([]byte(`{"data":{"id":{"record_id":"record-ada"},"status":"updated","created":false}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--apply", "--errors", errorsPath)
	if err != nil {
		t.Fatalf("expected no error, got %v\n%s", err, output)
	}
	if _, err := os.Stat(errorsPath); !os.IsNotExist(err) {
		t.Fatalf("expected no errors CSV to be created, stat err=%v", err)
	}
}

func TestRecordsImportApplyWritesOneFailedWriteRowToErrorsCSV(t *testing.T) {
	csvPath := writeImportCSV(t, "one-error.csv", "email_addresses,name,notes\nada@example.com,Ada,ok\nbad@example.com,\"Bad, Row\",\" keep \"\n")
	errorsPath := filepath.Join(t.TempDir(), "errors.csv")
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"people"}]}`))
			return
		case "/objects/people/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"email_addresses","type":"email-address","is_writable":true,"is_unique":true},{"api_slug":"name","type":"personal-name","is_writable":true},{"api_slug":"notes","type":"text","is_writable":true}]}`))
			return
		case "/objects/people/records":
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}

		email := decodeImportRequestEmail(t, r)
		if email == "bad@example.com" {
			http.Error(w, `{"error":"invalid row"}`, http.StatusUnprocessableEntity)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"id":{"record_id":"record-ada"},"status":"updated","created":false}}`))
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--apply", "--continue-on-error", "--errors", errorsPath)
	if err == nil {
		t.Fatalf("expected apply failure, got nil\n%s", output)
	}

	records := readCSVFile(t, errorsPath)
	if len(records) != 2 {
		t.Fatalf("expected header and one failed row, got %#v", records)
	}
	wantHeaderPrefix := []string{"email_addresses", "name", "notes"}
	for i, want := range wantHeaderPrefix {
		if records[0][i] != want {
			t.Fatalf("expected header %d to be %q, got %#v", i, want, records[0])
		}
	}
	if records[1][0] != "bad@example.com" || records[1][1] != "Bad, Row" || records[1][2] != " keep " {
		t.Fatalf("failed row did not preserve original data: %#v", records[1])
	}
	if records[1][3] != "3" || records[1][7] != "failed" {
		t.Fatalf("unexpected error metadata columns: %#v", records[1])
	}
	if !strings.Contains(records[1][8], "Attio rejected") {
		t.Fatalf("expected write failure context, got %#v", records[1])
	}
}

func TestRecordsImportApplyWritesMultipleValidationFailuresToErrorsCSV(t *testing.T) {
	csvPath := writeImportCSV(t, "validation-errors.csv", "email_addresses,name\nada@example.com,\ncharles@example.com,\n")
	errorsPath := filepath.Join(t.TempDir(), "errors.csv")
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"people"}]}`))
		case "/objects/people/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"email_addresses","type":"email-address","is_writable":true,"is_unique":true},{"api_slug":"name","type":"personal-name","is_writable":true,"is_required":true}]}`))
		case "/objects/people/records":
			t.Fatalf("write endpoint should not be called for invalid planned rows")
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--apply", "--continue-on-error", "--errors", errorsPath)
	if err == nil {
		t.Fatalf("expected apply failure, got nil\n%s", output)
	}

	records := readCSVFile(t, errorsPath)
	if len(records) != 3 {
		t.Fatalf("expected header and two failed rows, got %#v", records)
	}
	if records[1][2] != "2" || records[2][2] != "3" {
		t.Fatalf("expected original row numbers 2 and 3, got %#v", records)
	}
	if !strings.Contains(records[1][7], `missing required attribute "name"`) || !strings.Contains(records[2][7], `missing required attribute "name"`) {
		t.Fatalf("expected validation error context, got %#v", records)
	}
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

func TestRecordsImportApplyWritesRecordThenListEntry(t *testing.T) {
	csvPath := writeImportCSV(t, "record-entry.csv", "email_addresses,name,stage\nada@example.com,Ada Lovelace,Qualified\n")
	recordWritten := false
	entryWritten := false
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"id":{"object_id":"object-people"},"api_slug":"people","singular_noun":"Person","plural_noun":"People"}]}`))
			return
		case "/objects/people/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"email_addresses","is_writable":true,"is_unique":true},{"api_slug":"name","is_writable":true}]}`))
			return
		case "/lists":
			_, _ = w.Write([]byte(`{"data":[{"id":{"list_id":"list-sales"},"api_slug":"sales","name":"Sales Pipeline","parent_object":["people"]}]}`))
			return
		case "/lists/sales/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"stage","is_writable":true}]}`))
			return
		case "/objects/people/records":
			recordWritten = true
			if r.Method != http.MethodPut {
				t.Fatalf("expected record upsert PUT, got %s", r.Method)
			}
			var payload struct {
				Data struct {
					Values map[string]any `json:"values"`
				} `json:"data"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode record request: %v", err)
			}
			if _, hasStage := payload.Data.Values["stage"]; hasStage {
				t.Fatalf("entry-mapped stage should not be sent as a record value: %#v", payload.Data.Values)
			}
			_, _ = w.Write([]byte(`{"data":{"id":{"record_id":"record-ada"},"status":"updated","created":false}}`))
			return
		case "/lists/sales/entries":
			entryWritten = true
			if r.Method != http.MethodPut {
				t.Fatalf("expected default list upsert PUT, got %s", r.Method)
			}
			var payload struct {
				Data struct {
					ParentRecordID string         `json:"parent_record_id"`
					ParentObject   string         `json:"parent_object"`
					EntryValues    map[string]any `json:"entry_values"`
				} `json:"data"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode entry request: %v", err)
			}
			if payload.Data.ParentRecordID != "record-ada" || payload.Data.ParentObject != "people" {
				t.Fatalf("unexpected entry parent payload: %#v", payload.Data)
			}
			if payload.Data.EntryValues["stage"] != "Qualified" {
				t.Fatalf("unexpected entry values: %#v", payload.Data.EntryValues)
			}
			_, _ = w.Write([]byte(`{"data":{"id":{"entry_id":"entry-sales"},"parent_record_id":"record-ada","parent_object":"people","created":true}}`))
			return
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--list", "sales", "--entry-map", "stage=stage", "--apply")
	if err != nil {
		t.Fatalf("expected no error, got %v\n%s", err, output)
	}
	if !recordWritten || !entryWritten {
		t.Fatalf("expected record and entry writes, record=%v entry=%v", recordWritten, entryWritten)
	}
	assertContains(t, output, "List: sales")
	assertContains(t, output, "record-ada")
	assertContains(t, output, "entry-sales")
	assertContains(t, output, "ENTRY STATUS")
}

func TestRecordsImportListParentObjectMismatch(t *testing.T) {
	csvPath := writeImportCSV(t, "entry-mismatch.csv", "email_addresses,name,stage\nada@example.com,Ada Lovelace,Qualified\n")
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"id":{"object_id":"object-people"},"api_slug":"people"},{"id":{"object_id":"object-companies"},"api_slug":"companies"}]}`))
		case "/objects/people/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"email_addresses","is_writable":true,"is_unique":true},{"api_slug":"name","is_writable":true}]}`))
		case "/lists":
			_, _ = w.Write([]byte(`{"data":[{"id":{"list_id":"list-sales"},"api_slug":"sales","name":"Sales Pipeline","parent_object":["companies"]}]}`))
		case "/lists/sales/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"stage","is_writable":true}]}`))
		case "/objects/people/records", "/lists/sales/entries":
			t.Fatalf("write endpoint should not be called after list parent mismatch")
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
	}))

	_, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--list", "sales", "--entry-map", "stage=stage")
	if err == nil {
		t.Fatal("expected list parent object mismatch error")
	}
	assertErrorContains(t, err, `list "sales" accepts parent object "companies"; got import object "people"`)
}

func TestRecordsImportListEntryMissingRecordIDResponse(t *testing.T) {
	csvPath := writeImportCSV(t, "missing-record-id-entry.csv", "email_addresses,name,stage\nada@example.com,Ada Lovelace,Qualified\n")
	entryCalled := false
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"people"}]}`))
		case "/objects/people/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"email_addresses","is_writable":true,"is_unique":true},{"api_slug":"name","is_writable":true}]}`))
		case "/lists":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"sales","parent_object":["people"]}]}`))
		case "/lists/sales/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"stage","is_writable":true}]}`))
		case "/objects/people/records":
			_, _ = w.Write([]byte(`{"data":{"status":"updated","created":false}}`))
		case "/lists/sales/entries":
			entryCalled = true
			t.Fatalf("entry endpoint should not be called without a record ID")
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--list", "sales", "--entry-map", "stage=stage", "--apply")
	if err == nil {
		t.Fatalf("expected apply failure, got nil\n%s", output)
	}
	if entryCalled {
		t.Fatal("entry endpoint was called")
	}
	assertContains(t, output, "list-entry write skipped")
	assertContains(t, output, "failed")
}

func TestRecordsImportListMetadataFallback(t *testing.T) {
	csvPath := writeImportCSV(t, "entry-fallback.csv", "email_addresses,name,stage\nada@example.com,Ada Lovelace,Qualified\n")
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"people"}]}`))
		case "/objects/people/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"email_addresses","is_writable":true,"is_unique":true},{"api_slug":"name","is_writable":true}]}`))
		case "/lists":
			http.Error(w, `{"error":"missing list_configuration:read"}`, http.StatusForbidden)
		case "/objects/people/records", "/lists/sales/entries":
			t.Fatalf("write endpoint should not be called in import dry-run")
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--list", "sales", "--entry-map", "stage=stage")
	if err != nil {
		t.Fatalf("expected metadata fallback plan, got %v\n%s", err, output)
	}
	assertContains(t, output, "List metadata unavailable")
	assertContains(t, output, "valid")
	assertContains(t, output, "ENTRY VALUES")
	assertContains(t, output, "Qualified")
}

func TestRecordsImportListEntryPayloadConstruction(t *testing.T) {
	csvPath := writeImportCSV(t, "entry-payload.csv", "email_addresses,name,pipeline_stage\nada@example.com,Ada Lovelace,Qualified\n")
	attioTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/objects":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"people"}]}`))
			return
		case "/objects/people/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"email_addresses","is_writable":true,"is_unique":true},{"api_slug":"name","is_writable":true}]}`))
			return
		case "/lists":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"sales","parent_object":["people"]}]}`))
			return
		case "/lists/sales/attributes":
			_, _ = w.Write([]byte(`{"data":[{"api_slug":"stage","is_writable":true},{"api_slug":"source","is_writable":true}]}`))
			return
		case "/objects/people/records":
			_, _ = w.Write([]byte(`{"data":{"id":{"record_id":"record-ada"},"status":"updated","created":false}}`))
			return
		case "/lists/sales/entries":
			if r.Method != http.MethodPost {
				t.Fatalf("expected list create POST, got %s", r.Method)
			}
			var payload struct {
				Data struct {
					ParentRecordID string         `json:"parent_record_id"`
					ParentObject   string         `json:"parent_object"`
					EntryValues    map[string]any `json:"entry_values"`
				} `json:"data"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode entry request: %v", err)
			}
			if payload.Data.ParentRecordID != "record-ada" || payload.Data.ParentObject != "people" {
				t.Fatalf("unexpected entry parent: %#v", payload.Data)
			}
			if payload.Data.EntryValues["stage"] != "Qualified" || payload.Data.EntryValues["source"] != "csv" {
				t.Fatalf("unexpected entry values: %#v", payload.Data.EntryValues)
			}
			if _, exists := payload.Data.EntryValues["pipeline_stage"]; exists {
				t.Fatalf("expected mapped entry attribute, got raw CSV column: %#v", payload.Data.EntryValues)
			}
			_, _ = w.Write([]byte(`{"data":{"id":{"entry_id":"entry-sales"},"parent_record_id":"record-ada","parent_object":"people","status":"created","created":true}}`))
			return
		default:
			t.Fatalf("unexpected path %s", r.URL.String())
		}
	}))

	output, err := executeTestCommand(t, newRecordsCommand(), "import", "people", csvPath, "--list", "sales", "--list-mode", "create", "--entry-map", "pipeline_stage=stage", "--entry-set", "source=csv", "--apply")
	if err != nil {
		t.Fatalf("expected no error, got %v\n%s", err, output)
	}
	assertContains(t, output, "List mode: create")
	assertContains(t, output, "entry-sales")
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
		"--continue-on-error",
		"--errors",
		"--set",
		"--set-json",
		"--list",
		"--list-mode",
		"--entry-map",
		"--entry-set",
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

func decodeImportRequestEmail(t *testing.T, r *http.Request) string {
	t.Helper()
	var payload struct {
		Data struct {
			Values map[string]any `json:"values"`
		} `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	email, _ := payload.Data.Values["email_addresses"].(string)
	return email
}

func readCSVFile(t *testing.T, path string) [][]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open CSV %q: %v", path, err)
	}
	defer file.Close()

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("read CSV %q: %v", path, err)
	}
	return records
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
