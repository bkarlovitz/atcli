package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(newListsCommand())
}

func newListsCommand() *cobra.Command {
	listsCmd := &cobra.Command{
		Use:   "lists",
		Short: "Inspect Attio lists",
	}
	listsCmd.AddCommand(newListsListCommand())
	listsCmd.AddCommand(newListsAttributesCommand())
	return listsCmd
}

func newListsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Attio lists",
		RunE:  runListsList,
	}
}

func runListsList(cmd *cobra.Command, _ []string) error {
	client, err := loadAttioClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
	defer cancel()

	lists, err := client.ListLists(ctx)
	if err != nil {
		return fmt.Errorf("could not list lists: %w", err)
	}

	printLists(cmd.OutOrStdout(), lists)
	return nil
}

func newListsAttributesCommand() *cobra.Command {
	var includeArchived bool
	attributesCmd := &cobra.Command{
		Use:   "attributes <list>",
		Short: "List attributes for an Attio list",
		Long: strings.TrimSpace(`
List attributes for an Attio list.

The <list> argument is an Attio list slug or ID. List attributes belong to
list entries, which are separate from the parent record's object attributes.
`),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("accepts 1 arg, received %d", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runListsAttributes(cmd, args[0], includeArchived)
		},
	}
	attributesCmd.Flags().BoolVar(&includeArchived, "all", false, "include archived attributes")
	return attributesCmd
}

func runListsAttributes(cmd *cobra.Command, list string, includeArchived bool) error {
	client, err := loadAttioClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
	defer cancel()

	attributes, err := client.ListListAttributes(ctx, list, includeArchived)
	if err != nil {
		return fmt.Errorf("could not list list attributes: %w", err)
	}

	printAttributes(cmd.OutOrStdout(), visibleAttributes(attributes, includeArchived))
	return nil
}
