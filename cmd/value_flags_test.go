package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseRecordValueFlagsValidStrings(t *testing.T) {
	values, err := parseRecordValueFlags([]string{"name=Ada Lovelace", "empty="}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if values["name"] != "Ada Lovelace" {
		t.Fatalf("expected string value, got %#v", values["name"])
	}
	if values["empty"] != "" {
		t.Fatalf("expected empty string value, got %#v", values["empty"])
	}
}

func TestParseRecordValueFlagsValidJSONValues(t *testing.T) {
	values, err := parseRecordValueFlags(nil, []string{
		`tags=["vip","lead"]`,
		`count=42`,
		`profile={"role":"admin"}`,
		`active=true`,
		`deleted_at=null`,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	tags, ok := values["tags"].([]any)
	if !ok || len(tags) != 2 || tags[0] != "vip" || tags[1] != "lead" {
		t.Fatalf("unexpected array value: %#v", values["tags"])
	}
	if values["count"] != json.Number("42") {
		t.Fatalf("expected JSON number, got %#v", values["count"])
	}
	profile, ok := values["profile"].(map[string]any)
	if !ok || profile["role"] != "admin" {
		t.Fatalf("unexpected object value: %#v", values["profile"])
	}
	if values["active"] != true {
		t.Fatalf("expected boolean value, got %#v", values["active"])
	}
	if values["deleted_at"] != nil {
		t.Fatalf("expected null value, got %#v", values["deleted_at"])
	}
}

func TestParseRecordValueFlagsRejectsDuplicateNames(t *testing.T) {
	_, err := parseRecordValueFlags([]string{"name=Ada"}, []string{`name="Grace"`})
	if err == nil {
		t.Fatal("expected duplicate error")
	}
	assertErrorContains(t, err, `duplicate attribute "name"`)
}

func TestParseRecordValueFlagsRejectsEmptyNames(t *testing.T) {
	for _, flags := range [][]string{{"=Ada"}, {"  =Ada"}} {
		_, err := parseRecordValueFlags(flags, nil)
		if err == nil {
			t.Fatalf("expected empty name error for %#v", flags)
		}
		assertErrorContains(t, err, "attribute name cannot be empty")
	}
}

func TestParseRecordValueFlagsRejectsMissingEquals(t *testing.T) {
	_, err := parseRecordValueFlags([]string{"name"}, nil)
	if err == nil {
		t.Fatal("expected missing equals error")
	}
	assertErrorContains(t, err, "expected attr=value")
}

func TestParseRecordValueFlagsRejectsMalformedJSON(t *testing.T) {
	_, err := parseRecordValueFlags(nil, []string{`tags=["vip"`})
	if err == nil {
		t.Fatal("expected malformed JSON error")
	}
	assertErrorContains(t, err, "invalid JSON")
}

func assertErrorContains(t *testing.T, err error, expected string) {
	t.Helper()
	if !strings.Contains(err.Error(), expected) {
		t.Fatalf("expected error to contain %q, got %v", expected, err)
	}
}
