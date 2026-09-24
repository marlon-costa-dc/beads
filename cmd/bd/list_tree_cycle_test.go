package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/steveyegge/beads/internal/types"
)

// captureBoundedStdout runs fn with os.Stdout redirected and returns what it
// printed, discarding anything past limit.
//
// The package already has captureStdout (test_helpers_pure_test.go), but it
// buffers without a cap: against an unguarded renderer, whose output is
// unbounded, that helper would exhaust memory instead of failing the test. This
// variant caps the read and keeps draining so the writer never blocks on a full
// pipe. It takes the same stdioMutex to stay race-free with the existing helper.
func captureBoundedStdout(t *testing.T, limit int64, fn func()) string {
	t.Helper()

	stdioMutex.Lock()
	defer stdioMutex.Unlock()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, io.LimitReader(r, limit)); err != nil {
			done <- fmt.Sprintf("capture read error: %v", err)
			return
		}
		// Drain any remainder so the writer never blocks on a full pipe.
		if _, err := io.Copy(io.Discard, r); err != nil {
			done <- fmt.Sprintf("capture drain error: %v", err)
			return
		}
		done <- buf.String()
	}()

	fn()

	os.Stdout = orig
	if err := w.Close(); err != nil {
		t.Fatalf("close capture writer: %v", err)
	}
	out := <-done
	if err := r.Close(); err != nil {
		t.Fatalf("close capture reader: %v", err)
	}
	return out
}

