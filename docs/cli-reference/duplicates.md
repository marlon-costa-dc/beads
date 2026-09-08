---
title: "bd duplicates"
description: "Find and optionally merge duplicate issues"
---

{/* AUTO-GENERATED: do not edit manually */}

Generated from `bd help --doc duplicates`.

Find issues with identical content (title, description, design, acceptance criteria).
Groups issues by content hash and reports duplicates with suggested merge targets.
The merge target is chosen by:
1. Reference count (most referenced issue wins)
2. Lexicographically smallest ID if reference counts are equal
Only non-closed issues are considered.

Orchestrator-managed workflow beads (metadata carrying "gc."-prefixed keys,
e.g. Gas City spec/logical/control template instances) are skipped: their
identical template text is owned by the orchestrator lifecycle, not by
content deduplication. Pass --include-workflow to include them anyway.

Example:
  bd duplicates                    # Show all duplicate groups
  bd duplicates --auto-merge       # Automatically merge all duplicates
  bd duplicates --dry-run          # Show what would be merged
  bd duplicates --include-workflow # Also consider orchestrator-managed beads

```
bd duplicates [flags]
```

**Flags:**

```
      --auto-merge         Automatically merge all duplicates
      --dry-run            Show what would be merged without making changes
      --include-workflow   Also consider orchestrator-managed workflow beads (metadata with gc.* keys); they are skipped by default
```
