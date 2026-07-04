package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

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