// treeTestIssue builds a tree fixture whose title never repeats its ID, so a
// test counting ID occurrences counts rendered nodes, not title text.
func treeTestIssue(id, issueType string) *types.Issue {
	now := time.Now()
	return &types.Issue{
		ID:        id,
		Title:     issueType + " fixture",
		IssueType: types.IssueType(issueType),
		Status:    "open",
		Priority:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// epicBlockedByItsOwnChild builds the shape reported upstream in #5887: an epic
// and its child, where the epic is also blocked by that child.
//
//	child --parent-child--> epic     (child belongs to the epic)
//	epic  --blocks-------->  child   (epic is blocked by its own child)
func epicBlockedByItsOwnChild() ([]*types.Issue, map[string][]*types.Dependency) {
	parent, child := treeTestIssue("bd-ep41", "epic"), treeTestIssue("bd-ep40", "epic")
	deps := map[string][]*types.Dependency{
		child.ID:  {{IssueID: child.ID, DependsOnID: parent.ID, Type: types.DepParentChild}},
		parent.ID: {{IssueID: parent.ID, DependsOnID: child.ID, Type: types.DepBlocks}},
	}
	return []*types.Issue{parent, child}, deps
}

// TestBuildIssueTreeWithDeps_BlocksIsNotHierarchy covers upstream #5887: a
// `blocks` edge must not become a tree edge; the parent-child edge still nests.
func TestBuildIssueTreeWithDeps_BlocksIsNotHierarchy(t *testing.T) {
	issues, deps := epicBlockedByItsOwnChild()
	parent, child := issues[0], issues[1]

	roots, childrenMap := buildIssueTreeWithDeps(issues, deps)

	if len(roots) != 1 || roots[0].ID != parent.ID {
		t.Errorf("roots = %v, want exactly [%s]", roots, parent.ID)
	}
	if got := childrenMap[parent.ID]; len(got) != 1 || got[0].ID != child.ID {
		t.Errorf("childrenMap[%s] = %v, want [%s]", parent.ID, got, child.ID)
	}
	if got := childrenMap[child.ID]; len(got) != 0 {
		t.Errorf("childrenMap[%s] = %v, want empty: a blocks edge is not containment", child.ID, got)
	}
}

// TestDisplayPrettyList_EpicBlockedByChildTerminates is the end-to-end guard for
// #5887 through the public list entry point.
func TestDisplayPrettyList_EpicBlockedByChildTerminates(t *testing.T) {
	issues, deps := epicBlockedByItsOwnChild()

	finished := make(chan string, 1)
	go func() {
		finished <- captureBoundedStdout(t, 1<<20, func() {
			displayPrettyListWithDeps(issues, false, deps, false, false)
		})
	}()

	select {
	case out := <-finished:
		if !strings.Contains(out, "Total: 2 issues") {
			t.Errorf("summary missing; got:\n%s", out)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("displayPrettyListWithDeps did not terminate on the #5887 graph")
	}
}

// TestPrintPrettyTree_TerminatesOnCycle forces a cycle directly into childrenMap,
// bypassing tree construction, so the renderer's own defenses are under test.
// An unguarded renderer never returns here.
func TestPrintPrettyTree_TerminatesOnCycle(t *testing.T) {
	a, b := treeTestIssue("bd-cyca", "epic"), treeTestIssue("bd-cycb", "epic")
	childrenMap := map[string][]*types.Issue{
		a.ID: {b},
		b.ID: {a},
	}

	const limit = 1 << 20 // far above any sane output

	finished := make(chan string, 1)
	go func() {
		finished <- captureBoundedStdout(t, limit, func() {
			printPrettyTree(childrenMap, a.ID, "", nil)
		})
	}()

	select {
	case out := <-finished:
		if !strings.Contains(out, "(cycle)") {
			t.Errorf("cycle closure not marked; got:\n%s", out)
		}
		if n := strings.Count(out, b.ID); n != 1 {
			t.Errorf("%s rendered %d times, want exactly 1 before the cycle is cut:\n%s", b.ID, n, out)
		}
		if len(out) >= limit {
			t.Errorf("output hit the %d byte cap: renderer is still unbounded", limit)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("printPrettyTree did not terminate on a cyclic childrenMap")
	}
}

// TestPrintPrettyTree_DiamondStillRendersBothPaths guards against over-cutting:
// the visited set is path-scoped, so a node reachable through two parents
// renders under each and is not reported as a cycle.
func TestPrintPrettyTree_DiamondStillRendersBothPaths(t *testing.T) {
	root, left, right, leaf := treeTestIssue("bd-d0", "task"), treeTestIssue("bd-d1", "task"),
		treeTestIssue("bd-d2", "task"), treeTestIssue("bd-d3", "task")
	// Distinct title so counting the ID is not doubled by the title column.
	leaf.Title = "shared leaf"

	childrenMap := map[string][]*types.Issue{
		root.ID:  {left, right},
		left.ID:  {leaf},
		right.ID: {leaf},
	}

	out := captureBoundedStdout(t, 1<<20, func() {
		printPrettyTree(childrenMap, root.ID, "", nil)
	})

	if n := strings.Count(out, leaf.ID); n != 2 {
		t.Errorf("%s rendered %d times, want 2 (once under each parent):\n%s", leaf.ID, n, out)
	}
	if strings.Contains(out, "(cycle)") {
		t.Errorf("diamond wrongly reported as a cycle:\n%s", out)
	}
}

// TestPrintPrettyTree_DepthCeilingIsVisible verifies a chain deeper than the
// shared ceiling stops at defaultTreeMaxDepth and marks the cut node with the
// truncation marker instead of silently dropping its subtree.
func TestPrintPrettyTree_DepthCeilingIsVisible(t *testing.T) {
	const chain = defaultTreeMaxDepth + 5
	nodes := make([]*types.Issue, chain+1)
	for i := range nodes {
		nodes[i] = treeTestIssue(fmt.Sprintf("bd-deep%03d", i), "task")
	}
	childrenMap := make(map[string][]*types.Issue, chain)
	for i := 0; i < chain; i++ {
		childrenMap[nodes[i].ID] = []*types.Issue{nodes[i+1]}
	}

	out := captureBoundedStdout(t, 1<<20, func() {
		printPrettyTree(childrenMap, nodes[0].ID, "", nil)
	})

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != defaultTreeMaxDepth {
		t.Fatalf("rendered %d lines, want %d (the depth ceiling):\n%s", len(lines), defaultTreeMaxDepth, out)
	}
	last := lines[len(lines)-1]
	if !strings.Contains(last, nodes[defaultTreeMaxDepth].ID) || !strings.Contains(last, "…") {
		t.Errorf("last rendered line %q must be %s carrying the truncation marker", last, nodes[defaultTreeMaxDepth].ID)
	}
}
