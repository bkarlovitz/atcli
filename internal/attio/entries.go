package attio

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

type listEntryRequest struct {
	Data listEntryRequestData `json:"data"`
}

type listEntryRequestData struct {
	ParentRecordID string         `json:"parent_record_id"`
	ParentObject   string         `json:"parent_object"`
	EntryValues    map[string]any `json:"entry_values"`
}

type listEntryResponse struct {
	Data listEntryData `json:"data"`
}

type listEntryData struct {
	ListEntry
	Status    string `json:"status"`
	Outcome   string `json:"outcome"`
	Operation string `json:"operation"`
	Created   *bool  `json:"created"`
}

type ListEntryWrite struct {
	ParentRecordID string
	ParentObject   string
	EntryValues    map[string]any
}

type ListEntryResult struct {
	Entry   ListEntry
	Outcome string
	Created *bool
}

type ListEntry struct {
	ID             ListEntryID    `json:"id"`
	ParentRecordID string         `json:"parent_record_id"`
	ParentObject   string         `json:"parent_object"`
	CreatedAt      string         `json:"created_at"`
	EntryValues    map[string]any `json:"entry_values"`
}

type ListEntryID struct {
	WorkspaceID string `json:"workspace_id"`
	ListID      string `json:"list_id"`
	EntryID     string `json:"entry_id"`
}

func (c *Client) CreateListEntry(ctx context.Context, list string, write ListEntryWrite) (*ListEntryResult, error) {
	result, err := c.writeListEntry(ctx, "create list entry", "POST", list, write)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) AssertListEntry(ctx context.Context, list string, write ListEntryWrite) (*ListEntryResult, error) {
	result, err := c.writeListEntry(ctx, "assert list entry", "PUT", list, write)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) writeListEntry(ctx context.Context, action, method, list string, write ListEntryWrite) (*ListEntryResult, error) {
	if strings.TrimSpace(list) == "" {
		return nil, fmt.Errorf("list is required")
	}
	if strings.TrimSpace(write.ParentObject) == "" {
		return nil, fmt.Errorf("parent object is required")
	}
	if strings.TrimSpace(write.ParentRecordID) == "" {
		return nil, fmt.Errorf("parent record ID is required")
	}

	var response listEntryResponse
	payload := listEntryRequest{
		Data: listEntryRequestData{
			ParentRecordID: write.ParentRecordID,
			ParentObject:   write.ParentObject,
			EntryValues:    write.EntryValues,
		},
	}

	path := "/lists/" + url.PathEscape(list) + "/entries"
	if err := c.writeJSON(ctx, method, path, payload, &response); err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}

	return &ListEntryResult{
		Entry:   response.Data.ListEntry,
		Outcome: firstNonEmpty(response.Data.Status, response.Data.Outcome, response.Data.Operation),
		Created: response.Data.Created,
	}, nil
}
