package importplan

import (
	"strings"
	"testing"

	"atcli/internal/attio"
)

func TestBuildImportPlanValidatesRows(t *testing.T) {
	document := &CSVDocument{
		Headers: []string{"email_addresses", "name"},
		Rows: []CSVRow{
			{Number: 2, Values: map[string]string{"email_addresses": "ada@example.com", "name": "Ada"}},
			{Number: 3, Values: map[string]string{"email_addresses": "", "name": ""}},
		},
	}
	mapping, err := BuildMappingPlan(document.Headers, MappingOptions{})
	if err != nil {
		t.Fatalf("build mapping: %v", err)
	}

	plan, err := BuildImportPlan(document, mapping, ImportPlanOptions{
		ObjectIdentifier: "people",
		Mode:             ModeUpsert,
		MatchAttribute:   "email_addresses",
		Attributes: []attio.Attribute{
			{APISlug: "email_addresses", Type: "email-address", IsWritable: true, IsUnique: true},
			{APISlug: "name", Type: "personal-name", IsWritable: true, IsRequired: true},
		},
		MetadataAvailable: true,
	})
	if err != nil {
		t.Fatalf("expected no plan error, got %v", err)
	}
	if len(plan.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %#v", plan.Rows)
	}
	if !plan.Rows[0].Valid {
		t.Fatalf("expected first row valid, got %#v", plan.Rows[0])
	}
	if plan.Rows[1].Valid {
		t.Fatalf("expected second row invalid, got %#v", plan.Rows[1])
	}
	assertErrorListContains(t, plan.Rows[1].Errors, `missing required attribute "name"`)
	assertErrorListContains(t, plan.Rows[1].Errors, `matching attribute "email_addresses" must have a value`)
}

func TestBuildImportPlanMetadataPlanErrors(t *testing.T) {
	tests := []struct {
		name      string
		headers   []string
		attrs     []attio.Attribute
		match     string
		wantError string
	}{
		{
			name:      "unknown mapped column",
			headers:   []string{"nickname"},
			attrs:     []attio.Attribute{{APISlug: "name", IsWritable: true}},
			match:     "name",
			wantError: `unknown attribute "nickname"`,
		},
		{
			name:      "non writable mapped column",
			headers:   []string{"name"},
			attrs:     []attio.Attribute{{APISlug: "name", IsWritable: false, IsUnique: true}},
			match:     "name",
			wantError: `attribute "name" is not writable`,
		},
		{
			name:      "missing match metadata",
			headers:   []string{"name"},
			attrs:     []attio.Attribute{{APISlug: "name", IsWritable: true}},
			match:     "external_id",
			wantError: `matching attribute "external_id" was not found`,
		},
		{
			name:      "non unique match",
			headers:   []string{"name"},
			attrs:     []attio.Attribute{{APISlug: "name", IsWritable: true, IsUnique: false}},
			match:     "name",
			wantError: `matching attribute "name" is not unique`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document := &CSVDocument{
				Headers: tt.headers,
				Rows:    []CSVRow{{Number: 2, Values: map[string]string{tt.headers[0]: "value"}}},
			}
			mapping, err := BuildMappingPlan(document.Headers, MappingOptions{})
			if err != nil {
				t.Fatalf("build mapping: %v", err)
			}
			_, err = BuildImportPlan(document, mapping, ImportPlanOptions{
				ObjectIdentifier:  "people",
				Mode:              ModeUpsert,
				MatchAttribute:    tt.match,
				Attributes:        tt.attrs,
				MetadataAvailable: true,
			})
			if err == nil {
				t.Fatal("expected plan error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error to contain %q, got %v", tt.wantError, err)
			}
		})
	}
}

func assertErrorListContains(t *testing.T, errors []string, want string) {
	t.Helper()
	for _, candidate := range errors {
		if strings.Contains(candidate, want) {
			return
		}
	}
	t.Fatalf("expected errors to contain %q, got %#v", want, errors)
}
