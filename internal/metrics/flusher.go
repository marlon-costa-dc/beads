package metrics

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dolthub/eventkit"
	ga4tx "github.com/dolthub/eventkit/transport/ga4"
)

const (
	EnvEndpoint = "BEADS_METRICS_ENDPOINT"

	flushTimeout = 30 * time.Second

	// EnvHardStop overrides hardStop with a parsed time.Duration (e.g.
	// "90s"). A set-but-invalid value is a configuration error surfaced to
	// the caller, not silently replaced by the default; see resolveHardStop.
	EnvHardStop = "BEADS_METRICS_HARD_STOP"

	// hardStop is the default process-level backstop for the flusher child.
	// The ctx bound above covers the POST itself, but the child also runs
	// queue pruning and collector teardown outside that context; a lock
	// left wedged by a SIGKILLed parent must not turn the flusher into an
	// immortal orphan. When the watchdog fires the child exits in place —
	// worst case a batch is re-sent on the next flush, which the queue's
	// idempotent batching already tolerates.
	hardStop = 60 * time.Second
)

func RunSendMetrics() int {
	stop, err := resolveHardStop()
	if err != nil {
		fmt.Fprintf(os.Stderr, "send-metrics: %v\n", err)
		return 1
	}

	// Arm the watchdog before anything else: DataDir, pruning and teardown
	// all run outside the flush ctx and none of them is allowed to wedge
	// this process past stop.
	watchdog := time.AfterFunc(stop, func() {
		fmt.Fprint(os.Stderr, watchdogMessage(stop))
		os.Exit(1)
	})
	defer watchdog.Stop()

	dir, err := DataDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "send-metrics: %v\n", err)
		return 1
	}

	// Bound the queue before flushing: TTL out stale batches and orphaned
	// emitter temps, cap the rest drop-oldest (bd-ulfod: an unbounded queue
	// reached 149k files / 1.1GB when emission outran the throttled drain).
	// Out-of-band by construction — this child is already detached.
	if dropped, freed := PruneQueue(dir, time.Now()); dropped > 0 {
		fmt.Fprintf(os.Stderr, "send-metrics: pruned %d queued event file(s), freed %.1f MB\n",
			dropped, float64(freed)/(1<<20))
	}

	// With telemetry disabled this child exists only for the prune above:
	// nothing may be POSTed, but the backlog an earlier enabled configuration
	// queued still has to decay. The old ordering (enabled check first)
	// stranded eventsData forever on exactly the machine that just opted out —
	// 2M+ files / 15.8GB observed on one control VM (GH#5712).
	if !Enabled() {
		return 0
	}

	ga, err := ga4tx.New(ga4tx.Config{Endpoint: Endpoint()})
	if err != nil {
		fmt.Fprintf(os.Stderr, "send-metrics: ga4: %v\n", err)
		return 1
	}

	flusher := eventkit.NewFileFlusher(dir, ga)
	ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()
	if err := flusher.Flush(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "send-metrics: flush: %v\n", err)
		return 1
	}
	return 0
}

// resolveHardStop resolves the flusher watchdog duration: hardStop unless
// EnvHardStop is set, in which case it must parse as a positive
// time.Duration. An invalid or non-positive override is a configuration
// error returned to the caller — never silently replaced by the default,
// and never a partially-applied value.
func resolveHardStop() (time.Duration, error) {
	v := os.Getenv(EnvHardStop)
	if v == "" {
		return hardStop, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s=%q: %w", EnvHardStop, v, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s=%q: must be positive", EnvHardStop, v)
	}
	return d, nil
}

// watchdogMessage formats the watchdog's stderr message from the resolved
// duration, so an EnvHardStop override is reflected verbatim rather than a
// literal that could drift from the constant it replaces.
func watchdogMessage(stop time.Duration) string {
	return fmt.Sprintf("send-metrics: hard stop after %s (wedged lock or stall); exiting\n", stop)
}
