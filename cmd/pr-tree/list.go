package main

import "github.com/spf13/cobra"

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List PRs as trees",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
}
