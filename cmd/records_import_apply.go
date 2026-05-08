package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/tabwriter"
	"time"

	"atcli/internal/attio"
	"atcli/internal/importplan"
)

const importRowWriteTimeout = 15 * time.Second
const maxImportRateLimitRetries = 3

var importRateLimitSleep = sleepForImportRateLimit

type importApplyResult struct {
	ObjectIdentifier string
	Mode             string
	MatchAttribute   string
	Rows             []importApplyRowResult
	Planned          int
	Succeeded        int
	Failed           int
	Created          int
	Updated          int
	Elapsed          time.Duration
}

type importApplyRowResult struct {
	RowNumber         int
	Mode              string
	Object            string
	MatchingAttribute string
	RecordID          string
	Status            string
	Outcome           string
	Created           *bool
	Errors            []string
}

func executeImportPlan(ctx context.Context, client *attio.Client, plan *importplan.ImportPlan) importApplyResult {
	started := time.Now()
	result := importApplyResult{
		ObjectIdentifier: plan.ObjectIdentifier,
		Mode:             plan.Mode,
		MatchAttribute:   plan.MatchAttribute,
		Planned:          len(plan.Rows),
		Rows:             make([]importApplyRowResult, 0, len(plan.Rows)),
	}

	for _, row := range plan.Rows {
		rowResult := importApplyRowResult{
			RowNumber:         row.RowNumber,
			Mode:              row.Mode,
			Object:            plan.ObjectIdentifier,
			MatchingAttribute: plan.MatchAttribute,
		}

		if !row.Valid {
			rowResult.Status = "failed"
			rowResult.Errors = append([]string(nil), row.Errors...)
			result.recordRow(rowResult)
			continue
		}

		record, outcome, created, err := executeImportRow(ctx, client, plan, row)
		if err != nil {
			rowResult.Status = "failed"
			rowResult.Errors = []string{classifyRecordWriteError(fmt.Sprintf("import row %d", row.RowNumber), err).Error()}
			result.recordRow(rowResult)
			continue
		}

		rowResult.RecordID = record.ID.RecordID
		rowResult.Outcome = outcome
		rowResult.Created = created
		rowResult.Status = importApplySuccessStatus(outcome, created)
		result.recordRow(rowResult)
	}

	result.Elapsed = time.Since(started)
	return result
}

func executeImportRow(ctx context.Context, client *attio.Client, plan *importplan.ImportPlan, row importplan.PlannedRow) (attio.Record, string, *bool, error) {
	var lastErr error
	rateLimited := false
	for attempt := 0; attempt <= maxImportRateLimitRetries; attempt++ {
		record, outcome, created, err := executeImportRowOnce(ctx, client, plan, row)
		if err == nil {
			return record, outcome, created, nil
		}
		lastErr = err

		delay, retryable := importRateLimitDelay(err, attempt)
		if !retryable {
			return attio.Record{}, "", nil, lastErr
		}
		rateLimited = true
		if attempt == maxImportRateLimitRetries {
			break
		}
		if err := importRateLimitSleep(ctx, delay); err != nil {
			return attio.Record{}, "", nil, err
		}
	}
	if !rateLimited {
		return attio.Record{}, "", nil, lastErr
	}
	return attio.Record{}, "", nil, fmt.Errorf("rate limit retry attempts exhausted: %w", lastErr)
}

func executeImportRowOnce(ctx context.Context, client *attio.Client, plan *importplan.ImportPlan, row importplan.PlannedRow) (attio.Record, string, *bool, error) {
	rowCtx, cancel := context.WithTimeout(ctx, importRowWriteTimeout)
	defer cancel()

	switch plan.Mode {
	case importplan.ModeCreate:
		record, err := client.CreateRecord(rowCtx, plan.ObjectIdentifier, row.Values)
		if err != nil {
			return attio.Record{}, "", nil, err
		}
		return *record, "", nil, nil
	case importplan.ModeUpsert:
		assertResult, err := client.AssertRecord(rowCtx, plan.ObjectIdentifier, plan.MatchAttribute, row.Values)
		if err != nil {
			return attio.Record{}, "", nil, err
		}
		return assertResult.Record, assertResult.Outcome, assertResult.Created, nil
	default:
		return attio.Record{}, "", nil, fmt.Errorf("unsupported import mode %q", plan.Mode)
	}
}

