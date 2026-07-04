package main

import "github.com/spf13/cobra"

// repoFlag holds the value of the persistent --repo flag (owner/name).
var repoFlag string

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "pr-tree",
		Short: "Visualize GitHub pull-requests as a tree",
	}
	root.PersistentFlags().StringVar(&repoFlag, "repo", "", "Target repo as owner/name (defaults to the current git origin)")
	root.AddCommand(newListCmd())
	root.AddCommand(newReplantCmd())
	root.AddCommand(newAnnotateCmd())
	root.AddCommand(newGoCmd())
	return root
}
