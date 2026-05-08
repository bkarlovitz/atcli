package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(newObjectsCommand())
}

func newObjectsCommand() *cobra.Command {
	objectsCmd := &cobra.Command{
		Use:   "objects",
		Short: "Inspect Attio objects",
	}
	objectsCmd.AddCommand(newObjectsListCommand())
	objectsCmd.AddCommand(newObjectsAttributesCommand())
	return objectsCmd
}

func newObjectsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Attio objects",
		RunE:  runObjectsList,
	}
}

func runObjectsList(cmd *cobra.Command, _ []string) error {
	client, err := loadAttioClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
	defer cancel()

	objects, err := client.ListObjects(ctx)
	if err != nil {
		return fmt.Errorf("could not list objects: %w", err)
	}

	printObjects(cmd.OutOrStdout(), objects)
	return nil
}

func newObjectsAttributesCommand() *cobra.Command {
	var includeArchived bool
	attributesCmd := &cobra.Command{
		Use:   "attributes <object>",
		Short: "List attributes for an Attio object",
		Long: strings.TrimSpace(`
List attributes for an Attio object.

The <object> argument is an Attio object slug or ID. For standard objects,
Attio slugs are usually plural, such as "people" and "companies".
`),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("accepts 1 arg, received %d", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runObjectsAttributes(cmd, args[0], includeArchived)
		},
	}
	attributesCmd.Flags().BoolVar(&includeArchived, "all", false, "include archived attributes")
	return attributesCmd
}

func runObjectsAttributes(cmd *cobra.Command, object string, includeArchived bool) error {
	client, err := loadAttioClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
	defer cancel()

	attributes, err := client.ListObjectAttributes(ctx, object, includeArchived)
	if err != nil {
		return fmt.Errorf("could not list object attributes: %w", err)
	}

	printAttributes(cmd.OutOrStdout(), visibleAttributes(attributes, includeArchived))
	return nil
}
