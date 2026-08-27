package schema

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

var eventsTable = auxRekeyTables[0]

// Dolt surfaces schema-encoding drift as a recovered server panic. Migration
// 0057 repairs that drift before the schema-62 re-key passes run, so seeing it
// here is an integrity failure: the pass must stop instead of silently marking
// a partially rewritten table as complete.
var driftErr = errors.New("Error 1105 (HY000): panic recovered: invalid hash length: 19")

func expectEventsSelect(mock sqlmock.Sqlmock) *sqlmock.ExpectedQuery {
	return mock.ExpectQuery(regexp.QuoteMeta(
		fmt.Sprintf("SELECT id, %s FROM %s", eventsTable.columns, eventsTable.name)))
}

func TestRekeyAuxRowTableFailsClosedOnSchemaEncodingDriftRead(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	expectColumnExists(mock, true)
	expectEventsSelect(mock).WillReturnError(driftErr)

	wrote, err := rekeyAuxRowTable(context.Background(), db, eventsTable)
	if err == nil || !strings.Contains(err.Error(), driftErr.Error()) {
		t.Fatalf("rekeyAuxRowTable error = %v, want schema-encoding drift", err)
	}
	if wrote {
		t.Error("expected wrote=false when the table cannot be read")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestRekeyAuxRowTableFailsClosedOnSchemaEncodingDriftUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	expectColumnExists(mock, true)
	expectEventsSelect(mock).WillReturnRows(sqlmock.NewRows([]string{
		"id", "issue_id", "event_type", "actor", "old_value", "new_value", "comment", "created_at",
	}).AddRow("random-id", "bd-1", "created", "agent", nil, nil, "comment", "2026-08-26 00:00:00"))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE events SET id = ? WHERE id = ?")).
		WithArgs(sqlmock.AnyArg(), "random-id").
		WillReturnError(driftErr)

	wrote, err := rekeyAuxRowTable(context.Background(), db, eventsTable)
	if err == nil || !strings.Contains(err.Error(), driftErr.Error()) {
		t.Fatalf("rekeyAuxRowTable error = %v, want schema-encoding drift", err)
	}
	if !wrote {
		t.Error("expected wrote=true after beginning the rewrite")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
