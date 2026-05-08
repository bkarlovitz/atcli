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
	ListIdentifier    string
	ListMode          string
	EntryMapping      *MappingPlan
	EntryAttributes   []attio.Attribute
	EntryMetadata     bool
}

type ImportPlan struct {
	ObjectIdentifier  string
	Mode              string
	MatchAttribute    string
	MatchDefaulted    bool
	MetadataAvailable bool
	Warnings          []string
	Rows              []PlannedRow
	ListIdentifier    string
	ListMode          string
	EntryMetadata     bool
}

type PlannedRow struct {
	RowNumber         int
	Mode              string
	Values            map[string]any
	EntryValues       map[string]any
	SkippedEmpty      []SkippedValue
	EntrySkippedEmpty []SkippedValue
	Warnings          []string
	Errors            []string
	Valid             bool
}

func BuildImportPlan(document *CSVDocument, mapping *MappingPlan, opts ImportPlanOptions) (*ImportPlan, error) {
	if opts.Mode != ModeCreate && opts.Mode != ModeUpsert {
		return nil, fmt.Errorf("unsupported import mode %q; use create or upsert", opts.Mode)
	}
	if opts.ListIdentifier != "" && opts.ListMode != ModeCreate && opts.ListMode != ModeUpsert {
		return nil, fmt.Errorf("unsupported list import mode %q; use create or upsert", opts.ListMode)
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

	entryMapping := opts.EntryMapping
	if opts.ListIdentifier != "" && entryMapping == nil {
		entryMapping = &MappingPlan{}
	}
	entryAttributes := attributeLookup(opts.EntryAttributes)
	if opts.ListIdentifier != "" && opts.EntryMetadata {
		if err := validatePlanAttributes(entryMapping, entryAttributes); err != nil {
			return nil, fmt.Errorf("list entry: %w", err)
		}
	}

	plan := &ImportPlan{
		ObjectIdentifier:  opts.ObjectIdentifier,
		Mode:              opts.Mode,
		MatchAttribute:    opts.MatchAttribute,
		MatchDefaulted:    opts.MatchDefaulted,
		MetadataAvailable: opts.MetadataAvailable,
		Warnings:          append([]string(nil), opts.Warnings...),
		ListIdentifier:    opts.ListIdentifier,
		ListMode:          opts.ListMode,
		EntryMetadata:     opts.EntryMetadata,
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
		if opts.ListIdentifier != "" {
			entryPrepared, err := PrepareRow(row, entryMapping, ValuePreparationOptions{
				Attributes:     opts.EntryAttributes,
				MultiSeparator: opts.MultiSeparator,
			})
			if err != nil {
				planned.Errors = append(planned.Errors, fmt.Sprintf("list entry: %s", err.Error()))
			} else {
				planned.EntryValues = entryPrepared.Values
				planned.EntrySkippedEmpty = entryPrepared.SkippedEmpty
				if opts.EntryMetadata {
					planned.Errors = append(planned.Errors, prefixValidationErrors("list entry", validateRowRequiredValues(entryPrepared.Values, opts.EntryAttributes))...)
				}
			}
		}
		planned.Valid = len(planned.Errors) == 0
		plan.Rows = append(plan.Rows, planned)
	}

	return plan, nil
}

func prefixValidationErrors(prefix string, errors []string) []string {
	if len(errors) == 0 {
		return nil
	}
	prefixed := make([]string, 0, len(errors))
	for _, err := range errors {
		prefixed = append(prefixed, prefix+": "+err)
	}
	return prefixed
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
