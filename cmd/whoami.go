package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"atcli/internal/attio"
	"atcli/internal/auth"

	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the authenticated Attio workspace and member",
	RunE:  runWhoami,
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}

func runWhoami(cmd *cobra.Command, _ []string) error {
	token, err := auth.LoadToken()
	if err != nil {
		if errors.Is(err, auth.ErrNotAuthenticated) {
			return errors.New("not authenticated; run `atcli auth` or set ATTIO_ACCESS_TOKEN")
		}
		if errors.Is(err, auth.ErrCredentialStoreUnavailable) {
			return errors.New("could not read the OS credential store; unlock it, run `atcli auth`, or set ATTIO_ACCESS_TOKEN")
		}
		return fmt.Errorf("%w; run `atcli auth` or set ATTIO_ACCESS_TOKEN", err)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
	defer cancel()

	info, err := auth.Introspect(ctx, token)
	if err != nil {
		return fmt.Errorf("could not introspect token: %w", err)
	}
	if !info.Active {
		return errors.New("Attio reported that token is inactive; run `atcli auth` with a fresh token")
	}

	var member *attio.WorkspaceMember
	var memberErr error
	if info.AuthorizedByWorkspaceMemberID != "" {
		member, memberErr = attio.NewClient(token).GetWorkspaceMember(ctx, info.AuthorizedByWorkspaceMemberID)
	}

	printWhoami(cmd.OutOrStdout(), info, member, memberErr)
	return nil
}

func printWhoami(out io.Writer, info *auth.Introspection, member *attio.WorkspaceMember, memberErr error) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "Authenticated with Attio")
	_, _ = fmt.Fprintln(w)
	printRow(w, "Workspace:", info.WorkspaceName)
	printRow(w, "Slug:", info.WorkspaceSlug)
	printRow(w, "ID:", info.WorkspaceID)
	printRow(w, "Token type:", info.TokenType)
	printRow(w, "Scopes:", scopeDisplay(info.Scope))

	if info.AuthorizedByWorkspaceMemberID == "" {
		_ = w.Flush()
		return
	}

	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Authorized by workspace member:")
	printRow(w, "ID:", info.AuthorizedByWorkspaceMemberID)

	if member != nil {
		printRow(w, "Name:", memberName(member))
		printRow(w, "Email:", member.EmailAddress)
		printRow(w, "Access:", member.AccessLevel)
		_ = w.Flush()
		return
	}

	if memberErr != nil {
		if attio.IsPermissionError(memberErr) {
			printRow(w, "Details:", "unavailable; token needs user_management:read")
		} else {
			printRow(w, "Details:", "unavailable; "+memberErr.Error())
		}
	}

	_ = w.Flush()
}

func printRow(w io.Writer, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	_, _ = fmt.Fprintf(w, "%s\t%s\n", label, value)
}

func scopeDisplay(scope string) string {
	if strings.TrimSpace(scope) == "" {
		return ""
	}
	return strings.Join(strings.Fields(scope), ", ")
}

func memberName(member *attio.WorkspaceMember) string {
	return strings.TrimSpace(strings.Join([]string{member.FirstName, member.LastName}, " "))
}
