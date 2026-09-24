package schema

import (
	"context"

	"github.com/steveyegge/beads/internal/storage"
)

// Inspect reads both migration cursors and the migrations this binary would
// still apply to db, without mutating it. Each cursor is read once, so the
// pending list always agrees with the version it is reported against. It is
// the single implementation behind every backend's storage.SchemaInspector.
func Inspect(ctx context.Context, db DBConn) (storage.SchemaInspection, error) {
	current, err := mainSource.currentVersion(ctx, db)
	if err != nil {
		return storage.SchemaInspection{}, err
	}
	ignored, err := ignoredSource.currentVersion(ctx, db)
	if err != nil {
		return storage.SchemaInspection{}, err
	}
	return storage.SchemaInspection{
		CurrentVersion:         current,
		LatestVersion:          LatestVersion(),
		PendingVersions:        mainSource.pendingAfter(current),
		CurrentIgnoredVersion:  ignored,
		LatestIgnoredVersion:   LatestIgnoredVersion(),
		PendingIgnoredVersions: ignoredSource.pendingAfter(ignored),
	}, nil
}
