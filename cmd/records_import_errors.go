package cmd

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"atcli/internal/importplan"
)

var importErrorColumns = []string{
	"atcli_row_number",
	"atcli_mode",
	"atcli_object",
	"atcli_matching_attribute",
	"atcli_status",
	"atcli_errors",
}

func writeImportErrorCSV(path string, document *importplan.CSVDocument, result importApplyResult) error {
	if path == "" || result.Failed == 0 {
		return nil
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("write failed-row CSV %q: %w", path, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if err := writer.Write(importErrorCSVHeader(document)); err != nil {
		return fmt.Errorf("write failed-row CSV header: %w", err)
	}

	rows := importRowsByNumber(document)
	for _, resultRow := range result.Rows {
		if resultRow.Status != "failed" {
			continue
		}
		csvRow, ok := rows[resultRow.RowNumber]
		if !ok {
			return fmt.Errorf("write failed-row CSV: original CSV row %d was not found", resultRow.RowNumber)
		}
		if err := writer.Write(importErrorCSVRecord(document, csvRow, resultRow)); err != nil {
			return fmt.Errorf("write failed-row CSV row %d: %w", resultRow.RowNumber, err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("write failed-row CSV %q: %w", path, err)
	}
	return nil
}

func importErrorCSVHeader(document *importplan.CSVDocument) []string {
	header := append([]string(nil), document.Headers...)
	header = append(header, importErrorColumns...)
	return header
}

func importErrorCSVRecord(document *importplan.CSVDocument, csvRow importplan.CSVRow, resultRow importApplyRowResult) []string {
	record := make([]string, 0, len(document.Headers)+len(importErrorColumns))
	for _, header := range document.Headers {
		record = append(record, csvRow.Values[header])
	}
	record = append(record,
		fmt.Sprintf("%d", resultRow.RowNumber),
		resultRow.Mode,
		resultRow.Object,
		resultRow.MatchingAttribute,
		resultRow.Status,
		strings.Join(resultRow.Errors, "; "),
	)
	return record
}

func importRowsByNumber(document *importplan.CSVDocument) map[int]importplan.CSVRow {
	rows := make(map[int]importplan.CSVRow, len(document.Rows))
	for _, row := range document.Rows {
		rows[row.Number] = row
	}
	return rows
}
