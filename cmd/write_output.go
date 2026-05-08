package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"atcli/internal/attio"
)

const (
	outputFormatTable = "table"
	outputFormatJSON  = "json"
)

type recordWriteOutput struct {
	DryRun         bool
	Object         recordWriteObject
	MatchAttribute string
	MatchDefaulted bool
	Payload        map[string]any
	Record         *attio.Record
	Outcome        string
	Created        *bool
}

type recordWriteObject struct {
	Identifier   string
	ObjectID     string
	SingularNoun string
	PluralNoun   string
}

func recordCreatePayload(values map[string]any) map[string]any {
	return map[string]any{
		"data": map[string]any{
			"values": values,
		},
	}
}

func recordAssertPayload(values map[string]any) map[string]any {
	return recordCreatePayload(values)
}

func printRecordWriteOutput(out io.Writer, format string, result recordWriteOutput) error {
	switch format {
	case outputFormatTable:
		return printRecordWriteTable(out, result)
	case outputFormatJSON:
		return printRecordWriteJSON(out, result)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func printRecordWriteTable(out io.Writer, result recordWriteOutput) error {
	if result.DryRun {
		if _, err := fmt.Fprintln(out, "DRY RUN: no write endpoint called"); err != nil {
			return err
		}
		if result.MatchAttribute != "" {
			if _, err := fmt.Fprintf(out, "Matching attribute: %s\n", result.MatchAttribute); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(out, "Payload:"); err != nil {
			return err
		}
		return writeIndentedJSON(out, result.Payload)
	}

	recordID, objectID, createdAt, webURL := recordFields(result)
	if objectID == "" {
		objectID = result.Object.ObjectID
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if result.MatchAttribute == "" && result.Outcome == "" && result.Created == nil {
		if _, err := fmt.Fprintln(w, "OBJECT\tOBJECT ID\tRECORD ID\tSINGULAR\tPLURAL\tCREATED AT\tWEB URL"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			result.Object.Identifier,
			objectID,
			recordID,
			result.Object.SingularNoun,
			result.Object.PluralNoun,
			createdAt,
			webURL,
		); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(w, "OBJECT\tOBJECT ID\tRECORD ID\tMATCH\tOUTCOME\tSINGULAR\tPLURAL\tCREATED AT\tWEB URL"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			result.Object.Identifier,
			objectID,
			recordID,
			result.MatchAttribute,
			recordOutcome(result),
			result.Object.SingularNoun,
			result.Object.PluralNoun,
			createdAt,
			webURL,
		); err != nil {
			return err
		}
	}
	return w.Flush()
}

func printRecordWriteJSON(out io.Writer, result recordWriteOutput) error {
	recordID, objectID, createdAt, webURL := recordFields(result)
	if objectID == "" {
		objectID = result.Object.ObjectID
	}

	output := recordWriteJSONOutput{
		DryRun:              result.DryRun,
		WriteEndpointCalled: !result.DryRun,
		MatchAttribute:      result.MatchAttribute,
		MatchDefaulted:      result.MatchDefaulted,
		Outcome:             result.Outcome,
		Created:             result.Created,
		Object: recordWriteObjectJSON{
			Identifier:   result.Object.Identifier,
			ObjectID:     objectID,
			SingularNoun: result.Object.SingularNoun,
			PluralNoun:   result.Object.PluralNoun,
		},
	}
	if result.DryRun {
		output.Payload = result.Payload
	} else {
		output.Record = &recordWriteRecordJSON{
			RecordID:  recordID,
			CreatedAt: createdAt,
			WebURL:    webURL,
		}
	}

	return writeIndentedJSON(out, output)
}

type recordWriteJSONOutput struct {
	DryRun              bool                   `json:"dry_run"`
	WriteEndpointCalled bool                   `json:"write_endpoint_called"`
	MatchAttribute      string                 `json:"match_attribute,omitempty"`
	MatchDefaulted      bool                   `json:"match_defaulted,omitempty"`
	Outcome             string                 `json:"outcome,omitempty"`
	Created             *bool                  `json:"created,omitempty"`
	Object              recordWriteObjectJSON  `json:"object"`
	Payload             map[string]any         `json:"payload,omitempty"`
	Record              *recordWriteRecordJSON `json:"record,omitempty"`
}

type recordWriteObjectJSON struct {
	Identifier   string `json:"identifier"`
	ObjectID     string `json:"object_id,omitempty"`
	SingularNoun string `json:"singular_noun,omitempty"`
	PluralNoun   string `json:"plural_noun,omitempty"`
}

type recordWriteRecordJSON struct {
	RecordID  string `json:"record_id,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	WebURL    string `json:"web_url,omitempty"`
}

func recordFields(result recordWriteOutput) (recordID, objectID, createdAt, webURL string) {
	if result.Record == nil {
		return "", "", "", ""
	}
	return result.Record.ID.RecordID, result.Record.ID.ObjectID, result.Record.CreatedAt, result.Record.WebURL
}

func recordOutcome(result recordWriteOutput) string {
	if result.Outcome != "" {
		return result.Outcome
	}
	if result.Created == nil {
		return ""
	}
	if *result.Created {
		return "created"
	}
	return "updated"
}

func writeIndentedJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
