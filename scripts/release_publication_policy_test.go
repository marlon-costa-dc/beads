package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// This fork distributes bd only through the GitHub Releases of the repository
// whose v* tag triggered the release run. Publishing to a package
// registry or announcing a release anywhere else is outside that
// authorization, so those surfaces must not exist at all: a guard condition
// keeps a credentialed publish step one configuration change away from
// running.

func TestReleaseWorkflowHasNoPublishJobs(t *testing.T) {
	workflow := readCIWorkflow(t, "release.yml")

	for name := range workflow.Jobs {
		if strings.HasPrefix(name, "publish-") {
			t.Errorf("release workflow still declares registry publish job %q", name)
		}
	}
}

// Every release job must descend from release-target, the only job that
// resolves the release repository. It derives it from the run's own identity
// (GITHUB_REPOSITORY) and reads no Actions variable, so there is no second
// setting that could name a different repository.
func TestReleaseWorkflowJobsAreGatedByReleaseTarget(t *testing.T) {
	workflow := readCIWorkflow(t, "release.yml")

	target := workflow.job(t, "release-target")
	if target.If != "" {
		t.Errorf("release-target must always run so a malformed repository identity fails the run; has if: %q", target.If)
	}
	resolve := target.step(t, "Resolve release repository")
	if len(resolve.Env) != 0 {
		t.Errorf("release-target must derive the release repository from GITHUB_REPOSITORY, not from step env %v", resolve.Env)
	}
	for _, needle := range []string{"vars.", "RELEASE_REPOSITORY"} {
		if strings.Contains(resolve.Run, needle) {
			t.Errorf("release-target resolution references %q; the release repository is the run's own GITHUB_REPOSITORY", needle)
		}
	}
	if !strings.Contains(resolve.Run, "GITHUB_REPOSITORY") {
		t.Error("release-target resolution does not derive the repository from GITHUB_REPOSITORY")
	}

	for name := range workflow.Jobs {
		if name == "release-target" {
			continue
		}
		if !jobDescendsFrom(workflow, name, "release-target", map[string]bool{}) {
			t.Errorf("release job %q does not descend from release-target", name)
		}
		if strings.Contains(workflow.Jobs[name].If, "github.repository") {
			t.Errorf("release job %q compares github.repository itself (if: %q); the release repository is resolved only by release-target", name, workflow.Jobs[name].If)
		}
	}

	goreleaser := workflow.job(t, "goreleaser").step(t, "Run GoReleaser")
	for key, output := range map[string]string{
		"RELEASE_OWNER":      "owner",
		"RELEASE_NAME":       "name",
		"RELEASE_REPOSITORY": "repository",
	} {
		want := "${{ needs.release-target.outputs." + output + " }}"
		if got := goreleaser.Env[key]; got != want {
			t.Errorf("Run GoReleaser env %s = %q, want %q", key, got, want)
		}
	}
}

func jobDescendsFrom(workflow ciWorkflow, name, root string, seen map[string]bool) bool {
	if seen[name] {
		return false
	}
	seen[name] = true
	for _, need := range workflow.Jobs[name].Needs {
		if need == root || jobDescendsFrom(workflow, need, root, seen) {
			return true
		}
	}
	return false
}

type goReleaserPolicyConfig struct {
	Announce struct {
		Skip any `yaml:"skip"`
	} `yaml:"announce"`
	Release struct {
		GitHub struct {
			Owner string `yaml:"owner"`
			Name  string `yaml:"name"`
		} `yaml:"github"`
	} `yaml:"release"`
}

func TestGoReleaserDoesNotAnnounceOrPublishExternally(t *testing.T) {
	data := readRepoFile(t, ".goreleaser.yml")

	var config goReleaserPolicyConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("parse .goreleaser.yml: %v", err)
	}
	if skip, ok := config.Announce.Skip.(bool); !ok || !skip {
		t.Errorf("announce.skip = %#v, want true", config.Announce.Skip)
	}
	if got, want := config.Release.GitHub.Owner, "{{ .Env.RELEASE_OWNER }}"; got != want {
		t.Errorf("release.github.owner = %q, want %q", got, want)
	}
	if got, want := config.Release.GitHub.Name, "{{ .Env.RELEASE_NAME }}"; got != want {
		t.Errorf("release.github.name = %q, want %q", got, want)
	}

	var top map[string]any
	if err := yaml.Unmarshal(data, &top); err != nil {
		t.Fatalf("parse .goreleaser.yml: %v", err)
	}
	for _, publisher := range []string{
		"aurs", "blobs", "brews", "chocolateys", "dockers", "docker_manifests",
		"homebrew_casks", "kos", "nix", "npms", "publishers", "scoops",
		"snapcrafts", "uploads", "winget",
	} {
		if _, ok := top[publisher]; ok {
			t.Errorf(".goreleaser.yml declares external publisher %q", publisher)
		}
	}
}

func TestPythonVersionProjection(t *testing.T) {
	lib := shellPath(t, filepath.Join(sourceRepoRoot(t), "scripts", "lib", "python-version.sh"))

	for version, want := range map[string]string{
		"1.3.0":       "1.3.0",
		"1.3.0-fd.1":  "1.3.0+fd.1",
		"1.3.0-fd.12": "1.3.0+fd.12",
		"1.1.0-rc.1":  "1.1.0rc1",
		"1.1.0-rc2":   "1.1.0rc2",
	} {
		out, err := projectPythonVersion(lib, version)
		if err != nil {
			t.Errorf("python_version %q: %v\n%s", version, err, out)
			continue
		}
		if got := strings.TrimSpace(out); got != want {
			t.Errorf("python_version %q = %q, want %q", version, got, want)
		}
	}

	// Legacy fork suffixes and anything else without a declared projection
	// must fail rather than pass through.
	for _, version := range []string{
		"1.3.0-dc1", "1.2.2-fd4", "1.3.0-fd.", "1.3.0-beta.1", "v1.3.0", "1.3", "",
	} {
		out, err := projectPythonVersion(lib, version)
		if err == nil {
			t.Errorf("python_version %q succeeded with %q, want failure", version, strings.TrimSpace(out))
		}
	}
}

func projectPythonVersion(lib, version string) (string, error) {
	cmd := exec.Command("bash", "--noprofile", "--norc", "-c", `source "$1" && python_version "$2"`, "python-version-test", lib, version)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func readRepoFile(t *testing.T, relative string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(sourceRepoRoot(t), relative))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
