package render

import (
	"strconv"
	"strings"

	"github.com/alexkalderimis/pr-tree/internal/tree"
)

// AnnotateChange classifies what annotate would do to a PR body.
type AnnotateChange int

const (
	ChangeNone   AnnotateChange = iota // body already current
	ChangeInsert                       // no block yet; one will be added
	ChangeUpdate                       // an existing block/note will change
)

// AnnotateItem is the render view of one planned annotation.
type AnnotateItem struct {
	Number  int
	Title   string
	State   tree.State
	Change  AnnotateChange
	OldBody string
	NewBody string
}

// AnnotatePlan renders one header line per item and, for changed items, an
// indented coloured unified diff of the description (context collapsed to 3
// lines around each change).
func AnnotatePlan(items []AnnotateItem, opts Options) string {
	var b strings.Builder
	for _, it := range items {
		num := style("#"+strconv.Itoa(it.Number), opts.Color, ansiCyan)
		title := style(it.Title, opts.Color, ansiBold)
		b.WriteString("  " + num + " (" + title + ")  " + changeLabel(it.Change) + "\n")
		if it.Change != ChangeNone {
			ops := diffLines(strings.Split(it.OldBody, "\n"), strings.Split(it.NewBody, "\n"))
			b.WriteString(renderDiff(ops, opts.Color))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func changeLabel(c AnnotateChange) string {
	switch c {
	case ChangeInsert:
		return "insert links block"
	case ChangeUpdate:
		return "update links block"
	default:
		return "no change"
	}
}

type diffKind int

const (
	dEqual diffKind = iota
	dDel
	dIns
)

type diffOp struct {
	kind diffKind
	text string
}

// diffLines computes a line-level diff of a and b via longest common
// subsequence. Deletions precede insertions at each divergence.
func diffLines(a, b []string) []diffOp {
	n, m := len(a), len(b)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	var ops []diffOp
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			ops = append(ops, diffOp{dEqual, a[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, diffOp{dDel, a[i]})
			i++
		default:
			ops = append(ops, diffOp{dIns, b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, diffOp{dDel, a[i]})
	}
	for ; j < m; j++ {
		ops = append(ops, diffOp{dIns, b[j]})
	}
	return ops
}

// renderDiff formats diff ops as indented lines, collapsing runs of equal
// lines that are more than 3 away from any change behind a dim ellipsis.
func renderDiff(ops []diffOp, color bool) string {
	const ctx = 3
	const indent = "      "

	show := make([]bool, len(ops))
	for i, op := range ops {
		if op.kind == dEqual {
			continue
		}
		for k := i - ctx; k <= i+ctx; k++ {
			if k >= 0 && k < len(ops) {
				show[k] = true
			}
		}
	}

	var b strings.Builder
	hiddenPending := false
	for i, op := range ops {
		if op.kind == dEqual && !show[i] {
			hiddenPending = true
			continue
		}
		if hiddenPending {
			b.WriteString(indent + style("…", color, ansiDim) + "\n")
			hiddenPending = false
		}
		switch op.kind {
		case dDel:
			b.WriteString(indent + style("- "+op.text, color, ansiRed) + "\n")
		case dIns:
			b.WriteString(indent + style("+ "+op.text, color, ansiGreen) + "\n")
		default:
			b.WriteString(indent + style("  "+op.text, color, ansiDim) + "\n")
		}
	}
	if hiddenPending {
		b.WriteString(indent + style("…", color, ansiDim) + "\n")
	}
	return b.String()
}
