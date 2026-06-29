// Package git wraps the local git commands that `replant` needs to plan and
// (eventually) perform stack rebases. It shells out to the `git` binary, the
// same approach used by internal/config and internal/github for `git` and `gh`.
package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// Git runs git commands in a working directory. A zero/empty dir uses the
// process's current directory.
type Git struct {
	dir string
}

// New returns a Git rooted at dir (empty for the current directory).
func New(dir string) *Git {
	return &Git{dir: dir}
}

// Commit is a single commit in a range: its full OID and subject line.
type Commit struct {
	OID     string
	Subject string
}

// run executes `git <args...>` in g.dir and returns trimmed stdout. On failure
// it surfaces git's stderr, which carries the actionable message.
func (g *Git) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(string(out)), nil
}

// CurrentBranch returns the name of the checked-out branch.
func (g *Git) CurrentBranch() (string, error) {
	return g.run("rev-parse", "--abbrev-ref", "HEAD")
}

// RevParse resolves a ref or OID to its full commit OID.
func (g *Git) RevParse(ref string) (string, error) {
	return g.run("rev-parse", ref)
}

// MergeBase returns the best common ancestor of a and b — the fork point a
// child branch shares with its parent's pre-rebase head.
func (g *Git) MergeBase(a, b string) (string, error) {
	return g.run("merge-base", a, b)
}

// RevList returns the commits in a range expression (e.g. "a..b"), newest
// first, each with its OID and subject.
func (g *Git) RevList(rangeExpr string) ([]Commit, error) {
	// %H<tab>%s, NUL-terminated records so subjects may contain anything.
	out, err := g.run("log", "--format=%H%x09%s", "-z", rangeExpr)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	var commits []Commit
	for _, rec := range strings.Split(out, "\x00") {
		rec = strings.Trim(rec, "\n")
		if rec == "" {
			continue
		}
		oid, subject, _ := strings.Cut(rec, "\t")
		commits = append(commits, Commit{OID: oid, Subject: subject})
	}
	return commits, nil
}

// FetchOID fetches a single commit OID from remote, ensuring the object is
// present locally even after the source branch has been deleted.
func (g *Git) FetchOID(remote, oid string) error {
	_, err := g.run("fetch", "--quiet", remote, oid)
	return err
}

// WorktreeClean reports whether the working tree has no uncommitted changes.
func (g *Git) WorktreeClean() (bool, error) {
	out, err := g.run("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out == "", nil
}

// Fetch updates remote-tracking refs from remote.
func (g *Git) Fetch(remote string) error {
	_, err := g.run("fetch", "--quiet", remote)
	return err
}

// IsAncestor reports whether ancestor is an ancestor of (or equal to) descendant.
func (g *Git) IsAncestor(ancestor, descendant string) (bool, error) {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", ancestor, descendant)
	cmd.Dir = g.dir
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git merge-base --is-ancestor %s %s: %w", ancestor, descendant, err)
}

// AlreadyReplanted reports whether branch already sits on newBaseOID with the
// old parent commits (oldParentOID) shed — i.e. the replant for it is done.
func (g *Git) AlreadyReplanted(newBaseOID, oldParentOID, branch string) (bool, error) {
	onNewBase, err := g.IsAncestor(newBaseOID, branch)
	if err != nil || !onNewBase {
		return false, err
	}
	hasOld, err := g.IsAncestor(oldParentOID, branch)
	if err != nil {
		return false, err
	}
	return !hasOld, nil
}

// LocalBranchOID returns the OID of a local branch, and whether it exists.
func (g *Git) LocalBranchOID(branch string) (string, bool, error) {
	ref := "refs/heads/" + branch
	check := exec.Command("git", "show-ref", "--verify", "--quiet", ref)
	check.Dir = g.dir
	if err := check.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return "", false, nil
		}
		return "", false, fmt.Errorf("git show-ref %s: %w", ref, err)
	}
	oid, err := g.run("rev-parse", ref)
	if err != nil {
		return "", false, err
	}
	return oid, true, nil
}

// PrepareBranch ensures the local branch exists and points at headOID, ready to
// rebase. It refuses to touch a branch that has commits not contained in
// headOID (unpushed local work or a divergence) to avoid destroying work.
func (g *Git) PrepareBranch(branch, headOID string) error {
	oid, exists, err := g.LocalBranchOID(branch)
	if err != nil {
		return err
	}
	if !exists {
		_, err := g.run("branch", branch, headOID)
		return err
	}
	if oid == headOID {
		return nil
	}
	ahead, err := g.IsAncestor(headOID, branch) // headOID contained in branch => branch ahead
	if err != nil {
		return err
	}
	if ahead {
		return fmt.Errorf("branch %q has local commits not on origin; push or reset it before replanting", branch)
	}
	behind, err := g.IsAncestor(branch, headOID) // branch contained in headOID => safe to fast-forward
	if err != nil {
		return err
	}
	if !behind {
		return fmt.Errorf("branch %q has diverged from origin; reconcile it before replanting", branch)
	}
	if err := g.Checkout(branch); err != nil {
		return err
	}
	return g.ResetHard(headOID)
}

// Checkout switches to a ref (branch or OID).
func (g *Git) Checkout(ref string) error {
	_, err := g.run("checkout", "--quiet", ref)
	return err
}

// ResetHard moves the current branch and worktree to oid.
func (g *Git) ResetHard(oid string) error {
	_, err := g.run("reset", "--hard", "--quiet", oid)
	return err
}
