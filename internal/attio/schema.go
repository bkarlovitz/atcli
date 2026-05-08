package attio

import (
	"context"
	"fmt"
	"net/url"
)

type objectsResponse struct {
	Data []Object `json:"data"`
}

type Object struct {
	ID           ObjectID `json:"id"`
	APISlug      string   `json:"api_slug"`
	SingularNoun string   `json:"singular_noun"`
	PluralNoun   string   `json:"plural_noun"`
	CreatedAt    string   `json:"created_at"`
}

type ObjectID struct {
	WorkspaceID string `json:"workspace_id"`
	ObjectID    string `json:"object_id"`
}

type listsResponse struct {
	Data []List `json:"data"`
}

type List struct {
	ID              ListID   `json:"id"`
	APISlug         string   `json:"api_slug"`
	Name            string   `json:"name"`
	ParentObject    []string `json:"parent_object"`
	WorkspaceAccess string   `json:"workspace_access"`
	CreatedAt       string   `json:"created_at"`
}

type ListID struct {
	WorkspaceID string `json:"workspace_id"`
	ListID      string `json:"list_id"`
}

type attributesResponse struct {
	Data []Attribute `json:"data"`
}

type Attribute struct {
	ID                AttributeID `json:"id"`
	Title             string      `json:"title"`
	Description       string      `json:"description"`
	APISlug           string      `json:"api_slug"`
	Type              string      `json:"type"`
	IsSystemAttribute bool        `json:"is_system_attribute"`
	IsWritable        bool        `json:"is_writable"`
	IsEditable        *bool       `json:"is_editable"`
	IsRequired        bool        `json:"is_required"`
	IsUnique          bool        `json:"is_unique"`
	IsMultiselect     bool        `json:"is_multiselect"`
	IsArchived        bool        `json:"is_archived"`
}

type AttributeID struct {
	WorkspaceID string `json:"workspace_id"`
	ObjectID    string `json:"object_id"`
	ListID      string `json:"list_id"`
	AttributeID string `json:"attribute_id"`
}

func (c *Client) ListObjects(ctx context.Context) ([]Object, error) {
	var response objectsResponse
	if err := c.getJSON(ctx, "/objects", &response); err != nil {
		return nil, fmt.Errorf("list objects: %w", err)
	}
	return response.Data, nil
}

func (c *Client) ListLists(ctx context.Context) ([]List, error) {
	var response listsResponse
	if err := c.getJSON(ctx, "/lists", &response); err != nil {
		return nil, fmt.Errorf("list lists: %w", err)
	}
	return response.Data, nil
}

func (c *Client) ListObjectAttributes(ctx context.Context, object string, includeArchived bool) ([]Attribute, error) {
	attributes, err := c.listAttributes(ctx, "objects", object, includeArchived)
	if err != nil {
		return nil, fmt.Errorf("list object attributes: %w", err)
	}
	return attributes, nil
}

func (c *Client) ListListAttributes(ctx context.Context, list string, includeArchived bool) ([]Attribute, error) {
	attributes, err := c.listAttributes(ctx, "lists", list, includeArchived)
	if err != nil {
		return nil, fmt.Errorf("list list attributes: %w", err)
	}
	return attributes, nil
}

func (c *Client) listAttributes(ctx context.Context, target, identifier string, includeArchived bool) ([]Attribute, error) {
	var response attributesResponse
	path := "/" + url.PathEscape(target) + "/" + url.PathEscape(identifier) + "/attributes"
	if includeArchived {
		path += "?show_archived=true"
	}
	if err := c.getJSON(ctx, path, &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}
