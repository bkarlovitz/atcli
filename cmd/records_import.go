package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"atcli/internal/attio"
	"atcli/internal/importplan"

	"github.com/spf13/cobra"
)

type recordsImportOptions struct {
	setValues      []string
	setJSONValues  []string
	mapValues      []string
	ignoreColumns  []string
	mode           string
	matchAttribute string
	multiSeparator string
}

func newRecordsImportCommand() *cobra.Command {
	opts := recordsImportOptions{
		mode: importplan.ModeUpsert,
	}

	importCmd := &cobra.Command{
		Use:   "import <object> <csv>",
		Short: "Plan a CSV record import without writing records",
		Long: strings.TrimSpace(`
Plan a CSV record import without calling Attio write endpoints.

The <object> argument is an Attio object slug or ID. CSV headers map to Attio
attribute slugs by default. Use --map to map agent-friendly CSV headers to
Attio attributes and --ignore to leave a CSV column out of the planned payload.
`),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return fmt.Errorf("accepts 2 args, received %d", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecordsImport(cmd, args[0], args[1], opts)
		},
	}
	importCmd.Flags().StringArrayVar(&opts.setValues, "set", nil, "set a static attribute as a string value (attr=value)")
	importCmd.Flags().StringArrayVar(&opts.setJSONValues, "set-json", nil, "set a static attribute as a JSON value (attr=json)")
	importCmd.Flags().StringArrayVar(&opts.mapValues, "map", nil, "map a CSV column to an Attio attribute (csv_column=attio_attribute)")
	importCmd.Flags().StringArrayVar(&opts.ignoreColumns, "ignore", nil, "ignore a CSV column")
	importCmd.Flags().StringVar(&opts.mode, "mode", importplan.ModeUpsert, "planning mode: upsert or create")
	importCmd.Flags().StringVar(&opts.matchAttribute, "match", "", "unique attribute slug or ID to match existing records in upsert mode")
	importCmd.Flags().StringVar(&opts.multiSeparator, "multi-sep", "", "separator for multi-value CSV cells")

	return importCmd
}

func runRecordsImport(cmd *cobra.Command, object, csvPath string, opts recordsImportOptions) error {
	if opts.mode != importplan.ModeCreate && opts.mode != importplan.ModeUpsert {
		return fmt.Errorf("unsupported import mode %q; use create or upsert", opts.mode)
	}

	document, err := importplan.LoadCSV(csvPath)
	if err != nil {
		return err
	}

	staticValues, err := parseRecordValueFlags(opts.setValues, opts.setJSONValues)
	if err != nil {
		return err
	}

	rules, err := importplan.ParseMappingRules(opts.mapValues)
	if err != nil {
		return err
	}
	ignored, err := importplan.NormalizeIgnoredColumns(opts.ignoreColumns)
	if err != nil {
		return err
	}
	mapping, err := importplan.BuildMappingPlan(document.Headers, importplan.MappingOptions{
		Rules:        rules,
		Ignore:       ignored,
		StaticValues: staticValues,
	})
	if err != nil {
		return err
	}

	matchAttribute := ""
	matchDefaulted := false
	if opts.mode == importplan.ModeUpsert {
		matchAttribute, matchDefaulted, err = resolveRecordMatchAttribute(object, opts.matchAttribute)
		if err != nil {
			return err
		}
	}

	attributes, warnings, metadataAvailable, err := loadImportMetadata(cmd, object, opts.mode, matchDefaulted)
	if err != nil {
		return err
	}

	plan, err := importplan.BuildImportPlan(document, mapping, importplan.ImportPlanOptions{
		ObjectIdentifier:  object,
		Mode:              opts.mode,
		MatchAttribute:    matchAttribute,
		MatchDefaulted:    matchDefaulted,
		Attributes:        attributes,
		MetadataAvailable: metadataAvailable,
		MultiSeparator:    opts.multiSeparator,
		Warnings:          warnings,
	})
	if err != nil {
		return err
	}

	return printImportPlanTable(cmd.OutOrStdout(), plan)
}

func loadImportMetadata(cmd *cobra.Command, object, mode string, matchDefaulted bool) ([]attio.Attribute, []string, bool, error) {
	client, err := loadAttioClient()
	if err != nil {
		warning := fmt.Sprintf("Metadata unavailable: %v. Local validation skipped.", err)
		if mode == importplan.ModeUpsert {
			warning = fmt.Sprintf("Metadata unavailable: %v. Local validation and match uniqueness validation skipped.", err)
		}
		return nil, []string{warning}, false, nil
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
	defer cancel()

	metadata, err := loadRecordCreateMetadata(ctx, client, object)
	if err != nil {
		if isMetadataPermissionError(err) {
			if mode == importplan.ModeUpsert && matchDefaulted {
				return nil, nil, false, fmt.Errorf("metadata unavailable; token needs object_configuration:read. Pass --match explicitly to skip match uniqueness validation")
			}
			return nil, []string{"Metadata unavailable; token needs object_configuration:read. Local validation, noun display, and match uniqueness validation skipped."}, false, nil
		}
		return nil, nil, false, fmt.Errorf("could not fetch object metadata: %w", err)
	}

	return metadata.Attributes, nil, true, nil
}

func printImportPlanTable(out io.Writer, plan *importplan.ImportPlan) error {
	if _, err := fmt.Fprintln(out, "DRY RUN: no write endpoint called"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Object: %s\n", plan.ObjectIdentifier); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Mode: %s\n", plan.Mode); err != nil {
		return err
	}
	if plan.MatchAttribute != "" {
		if _, err := fmt.Fprintf(out, "Matching attribute: %s\n", plan.MatchAttribute); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(out, "Rows: %d\n", len(plan.Rows)); err != nil {
		return err
	}
	for _, warning := range plan.Warnings {
		if _, err := fmt.Fprintf(out, "Warning: %s\n", warning); err != nil {
			return err
		}
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "ROW\tSTATUS\tSKIPPED EMPTY\tVALUES\tERRORS"); err != nil {
		return err
	}
	for _, row := range plan.Rows {
		status := "valid"
		if !row.Valid {
			status = "invalid"
		}
		valuesJSON, err := compactJSON(row.Values)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(
			w,
			"%d\t%s\t%d\t%s\t%s\n",
			row.RowNumber,
			status,
			len(row.SkippedEmpty),
			valuesJSON,
			strings.Join(row.Errors, "; "),
		); err != nil {
			return err
		}
	}
	return w.Flush()
}

func compactJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
