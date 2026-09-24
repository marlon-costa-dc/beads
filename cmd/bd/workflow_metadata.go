package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/steveyegge/beads/internal/config"
	"github.com/steveyegge/beads/internal/types"
	"github.com/steveyegge/beads/internal/ui"
)

// workflowMetadataPrefixesKey names the config list of metadata key prefixes
// that mark orchestrator-managed workflow instance beads. An orchestrator (Gas
// City, for example, with gc.kind, gc.step_id, gc.root_bead_id, ...) owns the
// lifecycle of these beads: a spec bead, a logical step bead and an iteration
// control bead for the same step legitimately share identical text, and per-run
// instances repeat it across runs, so content-hash deduplication must not
// adjudicate them. The default list is declared once, in internal/config.
const workflowMetadataPrefixesKey = "dedup.workflow_metadata_prefixes"

// hasOrchestratorMetadata reports whether an issue carries a metadata key under
// any of the configured workflow prefixes. Metadata that is not a JSON object is
// an error naming the issue: a bead whose metadata cannot be read must not be
// silently treated as an ordinary issue.
func hasOrchestratorMetadata(issue *types.Issue, prefixes []string) (bool, error) {
	if len(issue.Metadata) == 0 {
		return false, nil
	}
	var meta map[string]json.RawMessage
	if err := json.Unmarshal(issue.Metadata, &meta); err != nil {
		return false, fmt.Errorf("issue %s: metadata is not a JSON object: %w", issue.ID, err)
	}
	for key := range meta {
		for _, prefix := range prefixes {
			if strings.HasPrefix(key, prefix) {
				return true, nil
			}
		}
	}
	return false, nil
}

// partitionWorkflowIssues splits issues into orchestrator-managed workflow
// instances and normal issues, using the configured workflow metadata prefixes.
// The dedup gates (bd duplicates, bd find-duplicates) only adjudicate normal
// issues unless the operator explicitly passes --include-workflow.
func partitionWorkflowIssues(issues []*types.Issue) (normal []*types.Issue, workflowCount int, err error) {
	prefixes := config.GetStringSlice(workflowMetadataPrefixesKey)
	normal = make([]*types.Issue, 0, len(issues))
	for _, issue := range issues {
		isWorkflow, err := hasOrchestratorMetadata(issue, prefixes)
		if err != nil {
			return nil, 0, err
		}
		if isWorkflow {
			workflowCount++
			continue
		}
		normal = append(normal, issue)
	}
	return normal, workflowCount, nil
}

// workflowScope records how orchestrator-managed workflow beads were treated
// by a dedup gate, so every output path reports the same scope.
type workflowScope struct {
	included bool
	skipped  int
}

// scopeWorkflowIssues returns the issues a dedup gate adjudicates: workflow
// beads are removed unless includeWorkflow is set.
func scopeWorkflowIssues(issues []*types.Issue, includeWorkflow bool) ([]*types.Issue, workflowScope, error) {
	scope := workflowScope{included: includeWorkflow}
	if includeWorkflow {
		return issues, scope, nil
	}
	candidates, skipped, err := partitionWorkflowIssues(issues)
	if err != nil {
		return nil, scope, err
	}
	scope.skipped = skipped
	return candidates, scope, nil
}

// addJSON writes the scope fields into a dedup JSON payload.
func (s workflowScope) addJSON(output map[string]interface{}) {
	output["workflow_skipped"] = s.skipped
	output["include_workflow"] = s.included
}

// skippedNotice is the text-mode line naming skipped workflow beads, or "".
func (s workflowScope) skippedNotice() string {
	if s.skipped == 0 {
		return ""
	}
	return fmt.Sprintf("%s Skipped %d orchestrator-managed workflow bead(s) (use --include-workflow to consider them)", ui.RenderAccent("ℹ"), s.skipped)
}
