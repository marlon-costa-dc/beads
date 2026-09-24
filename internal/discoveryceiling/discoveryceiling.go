// Package discoveryceiling bounds ancestor discovery of beads state for the
// hermetic test harness.
//
// Several code paths walk up from a directory looking for .beads directories,
// config.yaml, metadata or databases. The harness (scripts/ci/lib/test-env.sh)
// keeps its disposable root under the invoking user's cache, so that root can
// sit below a developer's real ~/.beads. Env names the root; every walk that
// starts inside it stops after inspecting it. A walk that starts outside it (the
// source checkout, for example) keeps its normal semantics, and with Env unset
// the package is inert.
//
// The package depends only on the standard library so that every discovery
// owner, including internal/config and internal/utils consumers, can import it
// without an import cycle.
package discoveryceiling

import (
	"os"
	"path/filepath"
	"strings"
)

// Env names the directory above which ancestor discovery must not look.
const Env = "BEADS_TEST_DISCOVERY_CEILING"

// For returns the canonical ceiling for a walk that starts at start, or ""
// when no ceiling applies: Env is unset, or start lies outside the ceiling.
func For(start string) string {
	raw := os.Getenv(Env)
	if raw == "" {
		return ""
	}
	ceiling := canonical(raw)
	rel, err := filepath.Rel(ceiling, canonical(start))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	return ceiling
}

// Reached reports whether a walk standing at dir has reached ceiling (as
// returned by For) and must stop after inspecting dir.
func Reached(dir, ceiling string) bool {
	return ceiling != "" && canonical(dir) == ceiling
}

// canonical returns the absolute, symlink-resolved form of path, so that a
// ceiling and a walk reached through different aliases of one directory
// (e.g. /var and /private/var on macOS) compare equal. A path that does not
// exist has no symlinks to resolve and keeps its absolute form.
func canonical(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs
	}
	return resolved
}
