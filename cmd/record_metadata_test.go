package cmd

import (
	"testing"

	"atcli/internal/attio"
)

func TestValidateRecordMatchAttributeSuccess(t *testing.T) {
	err := validateRecordMatchAttribute(
		map[string]any{"email_addresses": []any{"ada@example.com"}},
		[]attio.Attribute{{APISlug: "email_addresses", IsUnique: true}},
		"email_addresses",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateRecordMatchAttributeByIDWithMissingSlug(t *testing.T) {
	err := validateRecordMatchAttribute(
		map[string]any{"attr-external-id": "ext-123"},
		[]attio.Attribute{{ID: attio.AttributeID{AttributeID: "attr-external-id"}, IsUnique: true}},
		"attr-external-id",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateRecordMatchAttributeAcceptsSlugValueWhenMatchedByID(t *testing.T) {
	err := validateRecordMatchAttribute(
		map[string]any{"external_id": "ext-123"},
		[]attio.Attribute{{
			ID:       attio.AttributeID{AttributeID: "attr-external-id"},
			APISlug:  "external_id",
			IsUnique: true,
		}},
		"attr-external-id",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateRecordMatchAttributeNonUniqueFailure(t *testing.T) {
	err := validateRecordMatchAttribute(
		map[string]any{"domains": []any{"example.com"}},
		[]attio.Attribute{{APISlug: "domains", IsUnique: false}},
		"domains",
	)
	if err == nil {
		t.Fatal("expected non-unique match error")
	}
	assertErrorContains(t, err, `matching attribute "domains" is not unique`)
}

func TestValidateRecordMatchAttributeMissingValueFailure(t *testing.T) {
	err := validateRecordMatchAttribute(
		map[string]any{"name": "Example Co"},
		[]attio.Attribute{{APISlug: "domains", IsUnique: true}},
		"domains",
	)
	if err == nil {
		t.Fatal("expected missing match value error")
	}
	assertErrorContains(t, err, `matching attribute "domains" must have a value`)
}

func TestValidateRecordMatchAttributeMissingMetadataFields(t *testing.T) {
	err := validateRecordMatchAttribute(
		map[string]any{"domains": []any{"example.com"}},
		[]attio.Attribute{{APISlug: "domains"}},
		"domains",
	)
	if err == nil {
		t.Fatal("expected uniqueness error when metadata does not report uniqueness")
	}
	assertErrorContains(t, err, `matching attribute "domains" is not unique`)
}

func TestValidateRecordCreateValuesAcceptsAttributeID(t *testing.T) {
	err := validateRecordCreateValues(
		map[string]any{"attr-name": "Ada Lovelace"},
		[]attio.Attribute{{
			ID:         attio.AttributeID{AttributeID: "attr-name"},
			APISlug:    "name",
			IsWritable: true,
		}},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
