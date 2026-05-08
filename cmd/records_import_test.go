package cmd

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		"--set",
		"--set-json",
		"--mode",
		"--match",
		"--multi-sep",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected help to contain %q, got:\n%s", expected, output)
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
