package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"atcli/internal/attio"
)

func TestPrintRecordWriteTableOutput(t *testing.T) {
	var out bytes.Buffer
	err := printRecordWriteOutput(&out, outputFormatTable, recordWriteOutput{
		Object: recordWriteObject{
			Identifier:   "people",
			SingularNoun: "Person",
			PluralNoun:   "People",
		},
		Record: &attio.Record{
			ID:        attio.RecordID{ObjectID: "object-123", RecordID: "record-123"},
			CreatedAt: "2026-05-07T12:00:00Z",
			WebURL:    "https://app.attio.com/acme/person/record-123",
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	output := out.String()
	assertContains(t, output, "OBJECT")
	assertContains(t, output, "people")
	assertContains(t, output, "object-123")
	assertContains(t, output, "record-123")
	assertContains(t, output, "Person")
	assertContains(t, output, "People")
}

func TestPrintRecordWriteJSONOutput(t *testing.T) {
	var out bytes.Buffer
	err := printRecordWriteOutput(&out, outputFormatJSON, recordWriteOutput{
		Object: recordWriteObject{
			Identifier:   "people",
			ObjectID:     "object-from-metadata",
			SingularNoun: "Person",
			PluralNoun:   "People",
		},
		Record: &attio.Record{
			ID:     attio.RecordID{ObjectID: "object-from-record", RecordID: "record-123"},
			WebURL: "https://app.attio.com/acme/person/record-123",
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var got struct {
		DryRun              bool `json:"dry_run"`
		WriteEndpointCalled bool `json:"write_endpoint_called"`
		Object              struct {
			Identifier   string `json:"identifier"`
			ObjectID     string `json:"object_id"`
			SingularNoun string `json:"singular_noun"`
			PluralNoun   string `json:"plural_noun"`
		} `json:"object"`
		Record struct {
			RecordID string `json:"record_id"`
			WebURL   string `json:"web_url"`
		} `json:"record"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode JSON output: %v\n%s", err, out.String())
	}
	if got.DryRun || !got.WriteEndpointCalled {
		t.Fatalf("unexpected dry-run markers: %#v", got)
	}
	if got.Object.Identifier != "people" || got.Object.ObjectID != "object-from-record" {
		t.Fatalf("unexpected object JSON: %#v", got.Object)
	}
	if got.Object.SingularNoun != "Person" || got.Object.PluralNoun != "People" {
		t.Fatalf("expected object nouns, got %#v", got.Object)
	}
	if got.Record.RecordID != "record-123" || got.Record.WebURL == "" {
		t.Fatalf("unexpected record JSON: %#v", got.Record)
	}
}

func TestPrintRecordWriteDryRunOutput(t *testing.T) {
	values := map[string]any{
		"name": "Ada Lovelace",
		"tags": []any{"vip", "lead"},
	}
	payload := recordCreatePayload(values)

	var out bytes.Buffer
	err := printRecordWriteOutput(&out, outputFormatTable, recordWriteOutput{
		DryRun:  true,
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	output := out.String()
	assertContains(t, output, "DRY RUN")
	assertContains(t, output, "no write endpoint called")
	var got map[string]any
	if err := json.Unmarshal([]byte(output[strings.Index(output, "{"):]), &got); err != nil {
		t.Fatalf("expected payload JSON in output, got %v\n%s", err, output)
	}
	assertJSONEqual(t, payload, got)
}

func TestPrintRecordWriteOutputHandlesMissingOptionalMetadata(t *testing.T) {
	var out bytes.Buffer
	err := printRecordWriteOutput(&out, outputFormatJSON, recordWriteOutput{
		Object: recordWriteObject{
			Identifier: "custom_widgets",
		},
		Record: &attio.Record{
			ID: attio.RecordID{RecordID: "record-123"},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}
	object := got["object"].(map[string]any)
	if object["identifier"] != "custom_widgets" {
		t.Fatalf("expected object identifier, got %#v", object)
	}
	if _, ok := object["singular_noun"]; ok {
		t.Fatalf("did not expect missing singular noun to be emitted: %#v", object)
	}
	record := got["record"].(map[string]any)
	if record["record_id"] != "record-123" {
		t.Fatalf("expected record id, got %#v", record)
	}
}

func assertJSONEqual(t *testing.T, want, got any) {
	t.Helper()
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal expected JSON: %v", err)
	}
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal actual JSON: %v", err)
	}
	if !bytes.Equal(wantJSON, gotJSON) {
		t.Fatalf("expected JSON %s, got %s", wantJSON, gotJSON)
	}
}
