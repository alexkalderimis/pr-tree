package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/alexkalderimis/pr-tree/internal/annotate"
	"github.com/alexkalderimis/pr-tree/internal/config"
	"github.com/alexkalderimis/pr-tree/internal/git"
	"github.com/alexkalderimis/pr-tree/internal/github"
	"github.com/alexkalderimis/pr-tree/internal/render"
	"github.com/alexkalderimis/pr-tree/internal/tree"
)

func newAnnotateCmd() *cobra.Command {
	var apply, yes, noColor bool
	cmd := &cobra.Command{
		Use:   "annotate [#PR]",
		Short: "Upsert upstream/downstream links into a tree's PR descriptions (dry-run unless --apply)",
		Long: "Upsert a machine-managed links block (immediate upstream and " +
			"downstream PRs) into the description of every PR in the target's " +
			"whole tree, so the stack is navigable within GitHub. The target " +
			"defaults to the PR for the current branch. Without --apply it prints " +
			"a coloured diff of the changes it would make; with --apply it writes " +
			"the descriptions.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			color := colorEnabled(noColor, os.Getenv("NO_COLOR"), term.IsTerminal(int(os.Stdout.Fd())))
			return runAnnotate(cmd.Context(), repoFlag, args, apply, yes, color, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "Update PR descriptions (default: dry-run)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the update confirmation prompt")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	return cmd
}

// annotateItems maps plan updates into render view items, classifying each
// change as insert (old body had no block) or update (it did).
func annotateItems(updates []annotate.Update) []render.AnnotateItem {
	items := make([]render.AnnotateItem, len(updates))
	for i, u := range updates {
		change := render.ChangeNone
		if u.Changed {
			if annotate.HasBlock(u.OldBody) {
				change = render.ChangeUpdate
			} else {
				change = render.ChangeInsert
			}
		}
		items[i] = render.AnnotateItem{
			Number:  u.PR.Number,
			Title:   u.PR.Title,
			State:   u.PR.State,
			Change:  change,
			OldBody: u.OldBody,
			NewBody: u.NewBody,
		}
	}
	return items
}

func runAnnotate(ctx context.Context, repoFlag string, args []string, apply, yes, color bool, in io.Reader, out io.Writer) error {
	repo, err := config.Resolve(repoFlag)
	if err != nil {
		return err
	}
	token, err := github.Token()
	if err != nil {
		return err
	}
	client := github.New(token)

	prs, defaultBranch, err := client.FetchOpenPRs(ctx, repo)
	if err != nil {
		return fmt.Errorf("fetching PRs for %s: %w", repo, err)
	}

	g := git.New("")
	target, err := resolveTarget(g, args, prs)
	if err != nil {
		return err
	}
	prs = append(prs, fetchMissingParents(ctx, client, repo, prs)...)

	forest := tree.BuildForest(prs, defaultBranch)
	working := tree.WholeTree(forest, target)
	if working == nil {
		return fmt.Errorf("no PR #%d found in the current PR forest", target)
	}

	updates := annotate.Plan(working)
	fmt.Fprintf(out, "Annotate plan for the tree containing #%d:\n\n", target)
	fmt.Fprint(out, render.AnnotatePlan(annotateItems(updates), render.Options{Color: color}))

	var changed []annotate.Update
	for _, u := range updates {
		if u.Changed {
			changed = append(changed, u)
		}
	}

	if !apply {
		if len(changed) == 0 {
			fmt.Fprintln(out, "(all descriptions already current)")
		} else {
			fmt.Fprintln(out, "(dry-run: no PR descriptions were changed — pass --apply to write)")
		}
		return nil
	}

	if len(changed) == 0 {
		fmt.Fprintln(out, "Nothing to annotate: all descriptions already current.")
		return nil
	}
	if !yes {
		fmt.Fprintf(out, "Update %d PR description(s)? [y/N] ", len(changed))
		if !confirmed(in) {
			fmt.Fprintln(out, "Aborted; no descriptions were changed.")
			return nil
		}
	}
	for _, u := range changed {
		if err := client.UpdatePullRequestBody(ctx, u.PR.NodeID, u.NewBody); err != nil {
			return fmt.Errorf("updating #%d: %w", u.PR.Number, err)
		}
		fmt.Fprintf(out, "  updated #%d\n", u.PR.Number)
	}
	fmt.Fprintln(out, "\nAnnotate complete.")
	return nil
}
