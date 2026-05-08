package attio

import (
	"context"
	"fmt"
	"net/url"
)

type WorkspaceMemberResponse struct {
	Data WorkspaceMember `json:"data"`
}

type WorkspaceMember struct {
	ID           WorkspaceMemberID `json:"id"`
	FirstName    string            `json:"first_name"`
	LastName     string            `json:"last_name"`
	EmailAddress string            `json:"email_address"`
	AccessLevel  string            `json:"access_level"`
}

type WorkspaceMemberID struct {
	WorkspaceMemberID string `json:"workspace_member_id"`
}

func (c *Client) GetWorkspaceMember(ctx context.Context, workspaceMemberID string) (*WorkspaceMember, error) {
	var response WorkspaceMemberResponse
	path := "/workspace_members/" + url.PathEscape(workspaceMemberID)
	if err := c.getJSON(ctx, path, &response); err != nil {
		return nil, fmt.Errorf("get workspace member: %w", err)
	}
	return &response.Data, nil
}
