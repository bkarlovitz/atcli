package attio

import (
	"context"
	"fmt"
	"net/url"
	"strings"
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

type assertRecordResponse struct {
	Data assertRecordData `json:"data"`
}

type assertRecordData struct {
	Record
	Status    string `json:"status"`
	Outcome   string `json:"outcome"`
	Operation string `json:"operation"`
	Created   *bool  `json:"created"`
}

type AssertRecordResult struct {
	Record  Record
	Outcome string
	Created *bool
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

func (c *Client) AssertRecord(ctx context.Context, object, matchingAttribute string, values map[string]any) (*AssertRecordResult, error) {
	if strings.TrimSpace(object) == "" {
		return nil, fmt.Errorf("object is required")
	}
	if strings.TrimSpace(matchingAttribute) == "" {
		return nil, fmt.Errorf("matching attribute is required")
	}

	var response assertRecordResponse
	payload := createRecordRequest{
		Data: createRecordData{
			Values: values,
		},
	}

	query := url.Values{}
	query.Set("matching_attribute", matchingAttribute)
	path := "/objects/" + url.PathEscape(object) + "/records?" + query.Encode()
	if err := c.putJSON(ctx, path, payload, &response); err != nil {
		return nil, fmt.Errorf("assert record: %w", err)
	}

	result := &AssertRecordResult{
		Record:  response.Data.Record,
		Outcome: firstNonEmpty(response.Data.Status, response.Data.Outcome, response.Data.Operation),
		Created: response.Data.Created,
	}
	return result, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
