package prlintmake

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFmtCheckClean(t *testing.T) {
	output, err := runFmtCheck(t, "package fixture\n", "")
	if err != nil {
		t.Fatalf("fmt-check failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "All Go files are properly formatted") {
		t.Fatalf("clean file was not accepted:\n%s", output)
	}
}

func TestFmtCheckReportsUnformattedFiles(t *testing.T) {
	output, err := runFmtCheck(t, "package fixture\nfunc main(){println(\"x\")}\n", "")
	if got := processExitCode(err); got != 1 {
		t.Fatalf("exit = %d, want 1; error=%v\n%s", got, err, output)
	}
	if !strings.Contains(output, "main.go") || !strings.Contains(output, "Run 'make fmt' to fix formatting") {
		t.Fatalf("unformatted file was not reported:\n%s", output)
	}
}

func TestFmtCheckFailsWhenToolchainCannotResolve(t *testing.T) {
	output, err := runFmtCheck(t, "package fixture\n", "invalid")
	if got := processExitCode(err); got == 0 {
		t.Fatalf("invalid toolchain reported success: error=%v\n%s", err, output)
	}
	if !strings.Contains(output, "GOTOOLCHAIN") || strings.Contains(output, "All Go files are properly formatted") {
		t.Fatalf("toolchain failure was hidden:\n%s", output)
	}
}

// runFmtCheck exercises the public script with the real Go toolchain in an
// isolated repository fixture. PATH stubs cannot validate toolchain selection.
func runFmtCheck(t *testing.T, goSource, toolchain string) (string, error) {
	t.Helper()
	root := t.TempDir()
	scriptDir := filepath.Join(root, "scripts", "ci")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(sourceRepoRoot(), "scripts", "ci", "fmt-check.sh"))
	if err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(scriptDir, "fmt-check.sh")
	if err := os.WriteFile(scriptPath, script, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(goSource), 0o644); err != nil {
		t.Fatal(err)
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatalf("bash is required: %v", err)
	}
	cmd := exec.Command(bash, "--noprofile", "--norc", "--", shellVisiblePath(scriptPath))
	cmd.Dir = root
	overrides := map[string]string{"BASH_ENV": "", "BASHOPTS": "", "ENV": "", "LANG": "C", "LC_ALL": "C", "SHELLOPTS": ""}
	if toolchain != "" {
		overrides["GOTOOLCHAIN"] = toolchain
	}
	cmd.Env = environment(overrides)
	output, err := cmd.CombinedOutput()
	return strings.ReplaceAll(string(output), "\r\n", "\n"), err
}

func environment(overrides map[string]string) []string {
	overridden := make(map[string]struct{}, len(overrides))
	for key := range overrides {
		overridden[strings.ToUpper(key)] = struct{}{}
	}
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, ok := overridden[strings.ToUpper(key)]; !ok {
			env = append(env, entry)
		}
	}
	for key, value := range overrides {
		env = append(env, key+"="+value)
	}
	return env
}

func processExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func sourceRepoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("runtime.Caller failed")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func shellVisiblePath(path string) string {
	if runtime.GOOS != "windows" {
		return path
	}
	path = filepath.ToSlash(filepath.Clean(path))
	if len(path) >= 3 && path[1] == ':' && path[2] == '/' {
		return "/" + strings.ToLower(path[:1]) + path[2:]
	}
	return path
}
