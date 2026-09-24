//go:build cgo

package embeddeddolt_test

import (
	"reflect"
	"testing"

	"github.com/steveyegge/beads/internal/storage/schema"
)

// TestInspectReportsPhysicalSchemaCursors builds a database whose main series
// really stopped at an older migration (its DDL, not just its cursor) and
// proves inspection reports that cursor, the complete pending tail of both
// series, and changes nothing it reads.
func TestInspectReportsPhysicalSchemaCursors(t *testing.T) {
	requireEmbedded(t)
	ctx := t.Context()
	const seededAt = 59
	dataDir := seedMainSchemaAt(t, ctx, seededAt)
	conn, closeConn := openPinnedConn(t, ctx, dataDir)
	defer closeConn()

	pendingFrom := func(from, to int) []int {
		out := []int{}
		for v := from; v <= to; v++ {
			out = append(out, v)
		}
		return out
	}

	got, err := schema.Inspect(ctx, conn)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if got.CurrentVersion != seededAt {
		t.Errorf("CurrentVersion = %d, want %d", got.CurrentVersion, seededAt)
	}
	if got.LatestVersion != schema.LatestVersion() {
		t.Errorf("LatestVersion = %d, want %d", got.LatestVersion, schema.LatestVersion())
	}
	if want := pendingFrom(seededAt+1, schema.LatestVersion()); !reflect.DeepEqual(got.PendingVersions, want) {
		t.Errorf("PendingVersions = %v, want %v", got.PendingVersions, want)
	}
	// The seed applies only the main series, so the ignored cursor is absent.
	if got.CurrentIgnoredVersion != 0 {
		t.Errorf("CurrentIgnoredVersion = %d, want 0", got.CurrentIgnoredVersion)
	}
	if want := pendingFrom(1, schema.LatestIgnoredVersion()); !reflect.DeepEqual(got.PendingIgnoredVersions, want) {
		t.Errorf("PendingIgnoredVersions = %v, want %v", got.PendingIgnoredVersions, want)
	}
	if v := currentMainVersion(t, ctx, conn); v != seededAt {
		t.Errorf("schema_migrations after Inspect = %d, want unchanged %d", v, seededAt)
	}
	dirty, err := statusTables(ctx, conn)
	if err != nil {
		t.Fatalf("dolt_status: %v", err)
	}
	if len(dirty) != 0 {
		t.Errorf("Inspect dirtied the working set: %v", dirty)
	}
}
