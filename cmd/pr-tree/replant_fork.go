package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/alexkalderimis/pr-tree/internal/git"
	"github.com/alexkalderimis/pr-tree/internal/replant"
	"github.com/alexkalderimis/pr-tree/internal/tree"
)

var squashSuffixRe = regexp.MustCompile(`\s*\(#\d+\)$`)

// normalizeSubject strips the trailing " (#NNNN)" that GitHub appends to
// squash-merge commit subjects and trims surrounding whitespace, so a merged
// commit can be matched against its pre-merge counterpart on the stack.
func normalizeSubject(s string) string {
	s = strings.TrimSpace(s)
	s = squashSuffixRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// inferForkBySubject finds the fork point for a drifted rebase, where the
// parent's recorded head is no longer an ancestor of the child (the parent was
// squash-merged and/or the stack was rewritten). candidates are the child's
// commits tip-first; mergedSubjects holds the normalized subjects that belong to
// the already-merged parent. The fork is the newest candidate whose subject is
// in that set — the most recent commit that came from the parent. Everything
// from the fork down is dropped on rebase; newer commits are the PR's own work.
// Returns ok=false when nothing matches.
func inferForkBySubject(candidates []git.Commit, mergedSubjects map[string]bool) (oid string, ok bool) {
	for _, c := range candidates {
		if mergedSubjects[normalizeSubject(c.Subject)] {
			return c.OID, true
		}
	}
	return "", false
}

// subjectSet builds the normalized lookup set used to recognise a parent's
// commits on a drifted branch, from the parent's fetched commit subjects plus
// its PR title (a squash/restack often reduces a parent to a single commit
// carrying the PR title).
func subjectSet(parentCommitSubjects []string, parentTitle string) map[string]bool {
	set := make(map[string]bool, len(parentCommitSubjects)+1)
	for _, s := range parentCommitSubjects {
		set[normalizeSubject(s)] = true
	}
	if parentTitle != "" {
		set[normalizeSubject(parentTitle)] = true
	}
	return set
}

// forkResult is the chosen fork point (the last commit to drop) and a short
// human description of how it was found, for the dry-run/apply output.
type forkResult struct {
	OID    string
	Method string
}

// forkUnknownError means a drifted target's fork point could not be inferred:
// the parent's recorded head is not on the branch and no branch commit matched
// the parent's commits. The candidate list drives the guidance message.
type forkUnknownError struct {
	parentPR   int
	candidates []git.Commit
}

func (e *forkUnknownError) Error() string {
	return fmt.Sprintf("can't tell which commits are already merged: parent #%d's head is no longer on this branch and none of the %d branch commit(s) match its commits — re-run with --keep N to keep the newest N commits",
		e.parentPR, len(e.candidates))
}

// resolveFork picks the fork point for rebasing a step's branch — the commit
// after which the branch's own work begins. Priority:
//
//  1. --keep N on the target: keep the newest N commits (fork = child~N).
//  2. ancestry: if the parent's recorded head is still on the branch, its
//     merge-base with the child is the reliable fork (the common case).
//  3. drift: the parent was squash-merged and/or the stack was rewritten, so
//     match the branch's commits against the parent's actual commits and fork
//     at the newest match.
//  4. otherwise: return *forkUnknownError so the caller can guide the user to
//     --keep.
//
// baseRef is the ref the branch will be rebased onto (used to list the branch's
// candidate commits in the drift case). keepN is honored only for the target.
func resolveFork(g *git.Git, baseRef string, s replant.Step, child, parent tree.PullRequest, keepN int, mergedSubjects map[string]bool) (forkResult, error) {
	if keepN > 0 && s.TargetSelf {
		fork, err := g.RevParse(fmt.Sprintf("%s~%d", child.HeadOID, keepN))
		if err != nil {
			return forkResult{}, fmt.Errorf("--keep %d: %w", keepN, err)
		}
		return forkResult{OID: fork, Method: fmt.Sprintf("--keep %d", keepN)}, nil
	}

	onBranch, err := g.IsAncestor(parent.HeadOID, child.HeadOID)
	if err != nil {
		return forkResult{}, err
	}
	if onBranch {
		fork, err := g.MergeBase(parent.HeadOID, child.HeadOID)
		if err != nil {
			return forkResult{}, err
		}
		return forkResult{OID: fork, Method: fmt.Sprintf("parent #%d head", s.ParentPR)}, nil
	}

	// Drift: the parent's recorded head isn't on this branch. Recognise the
	// parent's commits among the branch's by subject.
	candidates, err := g.RevList(baseRef + ".." + child.HeadOID)
	if err != nil {
		return forkResult{}, err
	}
	if fork, ok := inferForkBySubject(candidates, mergedSubjects); ok {
		return forkResult{OID: fork, Method: fmt.Sprintf("matched parent #%d's commits", s.ParentPR)}, nil
	}
	return forkResult{}, &forkUnknownError{parentPR: s.ParentPR, candidates: candidates}
}
