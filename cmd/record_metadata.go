package cmd

import (
	"context"
	"fmt"
	"sort"

	"atcli/internal/attio"
)

type recordCreateMetadata struct {
	Object     recordWriteObject
	Attributes []attio.Attribute
}

func loadRecordCreateMetadata(ctx context.Context, client *attio.Client, object string) (*recordCreateMetadata, error) {
	objects, err := client.ListObjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("list objects: %w", err)
	}

	metadata := &recordCreateMetadata{
		Object: recordWriteObject{
			Identifier: object,
		},
	}
	for _, candidate := range objects {
		if candidate.APISlug == object || candidate.ID.ObjectID == object {
			metadata.Object.ObjectID = candidate.ID.ObjectID
			metadata.Object.SingularNoun = candidate.SingularNoun
			metadata.Object.PluralNoun = candidate.PluralNoun
			break
		}
	}

	attributes, err := client.ListObjectAttributes(ctx, object, false)
	if err != nil {
		return nil, fmt.Errorf("list object attributes: %w", err)
	}
	metadata.Attributes = attributes

	return metadata, nil
}

func isMetadataPermissionError(err error) bool {
	return attio.IsPermissionError(err)
}

func validateRecordCreateValues(values map[string]any, attributes []attio.Attribute) error {
	byIdentifier := make(map[string]attio.Attribute, len(attributes))
	for _, attribute := range attributes {
		for _, identifier := range attributeIdentifiers(attribute) {
			byIdentifier[identifier] = attribute
		}
	}

	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		attribute, ok := byIdentifier[name]
		if !ok {
			return fmt.Errorf("unknown attribute %q", name)
		}
		if !attribute.IsWritable {
			return fmt.Errorf("attribute %q is not writable", name)
		}
		if attribute.IsEditable != nil && !*attribute.IsEditable {
			return fmt.Errorf("attribute %q is not editable", name)
		}
	}

	missingRequired := make([]string, 0)
	for _, attribute := range attributes {
		if !attribute.IsRequired || attribute.APISlug == "" || !attributeCanBeSetOnCreate(attribute) {
			continue
		}
		if !recordValuePresent(values, attributeIdentifiers(attribute)) {
			missingRequired = append(missingRequired, attribute.APISlug)
		}
	}
	sort.Strings(missingRequired)
	if len(missingRequired) > 0 {
		return fmt.Errorf("missing required attribute %q", missingRequired[0])
	}

	return nil
}

func validateRecordMatchAttribute(values map[string]any, attributes []attio.Attribute, match string) error {
	attribute, ok := findAttributeByIdentifier(attributes, match)
	if !ok {
		return fmt.Errorf("matching attribute %q was not found in object metadata", match)
	}
	if !attribute.IsUnique {
		return fmt.Errorf("matching attribute %q is not unique; choose a unique attribute with --match", match)
	}
	if !recordValuePresent(values, attributeIdentifiers(attribute)) {
		return fmt.Errorf("matching attribute %q must have a value in the record payload", match)
	}

	return nil
}

func findAttributeByIdentifier(attributes []attio.Attribute, identifier string) (attio.Attribute, bool) {
	for _, attribute := range attributes {
		for _, candidate := range attributeIdentifiers(attribute) {
			if candidate == identifier {
				return attribute, true
			}
		}
	}
	return attio.Attribute{}, false
}

func attributeIdentifiers(attribute attio.Attribute) []string {
	identifiers := make([]string, 0, 2)
	if attribute.APISlug != "" {
		identifiers = append(identifiers, attribute.APISlug)
	}
	if attribute.ID.AttributeID != "" {
		identifiers = append(identifiers, attribute.ID.AttributeID)
	}
	return identifiers
}

func recordValuePresent(values map[string]any, identifiers []string) bool {
	for _, identifier := range identifiers {
		value, ok := values[identifier]
		if ok && value != nil {
			return true
		}
	}
	return false
}

func attributeCanBeSetOnCreate(attribute attio.Attribute) bool {
	return attribute.IsWritable && (attribute.IsEditable == nil || *attribute.IsEditable)
}