func importRateLimitDelay(err error, attempt int) (time.Duration, bool) {
	var apiErr *attio.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests {
		return 0, false
	}
	if apiErr.HasRetryAfter {
		return apiErr.RetryAfter, true
	}

	delay := 100 * time.Millisecond
	for i := 0; i < attempt; i++ {
		delay *= 2
	}
	if delay > 2*time.Second {
		return 2 * time.Second, true
	}
	return delay, true
}

func sleepForImportRateLimit(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (r *importApplyResult) recordRow(row importApplyRowResult) {
	r.Rows = append(r.Rows, row)
	switch row.Status {
	case "failed":
		r.Failed++
	case "created":
		r.Succeeded++
		r.Created++
	case "updated":
		r.Succeeded++
		r.Updated++
	default:
		r.Succeeded++
	}
}

func importApplySuccessStatus(outcome string, created *bool) string {
	if created != nil {
		if *created {
			return "created"
		}
		return "updated"
	}
	switch strings.ToLower(outcome) {
	case "created", "updated":
		return strings.ToLower(outcome)
	default:
		return "succeeded"
	}
}

func printImportApplyOutput(out io.Writer, format string, result importApplyResult) error {
	switch format {
	case outputFormatTable:
		return printImportApplyTable(out, result)
	case outputFormatJSONL:
		return printImportApplyJSONL(out, result)
	default:
		return fmt.Errorf("unsupported output format %q; use table or jsonl", format)
	}
}

func printImportApplyTable(out io.Writer, result importApplyResult) error {
	if _, err := fmt.Fprintln(out, "APPLY: write endpoint called for valid rows"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Object: %s\n", result.ObjectIdentifier); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Mode: %s\n", result.Mode); err != nil {
		return err
	}
	if result.MatchAttribute != "" {
		if _, err := fmt.Fprintf(out, "Matching attribute: %s\n", result.MatchAttribute); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(
		out,
		"Rows: %d (succeeded: %d, failed: %d, created: %d, updated: %d)\n",
		result.Planned,
		result.Succeeded,
		result.Failed,
		result.Created,
		result.Updated,
	); err != nil {
		return err
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "ROW\tSTATUS\tRECORD ID\tERRORS"); err != nil {
		return err
	}
	for _, row := range result.Rows {
		if _, err := fmt.Fprintf(
			w,
			"%d\t%s\t%s\t%s\n",
			row.RowNumber,
			row.Status,
			row.RecordID,
			strings.Join(row.Errors, "; "),
		); err != nil {
			return err
		}
	}
	return w.Flush()
}

func printImportApplyJSONL(out io.Writer, result importApplyResult) error {
	encoder := json.NewEncoder(out)
	for _, row := range result.Rows {
		event := importApplyRowEvent{
			Type:              "row",
			RowNumber:         row.RowNumber,
			Mode:              row.Mode,
			Object:            row.Object,
			MatchingAttribute: row.MatchingAttribute,
			RecordID:          row.RecordID,
			Status:            row.Status,
			Outcome:           row.Outcome,
			Created:           row.Created,
			Errors:            row.Errors,
		}
		if err := encoder.Encode(event); err != nil {
			return err
		}
	}
	return nil
}

type importApplyRowEvent struct {
	Type              string   `json:"type"`
	RowNumber         int      `json:"row_number"`
	Mode              string   `json:"mode"`
	Object            string   `json:"object"`
	MatchingAttribute string   `json:"matching_attribute,omitempty"`
	RecordID          string   `json:"record_id,omitempty"`
	Status            string   `json:"status"`
	Outcome           string   `json:"outcome,omitempty"`
	Created           *bool    `json:"created,omitempty"`
	Errors            []string `json:"errors,omitempty"`
}
