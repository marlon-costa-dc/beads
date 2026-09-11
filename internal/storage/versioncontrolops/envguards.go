package versioncontrolops

import "os"

// withRemoteEnvGuards runs fn — an engine statement that can touch a
// git-protocol remote (CALL DOLT_PUSH/DOLT_FETCH/DOLT_CLONE) — with bd's
// process environment made safe for the git plumbing Dolt spawns in-process.
//
// bd -C retargets the process-wide git context (GIT_DIR/GIT_WORK_TREE) so
// plain git call sites resolve the -C target repository. The embedded Dolt
// engine's git plumbing inherits that environment, and a GIT_WORK_TREE
// without a usable work tree in the remote's bare context aborts the
// transfer with "GIT_WORK_TREE (or --work-tree=<directory>) not allowed
// without specifying GIT_DIR". The values are removed from the process
// environment for the duration of the call and restored afterwards, so only
// Dolt's own git plumbing runs unguarded.
func withRemoteEnvGuards(fn func() error) error {
	guarded := []string{"GIT_DIR", "GIT_WORK_TREE"}
	var restore []func()
	for _, key := range guarded {
		if value, ok := os.LookupEnv(key); ok {
			_ = os.Unsetenv(key)
			restore = append(restore, func() { _ = os.Setenv(key, value) })
		}
	}
	defer func() {
		for _, r := range restore {
			r()
		}
	}()
	return fn()
}
