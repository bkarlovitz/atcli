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
	return recordsCmd
}

type recordsCreateOptions struct {
	setValues     []string
	setJSONValues []string
	dryRun        bool
	outputFormat  string
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

func validateOutputFormat(format string) error {
	switch format {
	case outputFormatTable, outputFormatJSON:
		return nil
	default:
		return fmt.Errorf("unsupported output format %q; use table or json", format)
	}
}
