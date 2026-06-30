package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/alexkalderimis/pr-tree/internal/git"
	"github.com/alexkalderimis/pr-tree/internal/replant"
	"github.com/alexkalderimis/pr-tree/internal/tree"
)

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func commitFile(t *testing.T, dir, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "add", "f")
	mustGit(t, dir, "commit", "-q", "-m", msg)
}

func TestNormalizeSubject(t *testing.T) {
	cases := []struct{ in, want string }{
		// GitHub appends " (#NNNN)" to squash-merge subjects; strip it so the
		// merged commit matches its un-suffixed counterpart on the stack.
		{"feat: route a transfer (#5924)", "feat: route a transfer"},
		{"feat: route a transfer", "feat: route a transfer"},
		{"  spaced out  ", "spaced out"},
		{"trailing space (#12) ", "trailing space"},
		// A bare "(#n)" not at the end is left alone.
		{"revert (#5) and redo", "revert (#5) and redo"},
	}
	for _, c := range cases {
		if got := normalizeSubject(c.in); got != c.want {
			t.Errorf("normalizeSubject(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestInferForkBySubject(t *testing.T) {
	// Candidates are tip-first, mirroring #5925: the top commit is the PR's own
	// work (not in master); the next matches a squash-merged ancestor.
	candidates := []git.Commit{
		{OID: "521a818", Subject: "feat: request & serialization surface"},
		{OID: "55880e1", Subject: "feat: route a User transfer recipient"},
		{OID: "b31a951", Subject: "feat: migrate user data"},
		{OID: "13ced8b", Subject: "feat: default-org foundation"},
	}
	merged := map[string]bool{
		// Master's squash subjects, already normalized.
		"feat: route a User transfer recipient": true,
		"feat: migrate user data":               true,
		"feat: default-org foundation":          true,
	}

	fork, ok := inferForkBySubject(candidates, merged)
	if !ok {
		t.Fatal("expected a fork to be inferred")
	}
	if fork != "55880e1" {
		t.Errorf("fork = %q, want 55880e1 (newest commit already in master)", fork)
	}
}

func TestInferForkBySubject_NoneMerged(t *testing.T) {
	candidates := []git.Commit{
		{OID: "aaa", Subject: "own work 2"},
		{OID: "bbb", Subject: "own work 1"},
	}
	if _, ok := inferForkBySubject(candidates, map[string]bool{"unrelated": true}); ok {
		t.Error("expected no fork when no candidate is in master")
	}
}

func TestInferForkBySubject_SuffixedMasterMatches(t *testing.T) {
	// The merged set is normalized by the caller, but verify a realistic match
	// where the candidate carries no suffix and master's did.
	candidates := []git.Commit{
		{OID: "tip", Subject: "feat: own"},
		{OID: "mid", Subject: "feat: route a transfer"},
	}
	merged := map[string]bool{normalizeSubject("feat: route a transfer (#5924)"): true}
	fork, ok := inferForkBySubject(candidates, merged)
	if !ok || fork != "mid" {
		t.Errorf("fork = %q ok=%v, want mid true", fork, ok)
	}
}

// buildDriftRepo creates the squash-merge + restack shape:
//
//	master: c0 - "parent work (#1)"   (the squash commit)
//	child:  c0 - P("parent work") - Q("own work")
//
// so the parent's squash commit on master is NOT an ancestor of child, and
// child carries a clean P whose subject matches the parent's work.
func buildDriftRepo(t *testing.T) (g *git.Git, pOID, qOID, masterTip string) {
	t.Helper()
	dir := t.TempDir()
	mustGit(t, dir, "init", "-q", "-b", "master")
	mustGit(t, dir, "config", "user.email", "t@example.com")
	mustGit(t, dir, "config", "user.name", "T")
	commitFile(t, dir, "c0", "c0")
	g = git.New(dir)
	c0, err := g.RevParse("HEAD")
	if err != nil {
		t.Fatal(err)
	}

	// child branch off c0 with the clean parent commit P then own commit Q.
	mustGit(t, dir, "checkout", "-q", "-b", "child")
	commitFile(t, dir, "p", "feat: parent work")
	if pOID, err = g.RevParse("HEAD"); err != nil {
		t.Fatal(err)
	}
	commitFile(t, dir, "q", "feat: own work")
	if qOID, err = g.RevParse("HEAD"); err != nil {
		t.Fatal(err)
	}

	// master gets the squash commit (different SHA, "(#1)" suffix).
	mustGit(t, dir, "checkout", "-q", "master")
	mustGit(t, dir, "reset", "-q", "--hard", c0)
	commitFile(t, dir, "squashed", "feat: parent work (#1)")
	if masterTip, err = g.RevParse("HEAD"); err != nil {
		t.Fatal(err)
	}
	return g, pOID, qOID, masterTip
}

func TestResolveFork_DriftInfersBySubject(t *testing.T) {
	g, pOID, qOID, masterTip := buildDriftRepo(t)
	s := replant.Step{TargetSelf: true, ParentMerged: true, ParentPR: 1}
	child := tree.PullRequest{Number: 2, HeadOID: qOID}
	parent := tree.PullRequest{Number: 1, HeadOID: masterTip} // not on child → drift
	merged := subjectSet([]string{"feat: parent work"}, "")

	fork, err := resolveFork(g, "master", s, child, parent, 0, merged)
	if err != nil {
		t.Fatalf("resolveFork: %v", err)
	}
	if fork.OID != pOID {
		t.Errorf("fork OID = %s, want P (%s)", fork.OID, pOID)
	}
	if fork.Method != "matched parent #1's commits" {
		t.Errorf("method = %q, want matched-commits", fork.Method)
	}
}

func TestResolveFork_KeepOverride(t *testing.T) {
	g, pOID, qOID, masterTip := buildDriftRepo(t)
	s := replant.Step{TargetSelf: true, ParentMerged: true, ParentPR: 1}
	child := tree.PullRequest{Number: 2, HeadOID: qOID}
	parent := tree.PullRequest{Number: 1, HeadOID: masterTip}

	// keep 1 → keep only Q, fork at its parent P.
	fork, err := resolveFork(g, "master", s, child, parent, 1, nil)
	if err != nil {
		t.Fatalf("resolveFork: %v", err)
	}
	if fork.OID != pOID {
		t.Errorf("fork OID = %s, want P (%s)", fork.OID, pOID)
	}
	if fork.Method != "--keep 1" {
		t.Errorf("method = %q, want --keep 1", fork.Method)
	}
}

func TestResolveFork_AncestryWhenParentOnBranch(t *testing.T) {
	g, pOID, qOID, _ := buildDriftRepo(t)
	s := replant.Step{TargetSelf: true, ParentPR: 1}
	child := tree.PullRequest{Number: 2, HeadOID: qOID}
	parent := tree.PullRequest{Number: 1, HeadOID: pOID} // P IS on child → no drift

	fork, err := resolveFork(g, "master", s, child, parent, 0, nil)
	if err != nil {
		t.Fatalf("resolveFork: %v", err)
	}
	if fork.OID != pOID {
		t.Errorf("fork OID = %s, want P (%s) via merge-base", fork.OID, pOID)
	}
	if fork.Method != "parent #1 head" {
		t.Errorf("method = %q, want parent head", fork.Method)
	}
}

func TestResolveFork_UnknownWhenNoMatch(t *testing.T) {
	g, _, qOID, masterTip := buildDriftRepo(t)
	s := replant.Step{TargetSelf: true, ParentMerged: true, ParentPR: 1}
	child := tree.PullRequest{Number: 2, HeadOID: qOID}
	parent := tree.PullRequest{Number: 1, HeadOID: masterTip}

	// No subjects match → caller must be told to use --keep.
	_, err := resolveFork(g, "master", s, child, parent, 0, map[string]bool{"unrelated": true})
	var unknown *forkUnknownError
	if !errors.As(err, &unknown) {
		t.Fatalf("expected *forkUnknownError, got %v", err)
	}
}
