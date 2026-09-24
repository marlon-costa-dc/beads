package metrics

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestResolveHardStopDefaultsWithoutOverride pins resolveHardStop's SSOT
// default: with no BEADS_METRICS_HARD_STOP set, the watchdog uses the
// package's own hardStop constant, not some zero or environment-shaped value.
func TestResolveHardStopDefaultsWithoutOverride(t *testing.T) {
	t.Setenv(EnvHardStop, "")
	os.Unsetenv(EnvHardStop)

	got, err := resolveHardStop()
	if err != nil {
		t.Fatalf("resolveHardStop() error = %v, want nil", err)
	}
	if got != hardStop {
		t.Errorf("resolveHardStop() = %s, want default %s", got, hardStop)
	}
}

// TestResolveHardStopParsesValidOverride proves a valid BEADS_METRICS_HARD_STOP
// value is parsed as a Go duration and replaces the default exactly.
func TestResolveHardStopParsesValidOverride(t *testing.T) {
	t.Setenv(EnvHardStop, "90s")

	got, err := resolveHardStop()
	if err != nil {
		t.Fatalf("resolveHardStop() error = %v, want nil", err)
	}
	if want := 90 * time.Second; got != want {
		t.Errorf("resolveHardStop() = %s, want %s", got, want)
	}
}

// TestResolveHardStopRejectsUnparsableOverride proves a malformed override is
// a surfaced configuration error, never a silent fall-back to the default.
func TestResolveHardStopRejectsUnparsableOverride(t *testing.T) {
	t.Setenv(EnvHardStop, "not-a-duration")

	got, err := resolveHardStop()
	if err == nil {
		t.Fatalf("resolveHardStop() error = nil, want error for unparsable %s", EnvHardStop)
	}
	if got != 0 {
		t.Errorf("resolveHardStop() = %s on error, want zero value", got)
	}
	if !strings.Contains(err.Error(), EnvHardStop) || !strings.Contains(err.Error(), "not-a-duration") {
		t.Errorf("resolveHardStop() error = %q, want it to name %s and the bad value", err, EnvHardStop)
	}
}

// TestResolveHardStopRejectsNonPositiveOverride proves a syntactically valid
// but non-positive duration (zero or negative) is rejected rather than
// silently arming a watchdog that fires immediately or never.
func TestResolveHardStopRejectsNonPositiveOverride(t *testing.T) {
	for _, v := range []string{"0s", "-1s"} {
		t.Run(v, func(t *testing.T) {
			t.Setenv(EnvHardStop, v)

			got, err := resolveHardStop()
			if err == nil {
				t.Fatalf("resolveHardStop() error = nil, want error for non-positive %s=%q", EnvHardStop, v)
			}
			if got != 0 {
				t.Errorf("resolveHardStop() = %s on error, want zero value", got)
			}
			if !strings.Contains(err.Error(), "must be positive") {
				t.Errorf("resolveHardStop() error = %q, want it to say the value must be positive", err)
			}
		})
	}
}

// TestWatchdogMessageFormatsActualDuration pins the watchdog's stderr message
// to the resolved duration instead of a hardcoded literal: an override must
// be reflected verbatim in the message, not the compiled-in default.
func TestWatchdogMessageFormatsActualDuration(t *testing.T) {
	t.Setenv(EnvHardStop, "45s")

	stop, err := resolveHardStop()
	if err != nil {
		t.Fatalf("resolveHardStop() error = %v, want nil", err)
	}

	got := watchdogMessage(stop)
	want := "send-metrics: hard stop after 45s (wedged lock or stall); exiting\n"
	if got != want {
		t.Errorf("watchdogMessage(%s) = %q, want %q", stop, got, want)
	}
}
