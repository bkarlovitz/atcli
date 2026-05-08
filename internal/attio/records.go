package attio

import (
	"context"
	"fmt"
	"net/url"
)

type createRecordRequest struct {
	Data createRecordData `json:"data"`
}

type createRecordData struct {
	Values map[string]any `json:"values"`
}

type createRecordResponse struct {
	Data Record `json:"data"`
}

type Record struct {
	ID        RecordID       `json:"id"`
	CreatedAt string         `json:"created_at"`
	WebURL    string         `json:"web_url"`
	Values    map[string]any `json:"values"`
}

type RecordID struct {
	WorkspaceID string `json:"workspace_id"`
	ObjectID    string `json:"object_id"`
	RecordID    string `json:"record_id"`
}

func (c *Client) CreateRecord(ctx context.Context, object string, values map[string]any) (*Record, error) {
	var response createRecordResponse
	payload := createRecordRequest{
		Data: createRecordData{
			Values: values,
		},
	}
	path := "/objects/" + url.PathEscape(object) + "/records"
	if err := c.postJSON(ctx, path, payload, &response); err != nil {
		return nil, fmt.Errorf("create record: %w", err)
	}
	return &response.Data, nil
}
