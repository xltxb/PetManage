package appointment

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type sqlCaptureLogger struct {
	logger.Interface
	statements []string
}

type noopConnector struct{}

func (noopConnector) Connect(context.Context) (driver.Conn, error) { return noopConn{}, nil }
func (noopConnector) Driver() driver.Driver                        { return noopDriver{} }

type noopDriver struct{}

func (noopDriver) Open(string) (driver.Conn, error) { return noopConn{}, nil }

type noopConn struct{}

func (noopConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("noop connection is dry-run only")
}
func (noopConn) Close() error              { return nil }
func (noopConn) Begin() (driver.Tx, error) { return nil, errors.New("noop connection is dry-run only") }

func (l *sqlCaptureLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, _ := fc()
	l.statements = append(l.statements, sql)
}

func newDryRunAppointmentRepo(t *testing.T, capture *sqlCaptureLogger) Repository {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sql.OpenDB(noopConnector{})}), &gorm.Config{
		DryRun: true,
		Logger: capture,
	})
	if err != nil {
		t.Fatalf("open dry-run db: %v", err)
	}
	return NewRepository(db)
}

func TestListByStoreOmitsStoreFilterForWildcardScope(t *testing.T) {
	capture := &sqlCaptureLogger{Interface: logger.Default.LogMode(logger.Silent)}
	repo := newDryRunAppointmentRepo(t, capture)

	_, _, err := repo.ListByStore(0, "", time.Time{}, time.Time{}, 1, 20)
	if err != nil {
		t.Fatalf("ListByStore wildcard error = %v", err)
	}

	for _, statement := range capture.statements {
		if strings.Contains(statement, "store_id") {
			t.Fatalf("wildcard query should not filter by store_id, got SQL: %s", statement)
		}
	}
}

func TestListByStoreKeepsStoreFilterForSingleStoreScope(t *testing.T) {
	capture := &sqlCaptureLogger{Interface: logger.Default.LogMode(logger.Silent)}
	repo := newDryRunAppointmentRepo(t, capture)

	_, _, err := repo.ListByStore(1, "", time.Time{}, time.Time{}, 1, 20)
	if err != nil {
		t.Fatalf("ListByStore single-store error = %v", err)
	}

	for _, statement := range capture.statements {
		if strings.Contains(statement, "store_id") {
			return
		}
	}
	t.Fatalf("single-store query should filter by store_id, got SQL statements: %#v", capture.statements)
}
