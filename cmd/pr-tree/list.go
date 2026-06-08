package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/alexkalderimis/pr-tree/internal/config"
	"github.com/alexkalderimis/pr-tree/internal/github"
	"github.com/alexkalderimis/pr-tree/internal/render"
	"github.com/alexkalderimis/pr-tree/internal/tree"
)

func newListCmd() *cobra.Command {
	var mine, toReview, noColor bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List PRs as trees",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TTY detection probes os.Stdout directly (not cmd.OutOrStdout());
			// in normal use they are the same, and this matches typical CLI
			// behavior of basing color on the process's real stdout.
			color := colorEnabled(noColor, os.Getenv("NO_COLOR"), term.IsTerminal(int(os.Stdout.Fd())))
			return runList(cmd.Context(), repoFlag, mine, toReview, color, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&mine, "mine", false, "Only show trees containing PRs you authored")
	cmd.Flags().BoolVar(&toReview, "to-review", false, "Only show trees containing PRs awaiting your review")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	return cmd
}

func runList(ctx context.Context, repoFlag string, mine, toReview, color bool, out io.Writer) error {
	repo, err := config.Resolve(repoFlag)
	if err != nil {
		return err
	}
	token, err := github.Token()
	if err != nil {
		return err
	}
	client := github.New(token)

	viewer, err := client.Viewer(ctx)
	if err != nil {
		return fmt.Errorf("looking up authenticated user: %w", err)
	}

	prs, defaultBranch, err := client.FetchOpenPRs(ctx, repo)
	if err != nil {
		return fmt.Errorf("fetching PRs for %s: %w", repo, err)
	}

	// Resolve merged/closed parents referenced by upstream links but not in the
	// open set, so they can anchor trees (the only way to place merged nodes).
	prs = append(prs, fetchMissingParents(ctx, client, repo, prs)...)

	forest := tree.BuildForest(prs, defaultBranch)
	filter := tree.Filter{Mine: mine, ToReview: toReview, Viewer: viewer}
	selected := tree.SelectTrees(forest, filter)
	pending := tree.ReviewPending(forest, filter)

	text := render.Render(selected, render.Options{ReviewPending: pending, Color: color})
	_, err = out.Write([]byte(text))
	return err
}

// fetchMissingParents looks up PRs referenced via `upstream:` links that are
// not already present (typically merged parents). Errors are non-fatal.
func fetchMissingParents(ctx context.Context, client *github.Client, repo config.Repo, prs []tree.PullRequest) []tree.PullRequest {
	have := make(map[int]bool, len(prs))
	for _, pr := range prs {
		have[pr.Number] = true
	}
	var missing []int
	seen := make(map[int]bool)
	for _, pr := range prs {
		if up := tree.ParseUpstream(pr.Body); up != 0 && !have[up] && !seen[up] {
			seen[up] = true
			missing = append(missing, up)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	extra, err := client.FetchPRsByNumber(ctx, repo, missing)
	if err != nil {
		return nil // best-effort; un-resolvable parents just become roots
	}
	return extra
}
