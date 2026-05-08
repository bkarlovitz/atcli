package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"atcli/internal/attio"
	"atcli/internal/auth"
)

func TestPrintWhoamiIncludesWorkspaceAndMember(t *testing.T) {
	var out bytes.Buffer
	printWhoami(&out, &auth.Introspection{
		Scope:                         "record_permission:read user_management:read",
		TokenType:                     "Bearer",
		WorkspaceID:                   "workspace-123",
		WorkspaceName:                 "Acme",
		WorkspaceSlug:                 "acme",
		AuthorizedByWorkspaceMemberID: "member-123",
	}, &attio.WorkspaceMember{
		FirstName:    "Susan",
		LastName:     "Kare",
		EmailAddress: "susan@example.com",
		AccessLevel:  "member",
	}, nil)

	output := out.String()
	assertContains(t, output, "Workspace: Acme")
	assertContains(t, output, "Scopes: record_permission:read, user_management:read")
	assertContains(t, output, "Name: Susan Kare")
	assertContains(t, output, "Email: susan@example.com")
}

func TestPrintWhoamiHandlesMissingMemberPermission(t *testing.T) {
	var out bytes.Buffer
	memberErr := fmt.Errorf("get workspace member: %w", &attio.APIError{
		StatusCode: http.StatusForbidden,
		Status:     "403 Forbidden",
	})

	printWhoami(&out, &auth.Introspection{
		WorkspaceName:                 "Acme",
		AuthorizedByWorkspaceMemberID: "member-123",
	}, nil, memberErr)

	assertContains(t, out.String(), "Details: unavailable; token needs user_management:read")
}

func assertContains(t *testing.T, output, expected string) {
	t.Helper()
	normalizedOutput := strings.Join(strings.Fields(output), " ")
	normalizedExpected := strings.Join(strings.Fields(expected), " ")
	if !strings.Contains(normalizedOutput, normalizedExpected) {
		t.Fatalf("expected output to contain %q, got:\n%s", expected, output)
	}
}
