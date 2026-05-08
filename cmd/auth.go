package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"atcli/internal/auth"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type authOptions struct {
	noValidate bool
	tokenStdin bool
}

var authOpts authOptions

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with Attio",
	Long: strings.TrimSpace(`
Authenticate with Attio by storing a workspace API key or OAuth access token.

For personal CLI usage, generate an API key in Attio's developer settings,
then paste it here. atcli stores the token in your OS credential store.
`),
	RunE: runAuth,
}

func init() {
	authCmd.Flags().BoolVar(&authOpts.noValidate, "no-validate", false, "store the token without checking it against Attio")
	authCmd.Flags().BoolVar(&authOpts.tokenStdin, "token-stdin", false, "read the Attio token from stdin")
	rootCmd.AddCommand(authCmd)
}

func runAuth(cmd *cobra.Command, _ []string) error {
	token, err := readToken(cmd.OutOrStdout(), cmd.InOrStdin(), authOpts.tokenStdin)
	if err != nil {
		return err
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("token cannot be empty")
	}

	var tokenInfo *auth.Introspection
	if !authOpts.noValidate {
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()

		info, err := auth.Introspect(ctx, token)
		if err != nil {
			return fmt.Errorf("could not validate token: %w", err)
		}
		if !info.Active {
			return errors.New("Attio reported that token is inactive")
		}
		tokenInfo = info
	}

	if err := auth.StoreToken(token); err != nil {
		if errors.Is(err, auth.ErrCredentialStoreUnavailable) {
			return errors.New("could not write to the OS credential store; unlock it or set ATTIO_ACCESS_TOKEN for shell-based auth")
		}
		return err
	}

	if tokenInfo != nil && tokenInfo.WorkspaceName != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Authenticated with Attio workspace %q.\n", tokenInfo.WorkspaceName)
		return nil
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Authenticated with Attio.")
	return nil
}

func readToken(out io.Writer, in io.Reader, fromStdin bool) (string, error) {
	if fromStdin {
		token, err := io.ReadAll(in)
		if err != nil {
			return "", fmt.Errorf("read token from stdin: %w", err)
		}
		return string(token), nil
	}

	stdin, ok := in.(*os.File)
	if !ok || !term.IsTerminal(int(stdin.Fd())) {
		return "", errors.New("stdin is not interactive; pass --token-stdin to read a token from stdin")
	}

	_, _ = fmt.Fprint(out, "Paste your Attio access token/API key: ")
	tokenBytes, err := term.ReadPassword(int(stdin.Fd()))
	_, _ = fmt.Fprintln(out)
	if err != nil {
		return "", fmt.Errorf("read token: %w", err)
	}

	return string(tokenBytes), nil
}
