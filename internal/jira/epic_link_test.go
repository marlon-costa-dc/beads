package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/steveyegge/beads/internal/config"
	"github.com/steveyegge/beads/internal/types"
)

// initHermeticConfig points the global config package at an isolated,
// empty directory so viper defaults (e.g. jira.epic_link_field) are
// established without reading the real ~/.beads or any project config.
func initHermeticConfig(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	t.Setenv("BEADS_DIR", "")
	t.Setenv("JIRA_EPIC_KEY", "")
	t.Setenv("JIRA_EPIC_LINK_FIELD", "")
	t.Setenv("BEADS_TEST_IGNORE_REPO_CONFIG", "1")
	t.Setenv("HOME", filepath.Join(tmpDir, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "xdg"))
	t.Chdir(tmpDir)

	config.ResetForTesting()
	t.Cleanup(config.ResetForTesting)
	if err := config.Initialize(); err != nil {
		t.Fatalf("config.Initialize: %v", err)
	}
}

// newTrackerForCreateIssue builds a Tracker backed by a test HTTP server and
// a fake tracker.Store carrying the given config values, sufficient to
// exercise CreateIssue's epic-link resolution.
func newTrackerForCreateIssue(t *testing.T, srvURL string, storeData map[string]string) *Tracker {
	t.Helper()
	tr := newTrackerWithServer(srvURL, "3")
	tr.projectKeys = []string{"PROJ"}
	tr.store = &configStore{data: storeData}
	return tr
}

// captureCreateIssueFields starts a Jira test server that records the
// "fields" object posted to POST /rest/api/3/issue and answers with a
// minimal created-issue payload followed by a GetIssue response.
func captureCreateIssueFields(t *testing.T, captured *map[string]interface{}) *httptest.Server {
	t.Helper()
	const key = "PROJ-1"
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/3/issue":
			var payload struct {
				Fields map[string]interface{} `json:"fields"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode create request: %v", err)
			}
			*captured = payload.Fields
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "10001", "key": key, "self": "self"})
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/"+key:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(issueResponse(key, "To Do"))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
}

func TestCreateIssueNoEpicKeyOmitsEpicField(t *testing.T) {
	initHermeticConfig(t)

	var captured map[string]interface{}
	srv := captureCreateIssueFields(t, &captured)
	defer srv.Close()

	tr := newTrackerForCreateIssue(t, srv.URL, map[string]string{})
	if _, err := tr.CreateIssue(context.Background(), &types.Issue{Title: "Test"}); err != nil {
		t.Fatalf("CreateIssue error: %v", err)
	}
	if _, ok := captured["parent"]; ok {
		t.Errorf("fields[parent] set with no jira.epic_key configured: %#v", captured["parent"])
	}
}

func TestCreateIssueEpicKeyDefaultsToParentField(t *testing.T) {
	initHermeticConfig(t)

	var captured map[string]interface{}
	srv := captureCreateIssueFields(t, &captured)
	defer srv.Close()

	tr := newTrackerForCreateIssue(t, srv.URL, map[string]string{
		"jira.epic_key": "PROJ-148",
	})
	if _, err := tr.CreateIssue(context.Background(), &types.Issue{Title: "Test"}); err != nil {
		t.Fatalf("CreateIssue error: %v", err)
	}

	parent, ok := captured["parent"].(map[string]interface{})
	if !ok {
		t.Fatalf("fields[parent] = %#v, want map with key PROJ-148", captured["parent"])
	}
	if parent["key"] != "PROJ-148" {
		t.Errorf("fields[parent][key] = %v, want PROJ-148", parent["key"])
	}
}

func TestCreateIssueEpicKeyUsesConfiguredClassicField(t *testing.T) {
	initHermeticConfig(t)

	var captured map[string]interface{}
	srv := captureCreateIssueFields(t, &captured)
	defer srv.Close()

	tr := newTrackerForCreateIssue(t, srv.URL, map[string]string{
		"jira.epic_key":        "PROJ-148",
		"jira.epic_link_field": "customfield_10014",
	})
	if _, err := tr.CreateIssue(context.Background(), &types.Issue{Title: "Test"}); err != nil {
		t.Fatalf("CreateIssue error: %v", err)
	}

	if _, ok := captured["parent"]; ok {
		t.Errorf("fields[parent] set = %#v, classic Jira must use the configured custom field instead", captured["parent"])
	}
	link, ok := captured["customfield_10014"].(map[string]interface{})
	if !ok {
		t.Fatalf("fields[customfield_10014] = %#v, want map with key PROJ-148", captured["customfield_10014"])
	}
	if link["key"] != "PROJ-148" {
		t.Errorf("fields[customfield_10014][key] = %v, want PROJ-148", link["key"])
	}
}

func TestCreateIssueEpicKeyWithEmptyFieldNameErrors(t *testing.T) {
	// Deliberately skip config.Initialize so the package-level SSOT default
	// for jira.epic_link_field ("parent") is unavailable, forcing the
	// all-sources-empty branch CreateIssue must reject.
	config.ResetForTesting()
	t.Cleanup(config.ResetForTesting)
	t.Setenv("JIRA_EPIC_KEY", "")
	t.Setenv("JIRA_EPIC_LINK_FIELD", "")

	var captured map[string]interface{}
	srv := captureCreateIssueFields(t, &captured)
	defer srv.Close()

	tr := newTrackerForCreateIssue(t, srv.URL, map[string]string{
		"jira.epic_key": "PROJ-148",
	})
	_, err := tr.CreateIssue(context.Background(), &types.Issue{Title: "Test"})
	if err == nil {
		t.Fatal("CreateIssue error = nil, want error for empty jira.epic_link_field")
	}
}

func TestJiraEpicLinkFieldDefaultsToParent(t *testing.T) {
	initHermeticConfig(t)

	if got := config.GetString("jira.epic_link_field"); got != "parent" {
		t.Errorf("config.GetString(jira.epic_link_field) = %q, want %q", got, "parent")
	}
}
