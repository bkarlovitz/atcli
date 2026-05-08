package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"atcli/internal/attio"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(newEntriesCommand())
}

type entriesWriteOptions struct {
	parentObject   string
	parentRecordID string
	setValues      []string
	setJSONValues  []string
	dryRun         bool
	outputFormat   string
}

type entryWriteList struct {
	Identifier   string
	ListID       string
	Name         string
	ParentObject []string
}

type entryWriteOutput struct {
	DryRun         bool
	List           entryWriteList
	ParentObject   recordWriteObject
	ParentRecordID string
	Payload        map[string]any
	Entry          *attio.ListEntry
	Outcome        string
	Created        *bool
}

type entryWriteMetadata struct {
	List                entryWriteList
	ParentObject        recordWriteObject
	ParentObjectAPISlug string
	Attributes          []attio.Attribute
}

func newEntriesCommand() *cobra.Command {
	entriesCmd := &cobra.Command{
		Use:   "entries",
		Short: "Add and manage Attio list entries",
	}
	entriesCmd.AddCommand(newEntriesAddCommand())
	return entriesCmd
}

func newEntriesAddCommand() *cobra.Command {
	opts := entriesWriteOptions{
		outputFormat: outputFormatTable,
	}

	addCmd := &cobra.Command{
		Use:   "add <list>",
		Short: "Add one record to an Attio list",
		Long: strings.TrimSpace(`
Add one existing record to an Attio list as a list entry.

The <list> argument is an Attio list slug or ID. The --parent-object value is
an Attio object slug or ID, and --parent-record-id is the record ID to add to
the list. List entries point at records and may have their own values.
`),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("accepts 1 arg, received %d", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEntriesAdd(cmd, args[0], opts)
		},
	}
	addCmd.Flags().StringVar(&opts.parentObject, "parent-object", "", "parent object slug or ID for the record being added")
	addCmd.Flags().StringVar(&opts.parentRecordID, "parent-record-id", "", "parent record ID to add to the list")
	addCmd.Flags().StringArrayVar(&opts.setValues, "set", nil, "set a list-entry attribute as a string value (attr=value)")
	addCmd.Flags().StringArrayVar(&opts.setJSONValues, "set-json", nil, "set a list-entry attribute as a JSON value (attr=json)")
	addCmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "print the write payload without calling the write endpoint")
	addCmd.Flags().StringVar(&opts.outputFormat, "output", outputFormatTable, "output format: table or json")

	return addCmd
}

func runEntriesAdd(cmd *cobra.Command, list string, opts entriesWriteOptions) error {
	if err := validateEntryWriteOptions(opts); err != nil {
		return err
	}

	values, err := parseRecordValueFlags(opts.setValues, opts.setJSONValues)
	if err != nil {
		return err
	}

	result := entryWriteOutput{
		DryRun: opts.dryRun,
		List: entryWriteList{
			Identifier: list,
		},
		ParentObject: recordWriteObject{
			Identifier: opts.parentObject,
		},
		ParentRecordID: opts.parentRecordID,
		Payload:        entryWritePayload(opts.parentObject, opts.parentRecordID, values),
	}

	if opts.dryRun {
		return printEntryWriteOutput(cmd.OutOrStdout(), opts.outputFormat, result)
	}

	client, err := loadAttioClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
	defer cancel()

	metadata, err := loadEntryWriteMetadata(ctx, client, list, opts.parentObject)
	if err != nil {
		if isMetadataPermissionError(err) {
			if _, warnErr := fmt.Fprintln(cmd.ErrOrStderr(), "Metadata unavailable; token needs list_configuration:read and object_configuration:read. Local validation, noun display, and parent compatibility validation skipped."); warnErr != nil {
				return warnErr
			}
		} else {
			return fmt.Errorf("could not fetch list entry metadata: %w", err)
		}
	} else {
		result.List = metadata.List
		result.ParentObject = metadata.ParentObject
		if err := validateEntryWriteMetadata(metadata, opts.parentObject, values); err != nil {
			return err
		}
	}

	entryResult, err := client.CreateListEntry(ctx, list, attio.ListEntryWrite{
		ParentRecordID: opts.parentRecordID,
		ParentObject:   opts.parentObject,
		EntryValues:    values,
	})
	if err != nil {
		return classifyEntryWriteError("add list entry", err)
	}
	entry := entryResult.Entry
	result.Entry = &entry
	result.Outcome = entryResult.Outcome
	result.Created = entryResult.Created

	return printEntryWriteOutput(cmd.OutOrStdout(), opts.outputFormat, result)
}

