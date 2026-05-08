package importplan

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"atcli/internal/attio"
)

func TestPrepareRowStringPassthroughTypes(t *testing.T) {
	for _, attributeType := range []string{
		"text",
		"personal-name",
		"email-address",
		"domain",
		"phone-number",
		"select",
		"status",
		"record-reference",
		"relationship",
		"unknown-complex-type",
	} {
		t.Run(attributeType, func(t *testing.T) {
			row := CSVRow{Number: 2, Values: map[string]string{"value": "  Ada Lovelace  "}}
			mapping := &MappingPlan{Columns: []ColumnMapping{{CSVColumn: "value", Attribute: "value"}}}

			prepared, err := PrepareRow(row, mapping, ValuePreparationOptions{
				Attributes: []attio.Attribute{{APISlug: "value", Type: attributeType}},
			})
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got := prepared.Values["value"]; got != "Ada Lovelace" {
				t.Fatalf("expected string passthrough, got %#v", got)
			}
		})
	}
}

func TestPrepareRowParsesSupportedTypes(t *testing.T) {
	tests := []struct {
		name          string
		attributeType string
		raw           string
		want          any
	}{
		{name: "number", attributeType: "number", raw: "42.5", want: 42.5},
		{name: "checkbox true", attributeType: "checkbox", raw: "yes", want: true},
		{name: "checkbox false", attributeType: "checkbox", raw: "0", want: false},
		{name: "date", attributeType: "date", raw: "2026-05-07", want: "2026-05-07"},
		{name: "timestamp", attributeType: "timestamp", raw: "2026-05-07T12:30:00-04:00", want: "2026-05-07T16:30:00Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := CSVRow{Number: 2, Values: map[string]string{"value": tt.raw}}
			mapping := &MappingPlan{Columns: []ColumnMapping{{CSVColumn: "value", Attribute: "value"}}}

			prepared, err := PrepareRow(row, mapping, ValuePreparationOptions{
				Attributes: []attio.Attribute{{APISlug: "value", Type: tt.attributeType}},
			})
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if !reflect.DeepEqual(prepared.Values["value"], tt.want) {
				t.Fatalf("expected %#v, got %#v", tt.want, prepared.Values["value"])
			}
		})
	}
}

func TestPrepareRowInvalidValues(t *testing.T) {
	tests := []struct {
		name          string
		attributeType string
		raw           string
		wantError     string
	}{
		{name: "number", attributeType: "number", raw: "abc", wantError: "expected number"},
		{name: "checkbox", attributeType: "checkbox", raw: "maybe", wantError: "expected checkbox value"},
		{name: "date", attributeType: "date", raw: "05/07/2026", wantError: "expected ISO date"},
		{name: "timestamp", attributeType: "timestamp", raw: "2026-05-07 12:30", wantError: "expected RFC3339 timestamp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := CSVRow{Number: 4, Values: map[string]string{"value": tt.raw}}
			mapping := &MappingPlan{Columns: []ColumnMapping{{CSVColumn: "value", Attribute: "value"}}}

			_, err := PrepareRow(row, mapping, ValuePreparationOptions{
				Attributes: []attio.Attribute{{APISlug: "value", Type: tt.attributeType}},
			})
			if err == nil {
				t.Fatal("expected invalid value error")
			}
			if !strings.Contains(err.Error(), "row 4 column \"value\"") || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestPrepareRowSplitsMultiValues(t *testing.T) {
	row := CSVRow{Number: 2, Values: map[string]string{"domains": "example.com; example.org; "}}
	mapping := &MappingPlan{Columns: []ColumnMapping{{CSVColumn: "domains", Attribute: "domains"}}}

	prepared, err := PrepareRow(row, mapping, ValuePreparationOptions{
		Attributes:     []attio.Attribute{{APISlug: "domains", Type: "domain", IsMultiselect: true}},
		MultiSeparator: ";",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := []any{"example.com", "example.org"}
	if !reflect.DeepEqual(prepared.Values["domains"], want) {
		t.Fatalf("expected %#v, got %#v", want, prepared.Values["domains"])
	}
}

func TestPrepareRowOmitsEmptyCells(t *testing.T) {
	row := CSVRow{Number: 2, Values: map[string]string{"name": "Ada", "notes": "  "}}
	mapping := &MappingPlan{Columns: []ColumnMapping{
		{CSVColumn: "name", Attribute: "name"},
		{CSVColumn: "notes", Attribute: "description"},
	}}

	prepared, err := PrepareRow(row, mapping, ValuePreparationOptions{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if _, ok := prepared.Values["description"]; ok {
		t.Fatalf("empty cell should not produce a value: %#v", prepared.Values)
	}
	if len(prepared.SkippedEmpty) != 1 || prepared.SkippedEmpty[0].Attribute != "description" {
		t.Fatalf("expected skipped empty description, got %#v", prepared.SkippedEmpty)
	}
}

func TestPrepareRowPreservesJSONStaticValues(t *testing.T) {
	var rawJSON any
	if err := json.Unmarshal([]byte(`{"source":"agent","rank":2}`), &rawJSON); err != nil {
		t.Fatalf("decode test JSON: %v", err)
	}

	row := CSVRow{Number: 2, Values: map[string]string{"name": "Ada"}}
	mapping := &MappingPlan{
		Columns:      []ColumnMapping{{CSVColumn: "name", Attribute: "name"}},
		StaticValues: map[string]any{"metadata": rawJSON},
	}

	prepared, err := PrepareRow(row, mapping, ValuePreparationOptions{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !reflect.DeepEqual(prepared.Values["metadata"], rawJSON) {
		t.Fatalf("expected JSON static value to pass through, got %#v", prepared.Values["metadata"])
	}
}
