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
	bySlug := make(map[string]attio.Attribute, len(attributes))
	for _, attribute := range attributes {
		if attribute.APISlug != "" {
			bySlug[attribute.APISlug] = attribute
		}
	}

	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		attribute, ok := bySlug[name]
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
		value, ok := values[attribute.APISlug]
		if !ok || value == nil {
			missingRequired = append(missingRequired, attribute.APISlug)
		}
	}
	sort.Strings(missingRequired)
	if len(missingRequired) > 0 {
		return fmt.Errorf("missing required attribute %q", missingRequired[0])
	}

	return nil
}

func attributeCanBeSetOnCreate(attribute attio.Attribute) bool {
	return attribute.IsWritable && (attribute.IsEditable == nil || *attribute.IsEditable)
}
