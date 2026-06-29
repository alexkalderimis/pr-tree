// Package tree holds the pr-tree domain model and pure logic for building,
// filtering, and shaping the forest of pull requests. It performs no I/O.
package tree

// State is the rendered status of a pull request.
type State string

const (
	StateOpen   State = "OPEN"
	StateDraft  State = "DRAFT"
	StateMerged State = "MERGED"
	StateClosed State = "CLOSED"
)

// IsLive reports whether a PR's head branch can be assumed to still exist,
// and so participate in branch-topology linking. Merged/closed branches are
// typically deleted.
func (s State) IsLive() bool {
	return s == StateOpen || s == StateDraft
}

// ReviewDecision is GitHub's aggregate review decision for a PR.
type ReviewDecision string

// ReviewApproved means the PR has received the reviews required to merge.
const ReviewApproved ReviewDecision = "APPROVED"

// PullRequest is the domain model used across pr-tree.
type PullRequest struct {
	Number         int
	Title          string
	State          State
	Author         string         // author login
	Reviewers      []string       // requested reviewer logins
	BaseRef        string         // branch this PR merges into
	HeadRef        string         // this PR's branch
	HeadOID        string         // commit OID at the head of this PR's branch
	Body           string         // PR description (parsed for upstream links)
	ReviewDecision ReviewDecision // GitHub review decision (e.g. APPROVED)
}

// Node is a PR positioned within the forest, with its child PRs.
type Node struct {
	PR       PullRequest
	Children []*Node
}
