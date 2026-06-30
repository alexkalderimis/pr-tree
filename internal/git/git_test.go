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

func TestIsAncestor(t *testing.T) {
	dir := initRepo(t)
	g := New(dir)
	aTip, _ := g.RevParse("a")
	yes, err := g.IsAncestor(aTip, "b")
	if err != nil || !yes {
		t.Fatalf("a should be ancestor of b: %v %v", yes, err)
	}
	no, err := g.IsAncestor("b", "a")
	if err != nil || no {
		t.Fatalf("b should NOT be ancestor of a: %v %v", no, err)
	}
}

func TestAlreadyReplanted(t *testing.T) {
	// Simulate a squash-merge of a into main, then replant b onto main.
	dir := initRepo(t)
	g := New(dir)
	aTip, _ := g.RevParse("a")
	run(t, dir, "checkout", "-q", "main")
	run(t, dir, "merge", "--squash", "a")
	run(t, dir, "commit", "-qm", "squash a")

	// Before replant: b still contains a's commits -> NOT replanted onto main.
	mainTip, _ := g.RevParse("main")
	done, err := g.AlreadyReplanted(mainTip, aTip, "b")
	if err != nil {
		t.Fatalf("AlreadyReplanted: %v", err)
	}
	if done {
		t.Fatal("b is not yet replanted; want false")
	}

	// Replant b onto main, dropping a's commits.
	fork, _ := g.MergeBase(aTip, "b")
	run(t, dir, "-c", "core.commentChar=#", "rebase", "--onto", "main", fork, "b")
	mainTip, _ = g.RevParse("main")
	done, err = g.AlreadyReplanted(mainTip, aTip, "b")
	if err != nil {
		t.Fatalf("AlreadyReplanted: %v", err)
	}
	if !done {
		t.Fatal("b is now replanted onto main; want true")
	}
}

func TestLocalBranchOID(t *testing.T) {
	dir := initRepo(t)
	g := New(dir)
	oid, exists, err := g.LocalBranchOID("a")
	if err != nil || !exists || len(oid) < 7 {
		t.Fatalf("branch a: oid=%q exists=%v err=%v", oid, exists, err)
	}
	_, exists, err = g.LocalBranchOID("does-not-exist")
	if err != nil || exists {
		t.Fatalf("missing branch: exists=%v err=%v", exists, err)
	}
}

func TestPrepareBranch(t *testing.T) {
	dir := initRepo(t)
	g := New(dir)

	// Missing branch is created at headOID.
	bTip, _ := g.RevParse("b")
	if err := g.PrepareBranch("newbranch", bTip); err != nil {
		t.Fatalf("create: %v", err)
	}
	oid, exists, _ := g.LocalBranchOID("newbranch")
	if !exists || oid != bTip {
		t.Fatalf("newbranch = %q, want %s", oid, bTip)
	}

	// Branch ahead of headOID (local unpushed commits) is refused.
	aTip, _ := g.RevParse("a")
	if err := g.PrepareBranch("b", aTip); err == nil {
		t.Fatal("expected refusal: b is ahead of a")
	}

	// Branch behind headOID is fast-forwarded to it.
	run(t, dir, "branch", "-f", "behind", "a")
	if err := g.PrepareBranch("behind", bTip); err != nil {
		t.Fatalf("ff: %v", err)
	}
	oid, _, _ = g.LocalBranchOID("behind")
	if oid != bTip {
		t.Fatalf("behind = %q, want %s after ff", oid, bTip)
	}

	// Diverged branch (its own commit off main, neither ancestor of headOID)
	// is refused and left untouched — the unpushed work must not be destroyed.
	run(t, dir, "checkout", "-q", "-b", "diverged", "main")
	commit(t, dir, "h", "d1", "d1")
	divergedTip, _, _ := g.LocalBranchOID("diverged")
	if err := g.PrepareBranch("diverged", bTip); err == nil {
		t.Fatal("expected refusal: diverged shares no ancestry line with b")
	}
	oid, _, _ = g.LocalBranchOID("diverged")
	if oid != divergedTip {
		t.Fatalf("diverged branch was modified: %q, want unchanged %s", oid, divergedTip)
	}
}