func validateEntryWriteOptions(opts entriesWriteOptions) error {
	if err := validateOutputFormat(opts.outputFormat); err != nil {
		return err
	}
	if strings.TrimSpace(opts.parentObject) == "" {
		return fmt.Errorf("--parent-object is required")
	}
	if strings.TrimSpace(opts.parentRecordID) == "" {
		return fmt.Errorf("--parent-record-id is required")
	}
	return nil
}

func loadEntryWriteMetadata(ctx context.Context, client *attio.Client, list, parentObject string) (*entryWriteMetadata, error) {
	lists, err := client.ListLists(ctx)
	if err != nil {
		return nil, fmt.Errorf("list lists: %w", err)
	}

	metadata := &entryWriteMetadata{
		List: entryWriteList{
			Identifier: list,
		},
		ParentObject: recordWriteObject{
			Identifier: parentObject,
		},
	}
	for _, candidate := range lists {
		if candidate.APISlug == list || candidate.ID.ListID == list {
			metadata.List.ListID = candidate.ID.ListID
			metadata.List.Name = candidate.Name
			metadata.List.ParentObject = append([]string(nil), candidate.ParentObject...)
			break
		}
	}

	objects, err := client.ListObjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("list objects: %w", err)
	}
	for _, candidate := range objects {
		if candidate.APISlug == parentObject || candidate.ID.ObjectID == parentObject {
			metadata.ParentObject.ObjectID = candidate.ID.ObjectID
			metadata.ParentObject.SingularNoun = candidate.SingularNoun
			metadata.ParentObject.PluralNoun = candidate.PluralNoun
			metadata.ParentObjectAPISlug = candidate.APISlug
			break
		}
	}

	attributes, err := client.ListListAttributes(ctx, list, false)
	if err != nil {
		return nil, fmt.Errorf("list list attributes: %w", err)
	}
	metadata.Attributes = attributes

	return metadata, nil
}

func validateEntryWriteMetadata(metadata *entryWriteMetadata, parentObject string, values map[string]any) error {
	if err := validateRecordCreateValues(values, metadata.Attributes); err != nil {
		return err
	}
	if len(metadata.List.ParentObject) > 0 && !listAllowsParentObject(metadata, parentObject) {
		return fmt.Errorf("list %q accepts parent object %q; got %q", metadata.List.Identifier, strings.Join(metadata.List.ParentObject, ", "), parentObject)
	}
	return nil
}

func listAllowsParentObject(metadata *entryWriteMetadata, parentObject string) bool {
	identifiers := map[string]struct{}{
		parentObject: {},
	}
	if metadata.ParentObject.ObjectID != "" {
		identifiers[metadata.ParentObject.ObjectID] = struct{}{}
	}
	if metadata.ParentObjectAPISlug != "" {
		identifiers[metadata.ParentObjectAPISlug] = struct{}{}
	}

	for _, allowed := range metadata.List.ParentObject {
		if _, ok := identifiers[allowed]; ok {
			return true
		}
	}
	return false
}

func entryWritePayload(parentObject, parentRecordID string, values map[string]any) map[string]any {
	return map[string]any{
		"data": map[string]any{
			"parent_record_id": parentRecordID,
			"parent_object":    parentObject,
			"entry_values":     values,
		},
	}
}

func printEntryWriteOutput(out io.Writer, format string, result entryWriteOutput) error {
	switch format {
	case outputFormatTable:
		return printEntryWriteTable(out, result)
	case outputFormatJSON:
		return printEntryWriteJSON(out, result)
	default:
		return fmt.Errorf("unsupported output format %q; use table or json", format)
	}
}

