package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// run executes git in dir and fails the test on error. Test-only helper for
// building fixture repositories.
func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// commit writes a file and commits it on the current branch, returning nothing;
// callers read OIDs back through the package under test.
func commit(t *testing.T, dir, file, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", file)
	run(t, dir, "commit", "-m", msg)
}

// initRepo creates a repo with this shape:
//
//	main: c0
//	a:    c0 - a1 - a2      (branched from main)
//	b:    c0 - a1 - a2 - b1 (branched from a)
//
// so merge-base(a, b) == tip of a, and a..b == [b1].
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	run(t, dir, "config", "user.email", "test@example.com")
	run(t, dir, "config", "user.name", "Test")
	commit(t, dir, "f", "c0", "c0")

	run(t, dir, "checkout", "-q", "-b", "a")
	commit(t, dir, "f", "a1", "a1")
	commit(t, dir, "f", "a2", "a2")

	run(t, dir, "checkout", "-q", "-b", "b")
	commit(t, dir, "f", "b1", "b1")

	run(t, dir, "checkout", "-q", "main")
	return dir
}

func TestCurrentBranch(t *testing.T) {
	dir := initRepo(t)
	run(t, dir, "checkout", "-q", "b")
	g := New(dir)

	got, err := g.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if got != "b" {
		t.Fatalf("CurrentBranch = %q, want b", got)
	}
}

func TestMergeBaseIsParentTip(t *testing.T) {
	dir := initRepo(t)
	g := New(dir)

	aTip, err := g.RevParse("a")
	if err != nil {
		t.Fatalf("RevParse(a): %v", err)
	}
	base, err := g.MergeBase("a", "b")
	if err != nil {
		t.Fatalf("MergeBase: %v", err)
	}
	if base != aTip {
		t.Fatalf("MergeBase(a,b) = %q, want a's tip %q", base, aTip)
	}
}

func TestRevListReturnsOwnCommits(t *testing.T) {
	dir := initRepo(t)
	g := New(dir)

	commits, err := g.RevList("a..b")
	if err != nil {
		t.Fatalf("RevList: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("a..b has %d commits, want 1: %+v", len(commits), commits)
	}
	if commits[0].Subject != "b1" {
		t.Fatalf("subject = %q, want b1", commits[0].Subject)
	}
	if len(commits[0].OID) < 7 {
		t.Fatalf("OID looks wrong: %q", commits[0].OID)
	}
}

func TestRevListDroppedRange(t *testing.T) {
	// The "dropped" set after a squash-merge: main..a == [a1, a2].
	dir := initRepo(t)
	g := New(dir)

	commits, err := g.RevList("main..a")
	if err != nil {
		t.Fatalf("RevList: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("main..a has %d commits, want 2: %+v", len(commits), commits)
	}
}

func TestWorktreeClean(t *testing.T) {
	dir := initRepo(t)
	g := New(dir)

	clean, err := g.WorktreeClean()
	if err != nil {
		t.Fatalf("WorktreeClean: %v", err)
	}
	if !clean {
		t.Fatal("fresh checkout should be clean")
	}

	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}
	clean, err = g.WorktreeClean()
	if err != nil {
		t.Fatalf("WorktreeClean: %v", err)
	}
	if clean {
		t.Fatal("modified worktree should be dirty")
	}
}

func TestFetchOID(t *testing.T) {
	// A second repo acts as a remote; fetch one of its ref-tip OIDs by SHA.
	remote := initRepo(t)
	rg := New(remote)
	oid, err := rg.RevParse("a")
	if err != nil {
		t.Fatalf("remote RevParse: %v", err)
	}

	local := t.TempDir()
	run(t, local, "init", "-q", "-b", "main")
	run(t, local, "config", "user.email", "test@example.com")
	run(t, local, "config", "user.name", "Test")
	g := New(local)

	if err := g.FetchOID(remote, oid); err != nil {
		t.Fatalf("FetchOID: %v", err)
	}
	got, err := g.RevParse(oid)
	if err != nil {
		t.Fatalf("object not present after fetch: %v", err)
	}
	if got != oid {
		t.Fatalf("RevParse(%s) = %s", oid, got)
	}
}
