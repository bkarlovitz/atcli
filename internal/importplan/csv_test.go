package importplan

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCSVValidInput(t *testing.T) {
	document, err := LoadCSV(fixturePath("valid.csv"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(document.Headers) != 3 {
		t.Fatalf("expected 3 headers, got %#v", document.Headers)
	}
	if document.Headers[0] != "name" || document.Headers[1] != "domains" || document.Headers[2] != "notes" {
		t.Fatalf("unexpected headers: %#v", document.Headers)
	}
	if len(document.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(document.Rows))
	}
	if document.Rows[0].Number != 2 {
		t.Fatalf("expected first data row number 2, got %d", document.Rows[0].Number)
	}
	if got := document.Rows[0].Values["domains"]; got != "example.com" {
		t.Fatalf("expected domains value, got %q", got)
	}
}

func TestLoadCSVValidationErrors(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantError string
	}{
		{
			name:      "duplicate headers",
			path:      fixturePath("duplicate_headers.csv"),
			wantError: `header "name" is duplicated`,
		},
		{
			name:      "empty header",
			path:      fixturePath("empty_header.csv"),
			wantError: "header at column 2 is empty",
		},
		{
			name:      "uneven rows",
			path:      fixturePath("uneven_rows.csv"),
			wantError: "row 2 has 2 fields; expected 3",
		},
		{
			name:      "empty file",
			path:      fixturePath("empty.csv"),
			wantError: "is empty",
		},
		{
			name:      "unreadable path",
			path:      fixturePath("missing.csv"),
			wantError: "read CSV",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadCSV(tt.path)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error to contain %q, got %v", tt.wantError, err)
			}
		})
	}
}

func fixturePath(name string) string {
	return filepath.Join("testdata", name)
}
