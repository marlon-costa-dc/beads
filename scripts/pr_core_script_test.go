package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestPRCoreScriptUsesExplicitTimeoutBudget(t *testing.T) {
	call := prCoreGoTestCall(t, nil)

	index := slices.Index(call, "-timeout")
	if index < 0 || index+1 >= len(call) {
		t.Fatalf("pr-core go test call has no explicit timeout budget: %q", call)
	}
	if call[index+1] != "30m" {
		t.Fatalf("pr-core timeout = %q, want 30m", call[index+1])
	}
}

func TestPRCoreScriptTimeoutBudgetIsConfigurable(t *testing.T) {
	call := prCoreGoTestCall(t, []string{"GO_TEST_TIMEOUT=45m"})

	index := slices.Index(call, "-timeout")
	if index < 0 || index+1 >= len(call) {
		t.Fatalf("pr-core go test call has no explicit timeout budget: %q", call)
	}
	if call[index+1] != "45m" {
		t.Fatalf("pr-core timeout = %q, want the configured 45m", call[index+1])
	}
}

func TestPRCoreScriptKeepsGoTemporaryBuildsOutOfSystemTmp(t *testing.T) {
	call, env := prCoreGoTestCallWithEnv(t, nil)
	if len(call) == 0 {
		t.Fatal("pr-core did not invoke go test")
	}
	for _, name := range []string{"TMPDIR", "GOTMPDIR", "GOCACHE"} {
		value := env[name]
		if value == "" {
			t.Fatalf("%s was not exported to go test", name)
		}
		if value == "/tmp" || strings.HasPrefix(value, "/tmp/") {
			t.Fatalf("%s still uses system /tmp: %q", name, value)
		}
		if !strings.Contains(value, ".test-tmp") {
			t.Fatalf("%s = %q, want project-scoped .test-tmp", name, value)
		}
	}
}

func prCoreGoTestCall(t *testing.T, extraEnv []string) []string {
	call, _ := prCoreGoTestCallWithEnv(t, extraEnv)
	return call
}

func prCoreGoTestCallWithEnv(t *testing.T, extraEnv []string) ([]string, map[string]string) {
	t.Helper()

	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("bash is required to test pr-core.sh: %v", err)
	}

	bin := t.TempDir()
	stateDir := t.TempDir()
	callLog := filepath.Join(stateDir, "go-calls")
	envLog := filepath.Join(stateDir, "go-env")
	if err := os.WriteFile(callLog, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	writeExecutable(t, filepath.Join(bin, "go"), `#!/bin/sh
set -eu
printf '%s\0' "$#" >>"$GO_CALL_LOG"
printf '%s\0' "$@" >>"$GO_CALL_LOG"
printf 'TMPDIR=%s\nGOTMPDIR=%s\nGOCACHE=%s\n' "${TMPDIR:-}" "${GOTMPDIR:-}" "${GOCACHE:-}" >"$GO_ENV_LOG"
case "${1:-}" in
env) shift; for name in "$@"; do printf '%s\n' ""; done ;;
esac
`)

	binPath := shellPath(t, bin)
	statePath := shellPath(t, stateDir)
	path := binPath + ":" + os.Getenv("PATH") + ":/usr/bin:/bin"
	if runtime.GOOS == "windows" {
		path = binPath + ":/usr/bin:/bin"
	}
	cmd := exec.Command(bash, "scripts/ci/pr-core.sh")
	cmd.Dir = sourceRepoRoot(t)
	cmd.Env = append([]string{
		"PATH=" + path,
		"HOME=" + statePath,
		"LC_ALL=C",
		"LANG=C",
		"BASH_ENV=",
		"ENV=",
		"GO_CALL_LOG=" + statePath + "/go-calls",
		"GO_ENV_LOG=" + statePath + "/go-env",
	}, extraEnv...)
	output, runErr := cmd.CombinedOutput()
	if runErr != nil {
		t.Fatalf("pr-core.sh failed: %v\n%s", runErr, output)
	}

	log, err := os.ReadFile(callLog)
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range parseArgvCalls(t, log) {
		if len(call) > 0 && call[0] == "test" {
			envBytes, readErr := os.ReadFile(envLog)
			if readErr != nil {
				t.Fatal(readErr)
			}
			env := make(map[string]string)
			for _, line := range strings.Split(strings.TrimSpace(string(envBytes)), "\n") {
				name, value, ok := strings.Cut(line, "=")
				if ok {
					env[name] = value
				}
			}
			return call, env
		}
	}
	t.Fatalf("pr-core.sh never invoked go test:\n%s", strings.TrimSpace(string(output)))
	return nil, nil
}
