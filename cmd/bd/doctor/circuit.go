package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/steveyegge/beads/internal/storage/dolt"
)

// CheckCircuitBreaker checks for stale circuit breaker state files that may
// block all bd operations. Returns a fixable DoctorCheck if stale files exist.
func CheckCircuitBreaker() DoctorCheck {
	// The storage layer owns the breaker directory; asking it keeps doctor
	// looking where live state is actually written.
	dir, err := dolt.CircuitBreakerDir()
	if err != nil {
		return DoctorCheck{
			Name:     "Circuit Breaker",
			Status:   StatusError,
			Message:  fmt.Sprintf("Cannot resolve circuit breaker state directory: %v", err),
			Category: CategoryRuntime,
		}
	}
	pattern := filepath.Join(dir, "beads-dolt-circuit-*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return DoctorCheck{
			Name:     "Circuit Breaker",
			Status:   StatusError,
			Message:  fmt.Sprintf("Cannot list circuit breaker state files in %s: %v", dir, err),
			Category: CategoryRuntime,
		}
	}
	if len(matches) == 0 {
		return DoctorCheck{
			Name:     "Circuit Breaker",
			Status:   StatusOK,
			Message:  "No stale circuit breaker files",
			Category: CategoryRuntime,
		}
	}

	staleCount := 0
	for _, path := range matches {
		data, err := os.ReadFile(path) //nolint:gosec // G304: path is from filepath.Glob with controlled pattern
		if err != nil {
			continue
		}
		var state struct {
			State     string    `json:"state"`
			TrippedAt time.Time `json:"tripped_at,omitempty"`
			LastFail  time.Time `json:"last_failure,omitempty"`
		}
		if err := json.Unmarshal(data, &state); err != nil {
			staleCount++ // corrupt file counts as stale
			continue
		}
		if state.State == "open" || state.State == "half-open" {
			// Only flag as stale if the breaker has been tripped for longer
			// than the staleness TTL (5 minutes). A recently-tripped breaker
			// during a real outage should not be cleared.
			ref := state.TrippedAt
			if ref.IsZero() {
				ref = state.LastFail
			}
			if ref.IsZero() || time.Since(ref) > 5*time.Minute {
				staleCount++
			}
		}
	}

	if staleCount == 0 {
		return DoctorCheck{
			Name:     "Circuit Breaker",
			Status:   StatusOK,
			Message:  "No stale circuit breaker files",
			Category: CategoryRuntime,
		}
	}

	return DoctorCheck{
		Name:     "Circuit Breaker",
		Status:   StatusWarning,
		Message:  fmt.Sprintf("%d stale circuit breaker file(s) found in %s", staleCount, dir),
		Fix:      "Run 'bd doctor --fix' to clear stale circuit breaker files",
		Category: CategoryRuntime,
	}
}
