package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(newRecordsCommand())
}

func newRecordsCommand() *cobra.Command {
	recordsCmd := &cobra.Command{
		Use:   "records",
		Short: "Create and manage Attio records",
	}
	recordsCmd.AddCommand(newRecordsCreateCommand())
	recordsCmd.AddCommand(newRecordsUpsertCommand())
	return recordsCmd
}

type recordsCreateOptions struct {
	setValues     []string
	setJSONValues []string
	dryRun        bool
	outputFormat  string
}

type recordsUpsertOptions struct {
	setValues      []string
	setJSONValues  []string
	dryRun         bool
	outputFormat   string
	matchAttribute string
}

func newRecordsCreateCommand() *cobra.Command {
	opts := recordsCreateOptions{
		outputFormat: outputFormatTable,
	}

	createCmd := &cobra.Command{
		Use:   "create <object>",
		Short: "Create one Attio record",
		Long: strings.TrimSpace(`
Create one Attio record from shell flags.

The <object> argument is an Attio object slug or ID. Standard object slugs are
usually plural, such as "people" and "companies". atcli sends this argument as
provided and does not singularize or pluralize it.
`),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("accepts 1 arg, received %d", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecordsCreate(cmd, args[0], opts)
		},
	}
	createCmd.Flags().StringArrayVar(&opts.setValues, "set", nil, "set an attribute as a string value (attr=value)")
	createCmd.Flags().StringArrayVar(&opts.setJSONValues, "set-json", nil, "set an attribute as a JSON value (attr=json)")
	createCmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "print the write payload without calling the write endpoint")
	createCmd.Flags().StringVar(&opts.outputFormat, "output", outputFormatTable, "output format: table or json")

	return createCmd
}

func newRecordsUpsertCommand() *cobra.Command {
	opts := recordsUpsertOptions{
		outputFormat: outputFormatTable,
	}

	upsertCmd := &cobra.Command{
		Use:   "upsert <object>",
		Short: "Create or update one Attio record",
		Long: strings.TrimSpace(`
Create or update one Attio record from shell flags using a unique matching
attribute.

The <object> argument is an Attio object slug or ID. Standard object slugs are
usually plural, such as "people" and "companies". atcli sends this argument as
provided and does not singularize or pluralize it.

Use --match to choose the unique attribute used to find an existing record.
Without --match, safe defaults are available only for companies, people, users,
and workspaces.
`),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("accepts 1 arg, received %d", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecordsUpsert(cmd, args[0], opts)
		},
	}
	upsertCmd.Flags().StringArrayVar(&opts.setValues, "set", nil, "set an attribute as a string value (attr=value)")
	upsertCmd.Flags().StringArrayVar(&opts.setJSONValues, "set-json", nil, "set an attribute as a JSON value (attr=json)")
	upsertCmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "print the write payload without calling the write endpoint")
	upsertCmd.Flags().StringVar(&opts.outputFormat, "output", outputFormatTable, "output format: table or json")
	upsertCmd.Flags().StringVar(&opts.matchAttribute, "match", "", "unique attribute slug or ID to match existing records")

	return upsertCmd
}

func runRecordsCreate(cmd *cobra.Command, object string, opts recordsCreateOptions) error {
	if err := validateOutputFormat(opts.outputFormat); err != nil {
		return err
	}

	values, err := parseRecordValueFlags(opts.setValues, opts.setJSONValues)
	if err != nil {
		return err
	}

	result := recordWriteOutput{
		DryRun: opts.dryRun,
		Object: recordWriteObject{
			Identifier: object,
		},
		Payload: recordCreatePayload(values),
	}

	if opts.dryRun {
		return printRecordWriteOutput(cmd.OutOrStdout(), opts.outputFormat, result)
	}

	client, err := loadAttioClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
	defer cancel()

	metadata, err := loadRecordCreateMetadata(ctx, client, object)
	if err != nil {
		if isMetadataPermissionError(err) {
			if _, warnErr := fmt.Fprintln(cmd.ErrOrStderr(), "Metadata unavailable; token needs object_configuration:read. Local validation and noun display skipped."); warnErr != nil {
				return warnErr
			}
		} else {
			return fmt.Errorf("could not fetch object metadata: %w", err)
		}
	} else {
		result.Object = metadata.Object
		if err := validateRecordCreateValues(values, metadata.Attributes); err != nil {
			return err
		}
	}

	record, err := client.CreateRecord(ctx, object, values)
	if err != nil {
		return fmt.Errorf("could not create record: %w", err)
	}
	result.Record = record

	return printRecordWriteOutput(cmd.OutOrStdout(), opts.outputFormat, result)
}

func runRecordsUpsert(cmd *cobra.Command, object string, opts recordsUpsertOptions) error {
	if err := validateOutputFormat(opts.outputFormat); err != nil {
		return err
	}

	values, err := parseRecordValueFlags(opts.setValues, opts.setJSONValues)
	if err != nil {
		return err
	}

	matchAttribute, usedDefaultMatch, err := resolveRecordMatchAttribute(object, opts.matchAttribute)
	if err != nil {
		return err
	}

	result := recordWriteOutput{
		DryRun: opts.dryRun,
		Object: recordWriteObject{
			Identifier: object,
		},
		MatchAttribute: matchAttribute,
		MatchDefaulted: usedDefaultMatch,
		Payload:        recordAssertPayload(values),
	}

	client, err := loadAttioClient()
	if err != nil {
		if opts.dryRun {
			if _, warnErr := fmt.Fprintf(cmd.ErrOrStderr(), "Metadata unavailable: %v. Local validation and match uniqueness validation skipped.\n", err); warnErr != nil {
				return warnErr
			}
			return printRecordWriteOutput(cmd.OutOrStdout(), opts.outputFormat, result)
		}
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
	defer cancel()

	metadata, err := loadRecordCreateMetadata(ctx, client, object)
	if err != nil {
		if isMetadataPermissionError(err) {
			if usedDefaultMatch {
				return fmt.Errorf("metadata unavailable; token needs object_configuration:read. Pass --match explicitly to skip match uniqueness validation")
			}
			if _, warnErr := fmt.Fprintln(cmd.ErrOrStderr(), "Metadata unavailable; token needs object_configuration:read. Local validation, noun display, and match uniqueness validation skipped."); warnErr != nil {
				return warnErr
			}
		} else {
			return fmt.Errorf("could not fetch object metadata: %w", err)
		}
	} else {
		result.Object = metadata.Object
		if err := validateRecordCreateValues(values, metadata.Attributes); err != nil {
			return err
		}
		if err := validateRecordMatchAttribute(values, metadata.Attributes, matchAttribute); err != nil {
			return err
		}
	}

	if opts.dryRun {
		return printRecordWriteOutput(cmd.OutOrStdout(), opts.outputFormat, result)
	}

	assertResult, err := client.AssertRecord(ctx, object, matchAttribute, values)
	if err != nil {
		return fmt.Errorf("could not upsert record: %w", err)
	}
	record := assertResult.Record
	result.Record = &record
	result.Outcome = assertResult.Outcome
	result.Created = assertResult.Created

	return printRecordWriteOutput(cmd.OutOrStdout(), opts.outputFormat, result)
}

func validateOutputFormat(format string) error {
	switch format {
	case outputFormatTable, outputFormatJSON:
		return nil
	default:
		return fmt.Errorf("unsupported output format %q; use table or json", format)
	}
}
