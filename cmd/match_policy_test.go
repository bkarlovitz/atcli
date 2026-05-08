package cmd

import "testing"

func TestResolveRecordMatchAttributeDefaults(t *testing.T) {
	tests := []struct {
		object    string
		wantMatch string
	}{
		{object: "companies", wantMatch: "domains"},
		{object: "people", wantMatch: "email_addresses"},
		{object: "users", wantMatch: "primary_email_address"},
		{object: "workspaces", wantMatch: "workspace_id"},
	}

	for _, tt := range tests {
		t.Run(tt.object, func(t *testing.T) {
			match, usedDefault, err := resolveRecordMatchAttribute(tt.object, "")
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if match != tt.wantMatch {
				t.Fatalf("expected match %q, got %q", tt.wantMatch, match)
			}
			if !usedDefault {
				t.Fatal("expected safe default to be used")
			}
		})
	}
}

func TestResolveRecordMatchAttributeExplicitOverride(t *testing.T) {
	match, usedDefault, err := resolveRecordMatchAttribute("people", "external_id")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if match != "external_id" {
		t.Fatalf("expected explicit match, got %q", match)
	}
	if usedDefault {
		t.Fatal("did not expect safe default when explicit match is set")
	}
}

func TestResolveRecordMatchAttributeRequiresExplicitMatch(t *testing.T) {
	for _, object := range []string{"deals", "custom_widgets", "unknown"} {
		t.Run(object, func(t *testing.T) {
			_, _, err := resolveRecordMatchAttribute(object, "")
			if err == nil {
				t.Fatal("expected match requirement error")
			}
			assertErrorContains(t, err, "--match is required")
			assertErrorContains(t, err, object)
		})
	}
}

func TestResolveRecordMatchAttributeDoesNotInferNouns(t *testing.T) {
	for _, object := range []string{"company", "person", "user", "workspace", "Companies", "People"} {
		t.Run(object, func(t *testing.T) {
			_, _, err := resolveRecordMatchAttribute(object, "")
			if err == nil {
				t.Fatal("expected match requirement error")
			}
			assertErrorContains(t, err, "--match is required")
		})
	}
}
