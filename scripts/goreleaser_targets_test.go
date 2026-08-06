package scripts_test

import (
	"strings"
	"testing"
)

// Every GoReleaser build target must be one this tree can actually compile.
// internal/procid only implements Capture for linux, darwin and windows, so a
// freebsd target is a build that fails by construction — it is declared
// coverage the release can never deliver.
func TestGoReleaserDeclaresOnlyBuildableTargets(t *testing.T) {
	config := readGoReleaserText(t)

	for _, unsupported := range []string{"freebsd"} {
		if strings.Contains(config, unsupported) {
			t.Errorf("goreleaser declares %q, which has no procid implementation and cannot build", unsupported)
		}
	}
}
