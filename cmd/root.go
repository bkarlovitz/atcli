package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:           "atcli",
	Short:         "A small CLI for Attio",
	SilenceErrors: true,
	SilenceUsage:  true,
}

func Execute() error {
	return rootCmd.Execute()
}
