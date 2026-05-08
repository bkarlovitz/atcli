package importplan

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
)

type CSVDocument struct {
	Path    string
	Headers []string
	Rows    []CSVRow
}

type CSVRow struct {
	Number int
	Values map[string]string
}

func LoadCSV(path string) (*CSVDocument, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read CSV %q: %w", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	rawHeaders, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("CSV file %q is empty", path)
		}
		return nil, fmt.Errorf("read CSV headers from %q: %w", path, err)
	}

	headers, err := validateCSVHeaders(rawHeaders)
	if err != nil {
		return nil, err
	}

	document := &CSVDocument{
		Path:    path,
		Headers: headers,
	}

	for rowNumber := 2; ; rowNumber++ {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read CSV row %d from %q: %w", rowNumber, path, err)
		}
		if len(record) != len(headers) {
			return nil, fmt.Errorf("CSV row %d has %d fields; expected %d", rowNumber, len(record), len(headers))
		}

		values := make(map[string]string, len(headers))
		for i, header := range headers {
			values[header] = record[i]
		}
		document.Rows = append(document.Rows, CSVRow{
			Number: rowNumber,
			Values: values,
		})
	}

	return document, nil
}

func validateCSVHeaders(rawHeaders []string) ([]string, error) {
	headers := make([]string, len(rawHeaders))
	seen := make(map[string]int, len(rawHeaders))
	for i, rawHeader := range rawHeaders {
		header := strings.TrimSpace(rawHeader)
		if header == "" {
			return nil, fmt.Errorf("CSV header at column %d is empty", i+1)
		}
		if previousColumn, ok := seen[header]; ok {
			return nil, fmt.Errorf("CSV header %q is duplicated at columns %d and %d", header, previousColumn, i+1)
		}
		seen[header] = i + 1
		headers[i] = header
	}
	return headers, nil
}
