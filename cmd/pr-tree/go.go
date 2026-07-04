package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/alexkalderimis/pr-tree/internal/config"
	"github.com/alexkalderimis/pr-tree/internal/git"
	"github.com/alexkalderimis/pr-tree/internal/github"
	"github.com/alexkalderimis/pr-tree/internal/tree"
)

// resolution is the outcome of dispatching a direction. When candidates is
// non-empty a choice is required; otherwise branch (and prNum, 0 for the
// default branch) is the destination.
type resolution struct {
	branch     string
	prNum      int // 0 => default branch
	candidates []*tree.Node
	prompt     string
}

// resolve maps a direction and the current node to a destination or a choice.
func resolve(direction string, forest []*tree.Node, cur *tree.Node, defaultBranch string) (resolution, error) {
	switch direction {
	case "up":
		if p := tree.Parent(forest, cur.PR.Number); p != nil {
			return resolution{branch: p.PR.HeadRef, prNum: p.PR.Number}, nil
		}
		if defaultBranch == "" {
			return resolution{}, fmt.Errorf("cannot determine the default branch to move up to")
		}
		return resolution{branch: defaultBranch}, nil
	case "down":
		switch len(cur.Children) {
		case 0:
			return resolution{}, fmt.Errorf("#%d has no open child PRs to descend into", cur.PR.Number)
		case 1:
			c := cur.Children[0]
			return resolution{branch: c.PR.HeadRef, prNum: c.PR.Number}, nil
		default:
			return resolution{candidates: cur.Children, prompt: "Descend into which child?"}, nil
		}
	case "root":
		r := tree.Root(forest, cur.PR.Number)
		return resolution{branch: r.PR.HeadRef, prNum: r.PR.Number}, nil
	case "leaf":
		leaves := tree.Leaves(cur)
		if len(leaves) == 1 {
			l := leaves[0]
			return resolution{branch: l.PR.HeadRef, prNum: l.PR.Number}, nil
		}
		return resolution{candidates: leaves, prompt: "Go to which leaf?"}, nil
	}
	return resolution{}, fmt.Errorf("unknown direction %q", direction)
}

// matchCurrentPR returns the live PR whose head branch is checked out.
func matchCurrentPR(branch string, prs []tree.PullRequest) (tree.PullRequest, bool) {
	for _, pr := range prs {
		if pr.State.IsLive() && pr.HeadRef == branch {
			return pr, true
		}
	}
	return tree.PullRequest{}, false
}

// printNodes writes a header and a 1-based numbered list of PRs.
func printNodes(out io.Writer, header string, nodes []*tree.Node) {
	fmt.Fprintln(out, header)
	for i, n := range nodes {
		fmt.Fprintf(out, "  %d) #%d  %s\n", i+1, n.PR.Number, n.PR.Title)
	}
}

// chooseNode prints a numbered list and reads a 1..N selection from in,
// re-prompting on invalid input. Returns an error if the input ends without a
// valid choice.
func chooseNode(in io.Reader, out io.Writer, prompt string, nodes []*tree.Node) (*tree.Node, error) {
	printNodes(out, prompt, nodes)
	r := bufio.NewReader(in)
	for {
		fmt.Fprint(out, "> ")
		line, err := r.ReadString('\n')
		choice, cerr := strconv.Atoi(strings.TrimSpace(line))
		if cerr == nil && choice >= 1 && choice <= len(nodes) {
			return nodes[choice-1], nil
		}
		if err != nil {
			return nil, fmt.Errorf("no selection made")
		}
		fmt.Fprintf(out, "please enter a number between 1 and %d\n", len(nodes))
	}
}

func newGoCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "go <up|down|root|leaf>",
		Short: "Navigate the PR tree by checking out a related branch",
		Long: "Check out a branch by its position relative to the current PR:\n" +
			"  up    the parent PR's branch (or the default branch if it has no live parent)\n" +
			"  down  a child PR's branch (prompts when there is more than one)\n" +
			"  root  the nearest unmerged root of the current tree\n" +
			"  leaf  the end of the current sequence (prompts when there is more than one leaf)\n\n" +
			"The current branch must be an open PR's head branch. Aborts if the " +
			"working tree has uncommitted changes.",
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"up", "down", "root", "leaf"},
		RunE: func(cmd *cobra.Command, args []string) error {
			interactive := term.IsTerminal(int(os.Stdin.Fd()))
			return runGo(cmd.Context(), repoFlag, args[0], dryRun, interactive, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Print the target branch instead of checking out")
	return cmd
}

// describe names a resolution's destination for user-facing messages.
func describe(res resolution) string {
	if res.prNum == 0 {
		return res.branch
	}
	return fmt.Sprintf("#%d (%s)", res.prNum, res.branch)
}

func runGo(ctx context.Context, repoFlag, direction string, dryRun, interactive bool, in io.Reader, out io.Writer) error {
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
	branch, err := g.CurrentBranch()
	if err != nil {
		return fmt.Errorf("inferring current branch: %w", err)
	}
	curPR, ok := matchCurrentPR(branch, prs)
	if !ok {
		return fmt.Errorf("not on a branch with an open PR (current: %s) — check out a PR's branch first", branch)
	}

	forest := tree.BuildForest(prs, defaultBranch)
	cur := tree.Find(forest, curPR.Number)
	if cur == nil {
		return fmt.Errorf("PR #%d is not in the current PR forest", curPR.Number)
	}

	res, err := resolve(direction, forest, cur, defaultBranch)
	if err != nil {
		return err
	}

	if len(res.candidates) > 0 {
		if dryRun {
			printNodes(out, "would prompt to choose among:", res.candidates)
			return nil
		}
		if !interactive {
			printNodes(out, "multiple candidates and stdin is not a terminal; check out one of:", res.candidates)
			return fmt.Errorf("cannot prompt for a choice")
		}
		chosen, err := chooseNode(in, out, res.prompt, res.candidates)
		if err != nil {
			return err
		}
		res.branch = chosen.PR.HeadRef
		res.prNum = chosen.PR.Number
	}

	if dryRun {
		fmt.Fprintln(out, res.branch)
		return nil
	}
	if res.branch == branch {
		fmt.Fprintf(out, "already on %s\n", describe(res))
		return nil
	}
	clean, err := g.WorktreeClean()
	if err != nil {
		return err
	}
	if !clean {
		return fmt.Errorf("working tree has uncommitted changes; commit or stash before switching branches")
	}
	if err := g.Checkout(res.branch); err != nil {
		return err
	}
	fmt.Fprintf(out, "Switched to %s\n", describe(res))
	return nil
}
