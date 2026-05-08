package importplan

import (
	"errors"
	"fmt"
	"strings"
)

type MappingRule struct {
	CSVColumn string
	Attribute string
}

type MappingOptions struct {
	Rules        []MappingRule
	Ignore       []string
	StaticValues map[string]any
}

type MappingPlan struct {
	Columns      []ColumnMapping
	Ignored      []string
	StaticValues map[string]any
}

type ColumnMapping struct {
	CSVColumn string
	Attribute string
}

func ParseMappingRules(rawRules []string) ([]MappingRule, error) {
	rules := make([]MappingRule, 0, len(rawRules))
	for _, raw := range rawRules {
		rule, err := ParseMappingRule(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid --map %q: %w", raw, err)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func ParseMappingRule(raw string) (MappingRule, error) {
	csvColumn, attribute, ok := strings.Cut(raw, "=")
	if !ok {
		return MappingRule{}, errors.New("expected csv_column=attio_attribute")
	}
	csvColumn = strings.TrimSpace(csvColumn)
	attribute = strings.TrimSpace(attribute)
	if csvColumn == "" {
		return MappingRule{}, errors.New("CSV column cannot be empty")
	}
	if attribute == "" {
		return MappingRule{}, errors.New("Attio attribute cannot be empty")
	}
	return MappingRule{CSVColumn: csvColumn, Attribute: attribute}, nil
}

func NormalizeIgnoredColumns(rawColumns []string) ([]string, error) {
	columns := make([]string, 0, len(rawColumns))
	for _, raw := range rawColumns {
		column := strings.TrimSpace(raw)
		if column == "" {
			return nil, errors.New("ignored CSV column cannot be empty")
		}
		columns = append(columns, column)
	}
	return columns, nil
}

func BuildMappingPlan(headers []string, opts MappingOptions) (*MappingPlan, error) {
	headerSet := make(map[string]struct{}, len(headers))
	for _, header := range headers {
		headerSet[header] = struct{}{}
	}

	ignoredSet := make(map[string]struct{}, len(opts.Ignore))
	for _, column := range opts.Ignore {
		if _, ok := headerSet[column]; !ok {
			return nil, fmt.Errorf("ignored CSV column %q was not found in headers", column)
		}
		ignoredSet[column] = struct{}{}
	}

	explicitRules := make(map[string]string, len(opts.Rules))
	for _, rule := range opts.Rules {
		if _, ok := headerSet[rule.CSVColumn]; !ok {
			return nil, fmt.Errorf("mapped CSV column %q was not found in headers", rule.CSVColumn)
		}
		if _, ignored := ignoredSet[rule.CSVColumn]; ignored {
			return nil, fmt.Errorf("CSV column %q cannot be both mapped and ignored", rule.CSVColumn)
		}
		if _, exists := explicitRules[rule.CSVColumn]; exists {
			return nil, fmt.Errorf("CSV column %q is mapped more than once", rule.CSVColumn)
		}
		explicitRules[rule.CSVColumn] = rule.Attribute
	}

	staticValues := cloneValues(opts.StaticValues)
	plan := &MappingPlan{
		Ignored:      append([]string(nil), opts.Ignore...),
		StaticValues: staticValues,
	}

	targetSources := make(map[string]string, len(headers))
	for _, header := range headers {
		if _, ignored := ignoredSet[header]; ignored {
			continue
		}
		attribute := header
		if explicitAttribute, ok := explicitRules[header]; ok {
			attribute = explicitAttribute
		}
		if previousColumn, exists := targetSources[attribute]; exists {
			return nil, fmt.Errorf("Attio attribute %q is targeted by both CSV columns %q and %q", attribute, previousColumn, header)
		}
		if _, static := staticValues[attribute]; static {
			return nil, fmt.Errorf("Attio attribute %q cannot be both mapped from CSV and set statically", attribute)
		}
		targetSources[attribute] = header
		plan.Columns = append(plan.Columns, ColumnMapping{
			CSVColumn: header,
			Attribute: attribute,
		})
	}

	return plan, nil
}

func cloneValues(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
