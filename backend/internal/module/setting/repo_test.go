package setting

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
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
	return noopStmt{}, nil
}
func (noopConn) Close() error              { return nil }
func (noopConn) Begin() (driver.Tx, error) { return noopTx{}, nil }

type noopStmt struct{}

func (noopStmt) Close() error                               { return nil }
func (noopStmt) NumInput() int                              { return -1 }
func (noopStmt) Exec([]driver.Value) (driver.Result, error) { return noopResult{}, nil }
func (noopStmt) Query([]driver.Value) (driver.Rows, error)  { return noopRows{}, nil }

type noopTx struct{}

func (noopTx) Commit() error   { return nil }
func (noopTx) Rollback() error { return nil }

type noopResult struct{}

func (noopResult) LastInsertId() (int64, error) { return 0, nil }
func (noopResult) RowsAffected() (int64, error) { return 0, nil }

type noopRows struct{}

func (noopRows) Columns() []string         { return []string{} }
func (noopRows) Close() error              { return nil }
func (noopRows) Next([]driver.Value) error { return io.EOF }

func (l *sqlCaptureLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, _ := fc()
	l.statements = append(l.statements, sql)
}

func newDryRunSettingRepo(t *testing.T, capture *sqlCaptureLogger) Repository {
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

func TestUpsertStoreSettingUsesTypedStorePredicate(t *testing.T) {
	capture := &sqlCaptureLogger{Interface: logger.Default.LogMode(logger.Silent)}
	repo := newDryRunSettingRepo(t, capture)
	storeID := int64(1)

	err := repo.Upsert(&SystemSetting{StoreID: &storeID, Key: "feature.sms_enabled", Value: "false"})
	if err != nil {
		t.Fatalf("Upsert error = %v", err)
	}

	for _, statement := range capture.statements {
		if strings.Contains(statement, "IS NULL AND") {
			t.Fatalf("store-specific upsert should not use an untyped IS NULL parameter, got SQL: %s", statement)
		}
	}
}
