package importplan

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"atcli/internal/attio"
)

type ValuePreparationOptions struct {
	Attributes     []attio.Attribute
	MultiSeparator string
}

type PreparedRow struct {
	RowNumber    int
	Values       map[string]any
	SkippedEmpty []SkippedValue
}

type SkippedValue struct {
	CSVColumn string
	Attribute string
}

func PrepareRow(row CSVRow, mapping *MappingPlan, opts ValuePreparationOptions) (*PreparedRow, error) {
	attributes := make(map[string]attio.Attribute, len(opts.Attributes))
	for _, attribute := range opts.Attributes {
		for _, identifier := range AttributeIdentifiers(attribute) {
			attributes[identifier] = attribute
		}
	}

	values := cloneValues(mapping.StaticValues)
	if values == nil {
		values = make(map[string]any)
	}

	prepared := &PreparedRow{
		RowNumber: row.Number,
		Values:    values,
	}

	for _, column := range mapping.Columns {
		raw := row.Values[column.CSVColumn]
		if strings.TrimSpace(raw) == "" {
			prepared.SkippedEmpty = append(prepared.SkippedEmpty, SkippedValue{
				CSVColumn: column.CSVColumn,
				Attribute: column.Attribute,
			})
			continue
		}

		attribute := attributes[column.Attribute]
		value, err := prepareCellValue(raw, attribute, opts.MultiSeparator)
		if err != nil {
			return nil, fmt.Errorf("row %d column %q: %w", row.Number, column.CSVColumn, err)
		}
		if value == nil {
			prepared.SkippedEmpty = append(prepared.SkippedEmpty, SkippedValue{
				CSVColumn: column.CSVColumn,
				Attribute: column.Attribute,
			})
			continue
		}
		prepared.Values[column.Attribute] = value
	}

	return prepared, nil
}

func prepareCellValue(raw string, attribute attio.Attribute, multiSeparator string) (any, error) {
	if attribute.IsMultiselect && multiSeparator != "" {
		parts := strings.Split(raw, multiSeparator)
		values := make([]any, 0, len(parts))
		for _, part := range parts {
			if strings.TrimSpace(part) == "" {
				continue
			}
			value, err := prepareScalarValue(part, attribute)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		if len(values) == 0 {
			return nil, nil
		}
		return values, nil
	}

	return prepareScalarValue(raw, attribute)
}

func prepareScalarValue(raw string, attribute attio.Attribute) (any, error) {
	value := strings.TrimSpace(raw)
	switch strings.ToLower(attribute.Type) {
	case "number":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("expected number, got %q", raw)
		}
		return parsed, nil
	case "checkbox", "boolean", "bool":
		parsed, ok := parseCheckbox(value)
		if !ok {
			return nil, fmt.Errorf("expected checkbox value true/false, yes/no, or 1/0, got %q", raw)
		}
		return parsed, nil
	case "date":
		parsed, err := time.Parse("2006-01-02", value)
		if err != nil {
			return nil, fmt.Errorf("expected ISO date YYYY-MM-DD, got %q", raw)
		}
		return parsed.Format("2006-01-02"), nil
	case "timestamp", "datetime", "date-time":
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return nil, fmt.Errorf("expected RFC3339 timestamp, got %q", raw)
		}
		return parsed.UTC().Format(time.RFC3339), nil
	default:
		return value, nil
	}
}

func parseCheckbox(value string) (bool, bool) {
	switch strings.ToLower(value) {
	case "true", "t", "yes", "y", "1":
		return true, true
	case "false", "f", "no", "n", "0":
		return false, true
	default:
		return false, false
	}
}

func AttributeIdentifiers(attribute attio.Attribute) []string {
	identifiers := make([]string, 0, 2)
	if attribute.APISlug != "" {
		identifiers = append(identifiers, attribute.APISlug)
	}
	if attribute.ID.AttributeID != "" {
		identifiers = append(identifiers, attribute.ID.AttributeID)
	}
	return identifiers
}
