# This fork

This repository is a fork of its GitHub parent. It ships the parent's release
tree unchanged, plus a small set of fork plumbing. The integration branch is
the repository's default branch. Fork releases are prereleases named
`vX.Y.Z-<label>.N`, where `vX.Y.Z` is the upstream release and `<label>` is
this fork's release label.

## Policy

- **Product code equals upstream.** `git diff <upstream-tag> origin/<default-branch>`
  lists only fork plumbing: this file, `scripts/fork/`, the fork release
  workflow, and the few upstream files the plumbing has to touch. The one
  exception is the frozen migrations described below.
- **A product patch needs a reason.** It lives on its own `fork/<topic>`
  branch and exists only for a defect we hit at runtime with no upstream fix.
  Refactors, style changes and dead-code removal of upstream code go upstream,
  never here.
- **Upstream's own mechanisms come first.** Before patching, look for an
  existing configuration key, environment variable, extension point or
  upstream commit.

## Frozen migrations adopted from the old integration branch

Before v1.3.0, the old `dc-use` absorbed unreleased upstream-main commit
`2bb1e20de` (#6304). That added `0067_add_versioned_beads_schema.{up,down}.sql`
and `ignored/0026_add_wisps_current_revision.up.sql`, and this fork's stores
recorded that ignored `0026` (its `content_hash` is the sha256 of that file).
Shipped migrations are frozen (`scripts/check-migration-hygiene.sh` Check C),
so the fork keeps all three files byte-identical. It numbers upstream v1.3.0's
own ignored `0026_dep_rekey_dedup_marker` as **ignored `0027`**, the next free
ordinal, as the hygiene rule prescribes.

What this means in practice:
- **Main series ends at 0067.** Its body is byte-identical to upstream main's
  `0067`, including the CLI direct-DDL override backported verbatim from
  upstream main, so a future upstream release carrying `0067` converges.
- **The dep re-key marker is pending ignored `0027`.** It forces the one
  merge-aware dependency-id re-key pass, exactly as upstream's `0026` does.
- **Syncing to an upstream release that renumbers these files**, for example
  upstream main's `ignored/0027_add_wisps_current_revision`: keep the fork's
  frozen files and give each new upstream ignored migration the next free
  ordinal. Never rename or edit a migration a store may have recorded.

## Update to a new upstream release

```bash
scripts/fork/fork.sh sync            # the latest upstream release
scripts/fork/fork.sh sync v1.3.1     # or an explicit upstream release tag
```

`sync` does the following:
1. Creates `lane/<default-branch>-<tag>` in a linked worktree next to the
   other worktrees.
2. Branches from the upstream tag and merges the default branch in with
   `-s ours`, so the tree equals the tag and the PR merges completely.
3. Re-applies the current fork plumbing (`git diff <previous-base> origin/<default-branch>`)
   with `git apply --3way`. A conflict stops the script.
4. Refreshes the bd pairing, where this repository has one.
5. Pushes the lane and opens a draft PR.

Commit and push hooks run normally. When the default branch already carries
the requested release, `sync` prints `already at <tag>` and changes nothing.

## Release

After the PR is merged:

```bash
scripts/fork/fork.sh release
```

This tags the merged head as the next `<base>-<label>.N` and pushes the tag.
The fork release workflow then builds and publishes the prerelease.

## bd ↔ gc pairing

Gas City checks that the `bd` CLI version equals the version of the beads
library it links, as an exact string. In the gascity fork,
`scripts/fork/fork.sh pair [<bd-fork-tag>]` does the pairing:
- it points `go.mod`'s `replace` for `github.com/steveyegge/beads` at the bd
  fork release, by default the newest one for the linked version;
- it refreshes `go.sum`;
- it records `BD_FORK_MODULE` and `BD_FORK_VERSION` in `deps.env`.

`scripts/check-gomod-replace.sh` accepts exactly that replace and nothing
else. Release the bd fork first, then pair and release gc.
