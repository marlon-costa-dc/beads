package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/steveyegge/beads/internal/storage/dolt"
)

// withUserCache points os.UserCacheDir at a fresh directory on every platform
// and returns the breaker directory the storage layer resolves inside it.
func withUserCache(t *testing.T) string {
	t.Helper()
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("LocalAppData", cache)
	dir, err := dolt.CircuitBreakerDir()
	if err != nil {
		t.Fatalf("dolt.CircuitBreakerDir() error = %v", err)
	}
	return dir
}

func writeBreakerState(t *testing.T, dir, name string, state map[string]any) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create breaker dir: %v", err)
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal breaker state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		t.Fatalf("write breaker state: %v", err)
	}
}

func TestCheckCircuitBreaker_NoStateIsOK(t *testing.T) {
	withUserCache(t)

	check := CheckCircuitBreaker()
	if check.Status != StatusOK {
		t.Fatalf("status = %q (%s), want %q", check.Status, check.Message, StatusOK)
	}
}

func TestCheckCircuitBreaker_StaleOpenStateInUserCacheWarns(t *testing.T) {
	dir := withUserCache(t)
	writeBreakerState(t, dir, "beads-dolt-circuit-127-0-0-1-3307-proj.json", map[string]any{
		"state":      "open",
		"tripped_at": time.Now().Add(-time.Hour),
	})

	check := CheckCircuitBreaker()
	if check.Status != StatusWarning {
		t.Fatalf("status = %q (%s), want %q", check.Status, check.Message, StatusWarning)
	}
	if !strings.Contains(check.Message, dir) {
		t.Fatalf("message %q does not name the user-cache breaker directory %q", check.Message, dir)
	}
}

func TestCheckCircuitBreaker_UnresolvableCacheIsAnError(t *testing.T) {
	// os.UserCacheDir fails on every platform once its inputs are empty.
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	t.Setenv("LocalAppData", "")

	check := CheckCircuitBreaker()
	if check.Status != StatusError {
		t.Fatalf("status = %q (%s), want %q", check.Status, check.Message, StatusError)
	}
}
