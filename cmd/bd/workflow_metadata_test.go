package main

import (
	"encoding/json"
	"testing"

	"github.com/steveyegge/beads/internal/config"
	"github.com/steveyegge/beads/internal/types"
)

func metaIssue(t *testing.T, id string, meta string) *types.Issue {
	t.Helper()
	issue := &types.Issue{ID: id, Title: "Build and sanity check", Status: types.StatusOpen}
	if meta != "" {
		issue.Metadata = json.RawMessage(meta)
	}
	return issue
}

// initDedupConfig loads the real config defaults in an isolated directory.
func initDedupConfig(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	config.ResetForTesting()
	t.Cleanup(config.ResetForTesting)
	if err := config.Initialize(); err != nil {
		t.Fatalf("config.Initialize: %v", err)
	}
}

func TestWorkflowMetadataPrefixesDefault(t *testing.T) {
	initDedupConfig(t)
	got := config.GetStringSlice(workflowMetadataPrefixesKey)
	if len(got) != 1 || got[0] != "gc." {
		t.Fatalf("%s default = %v, want [gc.]", workflowMetadataPrefixesKey, got)
	}
}

func TestHasOrchestratorMetadata(t *testing.T) {
	prefixes := []string{"gc."}
	tests := []struct {
		name string
		meta string
		want bool
	}{
		{"empty metadata", "", false},
		{"migration only", `{"migration":{"survivor_id":"x"}}`, false},
		{"gc kind", `{"gc.kind":"ralph"}`, true},
		{"gc root bead", `{"gc.root_bead_id":"gct-mk05t","gc.step_id":"x"}`, true},
		{"mixed", `{"migration":{},"gc.logical_bead_id":"y"}`, true},
		{"non-gc namespaced", `{"gct.other":1,"gcc":2,"agc":3}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := metaIssue(t, "bd-test", tt.meta)
			got, err := hasOrchestratorMetadata(issue, prefixes)
			if err != nil {
				t.Fatalf("hasOrchestratorMetadata(%s) error: %v", tt.meta, err)
			}
			if got != tt.want {
				t.Errorf("hasOrchestratorMetadata(%s) = %v, want %v", tt.meta, got, tt.want)
			}
		})
	}
}

func TestHasOrchestratorMetadataRejectsNonObject(t *testing.T) {
	for _, meta := range []string{`{not json`, `["gc.kind"]`, `"gc.kind"`} {
		issue := metaIssue(t, "bd-bad", meta)
		if _, err := hasOrchestratorMetadata(issue, []string{"gc."}); err == nil {
			t.Errorf("hasOrchestratorMetadata(%s) returned no error for non-object metadata", meta)
		}
	}
}

func TestPartitionWorkflowIssues(t *testing.T) {
	initDedupConfig(t)
	issues := []*types.Issue{
		metaIssue(t, "bd-1", ""),
		metaIssue(t, "bd-2", `{"gc.kind":"spec"}`),
		metaIssue(t, "bd-3", `{"migration":{"x":1}}`),
		metaIssue(t, "bd-4", `{"gc.step_id":"s"}`),
	}
	normal, workflowCount, err := partitionWorkflowIssues(issues)
	if err != nil {
		t.Fatalf("partitionWorkflowIssues: %v", err)
	}
	if workflowCount != 2 {
		t.Errorf("workflowCount = %d, want 2", workflowCount)
	}
	if len(normal) != 2 || normal[0].ID != "bd-1" || normal[1].ID != "bd-3" {
		t.Fatalf("normal = %v, want [bd-1 bd-3]", issueIDs(normal))
	}
}

func TestPartitionWorkflowIssuesUsesConfiguredPrefixes(t *testing.T) {
	initDedupConfig(t)
	config.Set(workflowMetadataPrefixesKey, []string{"orch."})
	issues := []*types.Issue{
		metaIssue(t, "bd-1", `{"gc.kind":"spec"}`),
		metaIssue(t, "bd-2", `{"orch.run":"r1"}`),
	}
	normal, workflowCount, err := partitionWorkflowIssues(issues)
	if err != nil {
		t.Fatalf("partitionWorkflowIssues: %v", err)
	}
	if workflowCount != 1 || len(normal) != 1 || normal[0].ID != "bd-1" {
		t.Fatalf("workflowCount = %d, normal = %v; want 1 and [bd-1]", workflowCount, issueIDs(normal))
	}
}

func TestPartitionWorkflowIssuesPropagatesMalformedMetadata(t *testing.T) {
	initDedupConfig(t)
	issues := []*types.Issue{
		metaIssue(t, "bd-1", ""),
		metaIssue(t, "bd-bad", `{not json`),
	}
	if _, _, err := partitionWorkflowIssues(issues); err == nil {
		t.Fatal("partitionWorkflowIssues returned no error for malformed metadata")
	}
}

func TestScopeWorkflowIssues(t *testing.T) {
	initDedupConfig(t)
	issues := []*types.Issue{
		metaIssue(t, "bd-1", ""),
		metaIssue(t, "bd-2", `{"gc.kind":"spec"}`),
		metaIssue(t, "bd-3", ""),
	}

	skipped, scope, err := scopeWorkflowIssues(issues, false)
	if err != nil {
		t.Fatalf("scopeWorkflowIssues(include=false): %v", err)
	}
	if scope.included || scope.skipped != 1 || len(skipped) != 2 {
		t.Fatalf("include=false: scope = %+v, issues = %v; want skipped=1 and 2 issues", scope, issueIDs(skipped))
	}
	groups := findDuplicateGroups(skipped)
	if len(groups) != 1 || len(groups[0]) != 2 {
		t.Fatalf("include=false: groups = %d, want one group of bd-1 + bd-3", len(groups))
	}
	output := map[string]interface{}{}
	scope.addJSON(output)
	if output["workflow_skipped"] != 1 || output["include_workflow"] != false {
		t.Errorf("addJSON = %v, want workflow_skipped=1 include_workflow=false", output)
	}
	if scope.skippedNotice() == "" {
		t.Error("skippedNotice is empty while a workflow bead was skipped")
	}

	all, scope, err := scopeWorkflowIssues(issues, true)
	if err != nil {
		t.Fatalf("scopeWorkflowIssues(include=true): %v", err)
	}
	if !scope.included || scope.skipped != 0 || len(all) != 3 {
		t.Fatalf("include=true: scope = %+v, issues = %v; want all 3 issues", scope, issueIDs(all))
	}
	if scope.skippedNotice() != "" {
		t.Errorf("skippedNotice = %q, want empty when nothing was skipped", scope.skippedNotice())
	}
}