func printEntryWriteTable(out io.Writer, result entryWriteOutput) error {
	if result.DryRun {
		if _, err := fmt.Fprintln(out, "DRY RUN: no write endpoint called"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "List: %s\n", result.List.Identifier); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "Parent object: %s\n", result.ParentObject.Identifier); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "Parent record ID: %s\n", result.ParentRecordID); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out, "Payload:"); err != nil {
			return err
		}
		return writeIndentedJSON(out, result.Payload)
	}

	entryID, listID, parentRecordID, parentObject, createdAt := entryFields(result)
	if listID == "" {
		listID = result.List.ListID
	}
	if parentObject == "" {
		parentObject = result.ParentObject.Identifier
	}
	if parentRecordID == "" {
		parentRecordID = result.ParentRecordID
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "LIST\tLIST ID\tENTRY ID\tNAME\tPARENT OBJECT\tPARENT OBJECT ID\tPARENT RECORD ID\tSINGULAR\tPLURAL\tOUTCOME\tCREATED AT"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		w,
		"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		result.List.Identifier,
		listID,
		entryID,
		result.List.Name,
		parentObject,
		result.ParentObject.ObjectID,
		parentRecordID,
		result.ParentObject.SingularNoun,
		result.ParentObject.PluralNoun,
		entryOutcome(result),
		createdAt,
	); err != nil {
		return err
	}
	return w.Flush()
}

func printEntryWriteJSON(out io.Writer, result entryWriteOutput) error {
	entryID, listID, parentRecordID, parentObject, createdAt := entryFields(result)
	if listID == "" {
		listID = result.List.ListID
	}
	if parentObject == "" {
		parentObject = result.ParentObject.Identifier
	}
	if parentRecordID == "" {
		parentRecordID = result.ParentRecordID
	}

	output := entryWriteJSONOutput{
		DryRun:              result.DryRun,
		WriteEndpointCalled: !result.DryRun,
		Outcome:             result.Outcome,
		Created:             result.Created,
		List: entryWriteListJSON{
			Identifier:   result.List.Identifier,
			ListID:       listID,
			Name:         result.List.Name,
			ParentObject: result.List.ParentObject,
		},
		Parent: entryWriteParentJSON{
			Object:       parentObject,
			ObjectID:     result.ParentObject.ObjectID,
			RecordID:     parentRecordID,
			SingularNoun: result.ParentObject.SingularNoun,
			PluralNoun:   result.ParentObject.PluralNoun,
		},
	}
	if result.DryRun {
		output.Payload = result.Payload
	} else {
		output.Entry = &entryWriteEntryJSON{
			EntryID:   entryID,
			CreatedAt: createdAt,
		}
	}

	return writeIndentedJSON(out, output)
}

type entryWriteJSONOutput struct {
	DryRun              bool                 `json:"dry_run"`
	WriteEndpointCalled bool                 `json:"write_endpoint_called"`
	Outcome             string               `json:"outcome,omitempty"`
	Created             *bool                `json:"created,omitempty"`
	List                entryWriteListJSON   `json:"list"`
	Parent              entryWriteParentJSON `json:"parent"`
	Payload             map[string]any       `json:"payload,omitempty"`
	Entry               *entryWriteEntryJSON `json:"entry,omitempty"`
}

type entryWriteListJSON struct {
	Identifier   string   `json:"identifier"`
	ListID       string   `json:"list_id,omitempty"`
	Name         string   `json:"name,omitempty"`
	ParentObject []string `json:"parent_object,omitempty"`
}

type entryWriteParentJSON struct {
	Object       string `json:"object"`
	ObjectID     string `json:"object_id,omitempty"`
	RecordID     string `json:"record_id"`
	SingularNoun string `json:"singular_noun,omitempty"`
	PluralNoun   string `json:"plural_noun,omitempty"`
}

type entryWriteEntryJSON struct {
	EntryID   string `json:"entry_id,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

func entryFields(result entryWriteOutput) (entryID, listID, parentRecordID, parentObject, createdAt string) {
	if result.Entry == nil {
		return "", "", "", "", ""
	}
	return result.Entry.ID.EntryID, result.Entry.ID.ListID, result.Entry.ParentRecordID, result.Entry.ParentObject, result.Entry.CreatedAt
}

func entryOutcome(result entryWriteOutput) string {
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