func TestRebaseDropsMergedCommits(t *testing.T) {
	dir := initRepo(t)
	g := New(dir)
	aTip, _ := g.RevParse("a")
	run(t, dir, "checkout", "-q", "main")
	run(t, dir, "merge", "--squash", "a")
	run(t, dir, "commit", "-qm", "squash a")
	fork, _ := g.MergeBase(aTip, "b")

	if err := g.Rebase("main", fork, "b"); err != nil {
		t.Fatalf("Rebase: %v", err)
	}
	commits, _ := g.RevList("main..b")
	if len(commits) != 1 || commits[0].Subject != "b1" {
		t.Fatalf("after replant main..b = %+v, want just b1", commits)
	}
	inProg, _ := g.RebaseInProgress()
	if inProg {
		t.Fatal("no rebase should be in progress after success")
	}
}

func TestRebaseConflictReported(t *testing.T) {
	// main and branch x both change the same line -> conflict on rebase.
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	run(t, dir, "config", "user.email", "t@e.com")
	run(t, dir, "config", "user.name", "T")
	commit(t, dir, "f", "base", "base")
	run(t, dir, "checkout", "-q", "-b", "x")
	commit(t, dir, "f", "from-x", "x1")
	run(t, dir, "checkout", "-q", "main")
	commit(t, dir, "f", "from-main", "m1")

	g := New(dir)
	base, _ := g.RevParse("main~1") // the shared "base" commit
	err := g.Rebase("main", base, "x")
	if err != ErrRebaseConflict {
		t.Fatalf("Rebase err = %v, want ErrRebaseConflict", err)
	}
	inProg, _ := g.RebaseInProgress()
	if !inProg {
		t.Fatal("a rebase should be in progress after a conflict")
	}
	run(t, dir, "rebase", "--abort") // clean up
}

func TestPushForceWithLease(t *testing.T) {
	// A bare remote; clone, rewrite history, force-push with lease.
	bare := t.TempDir()
	run(t, bare, "init", "-q", "--bare", "-b", "main")
	src := initRepo(t)
	run(t, src, "remote", "add", "origin", bare)
	run(t, src, "push", "-q", "origin", "b")

	g := New(src)
	run(t, src, "checkout", "-q", "b")
	// Amend the tip commit so local b diverges from origin/b (true force-push).
	run(t, src, "commit", "--amend", "-m", "b1-amended")
	if err := g.PushForceWithLease("origin", "b"); err != nil {
		t.Fatalf("PushForceWithLease: %v", err)
	}
}

func TestRebaseMergedTargetOntoDefault(t *testing.T) {
	// Shape from initRepo: main=c0; a=c0-a1-a2; b=c0-a1-a2-b1.
	// Simulate #A (branch a) squash-merged into main as a single commit, then
	// rebase the target branch b --onto main, dropping a1/a2 and keeping b1.
	dir := initRepo(t)
	g := New(dir)

	// Squash-merge a into main as one commit.
	run(t, dir, "checkout", "-q", "main")
	run(t, dir, "merge", "--squash", "a")
	run(t, dir, "commit", "-q", "-m", "squash of a")
	mainOID, err := g.RevParse("main")
	if err != nil {
		t.Fatal(err)
	}

	// Fork point of the target b is the tip of a (where b diverged).
	aOID, err := g.RevParse("a")
	if err != nil {
		t.Fatal(err)
	}
	bOID, err := g.RevParse("b")
	if err != nil {
		t.Fatal(err)
	}
	fork, err := g.MergeBase(aOID, bOID)
	if err != nil {
		t.Fatal(err)
	}

	if err := g.Rebase(mainOID, fork, "b"); err != nil {
		t.Fatalf("Rebase: %v", err)
	}

	// b now sits on main and no longer contains a's tip commit.
	onMain, err := g.IsAncestor(mainOID, "b")
	if err != nil {
		t.Fatal(err)
	}
	if !onMain {
		t.Error("b should sit on main after rebase")
	}
	hasOldParent, err := g.IsAncestor(aOID, "b")
	if err != nil {
		t.Fatal(err)
	}
	if hasOldParent {
		t.Error("b should no longer contain the old parent commits a1/a2")
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
