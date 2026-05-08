package importplan

import (
	"fmt"
	"sort"

	"atcli/internal/attio"
)

const (
	ModeCreate = "create"
	ModeUpsert = "upsert"
)

type ImportPlanOptions struct {
	ObjectIdentifier  string
	Mode              string
	MatchAttribute    string
	MatchDefaulted    bool
	Attributes        []attio.Attribute
	MetadataAvailable bool
	MultiSeparator    string
	Warnings          []string
}

type ImportPlan struct {
	ObjectIdentifier  string
	Mode              string
	MatchAttribute    string
	MatchDefaulted    bool
	MetadataAvailable bool
	Warnings          []string
	Rows              []PlannedRow
}

type PlannedRow struct {
	RowNumber    int
	Mode         string
	Values       map[string]any
	SkippedEmpty []SkippedValue
	Warnings     []string
	Errors       []string
	Valid        bool
}

func BuildImportPlan(document *CSVDocument, mapping *MappingPlan, opts ImportPlanOptions) (*ImportPlan, error) {
	if opts.Mode != ModeCreate && opts.Mode != ModeUpsert {
		return nil, fmt.Errorf("unsupported import mode %q; use create or upsert", opts.Mode)
	}

	attributes := attributeLookup(opts.Attributes)
	if opts.MetadataAvailable {
		if err := validatePlanAttributes(mapping, attributes); err != nil {
			return nil, err
		}
		if opts.Mode == ModeUpsert {
			match, ok := attributes[opts.MatchAttribute]
			if !ok {
				return nil, fmt.Errorf("matching attribute %q was not found in object metadata", opts.MatchAttribute)
			}
			if !match.IsUnique {
				return nil, fmt.Errorf("matching attribute %q is not unique; choose a unique attribute with --match", opts.MatchAttribute)
			}
		}
	}

	plan := &ImportPlan{
		ObjectIdentifier:  opts.ObjectIdentifier,
		Mode:              opts.Mode,
		MatchAttribute:    opts.MatchAttribute,
		MatchDefaulted:    opts.MatchDefaulted,
		MetadataAvailable: opts.MetadataAvailable,
		Warnings:          append([]string(nil), opts.Warnings...),
	}

	for _, row := range document.Rows {
		planned := PlannedRow{
			RowNumber: row.Number,
			Mode:      opts.Mode,
			Valid:     true,
		}

		prepared, err := PrepareRow(row, mapping, ValuePreparationOptions{
			Attributes:     opts.Attributes,
			MultiSeparator: opts.MultiSeparator,
		})
		if err != nil {
			planned.Valid = false
			planned.Errors = append(planned.Errors, err.Error())
			plan.Rows = append(plan.Rows, planned)
			continue
		}

		planned.Values = prepared.Values
		planned.SkippedEmpty = prepared.SkippedEmpty
		if opts.MetadataAvailable {
			planned.Errors = append(planned.Errors, validateRowRequiredValues(prepared.Values, opts.Attributes)...)
			if opts.Mode == ModeUpsert {
				match := attributes[opts.MatchAttribute]
				if !valuePresent(prepared.Values, AttributeIdentifiers(match)) {
					planned.Errors = append(planned.Errors, fmt.Sprintf("matching attribute %q must have a value", opts.MatchAttribute))
				}
			}
		}
		planned.Valid = len(planned.Errors) == 0
		plan.Rows = append(plan.Rows, planned)
	}

	return plan, nil
}

func validatePlanAttributes(mapping *MappingPlan, attributes map[string]attio.Attribute) error {
	for _, column := range mapping.Columns {
		attribute, ok := attributes[column.Attribute]
		if !ok {
			return fmt.Errorf("unknown attribute %q mapped from CSV column %q", column.Attribute, column.CSVColumn)
		}
		if err := validatePlanAttributeWritable(column.Attribute, attribute); err != nil {
			return err
		}
	}

	staticNames := make([]string, 0, len(mapping.StaticValues))
	for name := range mapping.StaticValues {
		staticNames = append(staticNames, name)
	}
	sort.Strings(staticNames)
	for _, name := range staticNames {
		attribute, ok := attributes[name]
		if !ok {
			return fmt.Errorf("unknown static attribute %q", name)
		}
		if err := validatePlanAttributeWritable(name, attribute); err != nil {
			return err
		}
	}

	return nil
}

func validatePlanAttributeWritable(name string, attribute attio.Attribute) error {
	if !attribute.IsWritable {
		return fmt.Errorf("attribute %q is not writable", name)
	}
	if attribute.IsEditable != nil && !*attribute.IsEditable {
		return fmt.Errorf("attribute %q is not editable", name)
	}
	return nil
}

func validateRowRequiredValues(values map[string]any, attributes []attio.Attribute) []string {
	var errors []string
	for _, attribute := range attributes {
		if !attribute.IsRequired || attribute.APISlug == "" || !attributeCanBeSet(attribute) {
			continue
		}
		if !valuePresent(values, AttributeIdentifiers(attribute)) {
			errors = append(errors, fmt.Sprintf("missing required attribute %q", attribute.APISlug))
		}
	}
	sort.Strings(errors)
	return errors
}

func attributeCanBeSet(attribute attio.Attribute) bool {
	return attribute.IsWritable && (attribute.IsEditable == nil || *attribute.IsEditable)
}

func attributeLookup(attributes []attio.Attribute) map[string]attio.Attribute {
	lookup := make(map[string]attio.Attribute, len(attributes))
	for _, attribute := range attributes {
		for _, identifier := range AttributeIdentifiers(attribute) {
			lookup[identifier] = attribute
		}
	}
	return lookup
}

func valuePresent(values map[string]any, identifiers []string) bool {
	for _, identifier := range identifiers {
		value, ok := values[identifier]
		if ok && value != nil {
			return true
		}
	}
	return false
}
