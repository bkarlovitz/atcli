package importplan

import (
	"strings"
	"testing"
)

func TestBuildMappingPlanDefaultMapping(t *testing.T) {
	plan, err := BuildMappingPlan([]string{"name", "domains"}, MappingOptions{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertColumnMappings(t, plan.Columns, []ColumnMapping{
		{CSVColumn: "name", Attribute: "name"},
		{CSVColumn: "domains", Attribute: "domains"},
	})
}

func TestBuildMappingPlanExplicitMapping(t *testing.T) {
	rules, err := ParseMappingRules([]string{"Company Name=name", "Domain=domains"})
	if err != nil {
		t.Fatalf("expected no parse error, got %v", err)
	}

	plan, err := BuildMappingPlan([]string{"Company Name", "Domain"}, MappingOptions{Rules: rules})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertColumnMappings(t, plan.Columns, []ColumnMapping{
		{CSVColumn: "Company Name", Attribute: "name"},
		{CSVColumn: "Domain", Attribute: "domains"},
	})
}

func TestBuildMappingPlanIgnoresColumns(t *testing.T) {
	ignored, err := NormalizeIgnoredColumns([]string{" notes "})
	if err != nil {
		t.Fatalf("expected no ignore parse error, got %v", err)
	}

	plan, err := BuildMappingPlan([]string{"name", "domains", "notes"}, MappingOptions{Ignore: ignored})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertColumnMappings(t, plan.Columns, []ColumnMapping{
		{CSVColumn: "name", Attribute: "name"},
		{CSVColumn: "domains", Attribute: "domains"},
	})
	if len(plan.Ignored) != 1 || plan.Ignored[0] != "notes" {
		t.Fatalf("unexpected ignored columns: %#v", plan.Ignored)
	}
}

func TestBuildMappingPlanAllowsStaticValues(t *testing.T) {
	plan, err := BuildMappingPlan([]string{"name"}, MappingOptions{
		StaticValues: map[string]any{"domains": []any{"example.com"}},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertColumnMappings(t, plan.Columns, []ColumnMapping{
		{CSVColumn: "name", Attribute: "name"},
	})
	if got := plan.StaticValues["domains"]; got == nil {
		t.Fatalf("expected static domains value, got %#v", plan.StaticValues)
	}
}

func TestBuildMappingPlanConflictErrors(t *testing.T) {
	tests := []struct {
		name      string
		headers   []string
		opts      MappingOptions
		wantError string
	}{
		{
			name:    "mapping missing CSV column",
			headers: []string{"name"},
			opts: MappingOptions{
				Rules: []MappingRule{{CSVColumn: "Domain", Attribute: "domains"}},
			},
			wantError: `mapped CSV column "Domain" was not found`,
		},
		{
			name:    "duplicate target attributes",
			headers: []string{"name", "Company Name"},
			opts: MappingOptions{
				Rules: []MappingRule{{CSVColumn: "Company Name", Attribute: "name"}},
			},
			wantError: `attribute "name" is targeted by both`,
		},
		{
			name:    "ignored column also mapped",
			headers: []string{"name", "notes"},
			opts: MappingOptions{
				Rules:  []MappingRule{{CSVColumn: "notes", Attribute: "description"}},
				Ignore: []string{"notes"},
			},
			wantError: `CSV column "notes" cannot be both mapped and ignored`,
		},
		{
			name:    "static value conflicts with mapped attribute",
			headers: []string{"name"},
			opts: MappingOptions{
				StaticValues: map[string]any{"name": "Static Name"},
			},
			wantError: `attribute "name" cannot be both mapped from CSV and set statically`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildMappingPlan(tt.headers, tt.opts)
			if err == nil {
				t.Fatal("expected conflict error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error to contain %q, got %v", tt.wantError, err)
			}
		})
	}
}

func assertColumnMappings(t *testing.T, got, want []ColumnMapping) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d mappings, got %#v", len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mapping %d: expected %#v, got %#v", i, want[i], got[i])
		}
	}
}
