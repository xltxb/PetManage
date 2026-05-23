package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// Migration holds a single migration's up and down SQL.
type Migration struct {
	Version     string
	Description string
	UpSQL       string
	DownSQL     string
}

// Migrator runs database migrations.
type Migrator struct {
	db            *sql.DB
	migrationsDir string
	tableName     string
}

// NewMigrator creates a new Migrator.
func NewMigrator(db *sql.DB, migrationsDir string) *Migrator {
	return &Migrator{
		db:            db,
		migrationsDir: migrationsDir,
		tableName:     "pet_migrations",
	}
}

// Init creates the migration tracking table if it does not exist.
func (m *Migrator) Init() error {
	_, err := m.db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`, m.tableName))
	return err
}

// LoadMigrations reads all .up.sql files from the migrations directory
// and pairs them with their .down.sql counterparts if present.
func (m *Migrator) LoadMigrations() ([]Migration, error) {
	entries, err := os.ReadDir(m.migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("reading migrations dir: %w", err)
	}

	seen := make(map[string]*Migration)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		parts := strings.SplitN(name, "_", 2)
		if len(parts) < 2 {
			continue
		}
		version := parts[0]
		desc := strings.TrimSuffix(parts[1], ".up.sql")

		upSQL, err := os.ReadFile(filepath.Join(m.migrationsDir, name))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}

		mig := &Migration{
			Version:     version,
			Description: desc,
			UpSQL:       string(upSQL),
		}

		downFile := filepath.Join(m.migrationsDir, version+"_"+desc+".down.sql")
		if downData, err := os.ReadFile(downFile); err == nil {
			mig.DownSQL = string(downData)
		}

		seen[version] = mig
	}

	var versions []string
	for v := range seen {
		versions = append(versions, v)
	}
	sort.Strings(versions)

	var result []Migration
	for _, v := range versions {
		result = append(result, *seen[v])
	}
	return result, nil
}

// appliedVersions returns the set of already-applied migration versions.
func (m *Migrator) appliedVersions() (map[string]bool, error) {
	rows, err := m.db.Query(fmt.Sprintf("SELECT version FROM %s", m.tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

// Migrate applies all pending migrations in version order. Already-applied
// migrations are skipped, making repeated runs idempotent.
func (m *Migrator) Migrate() error {
	if err := m.Init(); err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}

	migrations, err := m.LoadMigrations()
	if err != nil {
		return err
	}

	applied, err := m.appliedVersions()
	if err != nil {
		return fmt.Errorf("checking applied versions: %w", err)
	}

	for _, mig := range migrations {
		if applied[mig.Version] {
			fmt.Printf("  [SKIP] %s_%s (already applied)\n", mig.Version, mig.Description)
			continue
		}

		fmt.Printf("  [MIGRATE] %s_%s\n", mig.Version, mig.Description)
		tx, err := m.db.Begin()
		if err != nil {
			return err
		}

		if _, err := tx.Exec(mig.UpSQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s failed: %w", mig.Version, err)
		}

		if _, err := tx.Exec(
			fmt.Sprintf("INSERT INTO %s (version, applied_at) VALUES ($1, $2)", m.tableName),
			mig.Version, time.Now(),
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording migration %s: %w", mig.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", mig.Version, err)
		}
	}

	return nil
}

// Rollback reverts the most recently applied migration by running its down
// SQL and removing the tracking record.
func (m *Migrator) Rollback() error {
	if err := m.Init(); err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}

	var latestVersion string
	err := m.db.QueryRow(fmt.Sprintf(
		"SELECT version FROM %s ORDER BY applied_at DESC LIMIT 1", m.tableName,
	)).Scan(&latestVersion)
	if err == sql.ErrNoRows {
		return fmt.Errorf("no migrations to rollback")
	}
	if err != nil {
		return fmt.Errorf("finding latest migration: %w", err)
	}

	migrations, err := m.LoadMigrations()
	if err != nil {
		return err
	}

	var target *Migration
	for i := range migrations {
		if migrations[i].Version == latestVersion {
			target = &migrations[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("migration %s not found on disk", latestVersion)
	}
	if target.DownSQL == "" {
		return fmt.Errorf("no down migration for %s", latestVersion)
	}

	fmt.Printf("  [ROLLBACK] %s_%s\n", target.Version, target.Description)
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}

	if _, err := tx.Exec(target.DownSQL); err != nil {
		tx.Rollback()
		return fmt.Errorf("rollback %s failed: %w", target.Version, err)
	}

	if _, err := tx.Exec(
		fmt.Sprintf("DELETE FROM %s WHERE version = $1", m.tableName),
		target.Version,
	); err != nil {
		tx.Rollback()
		return fmt.Errorf("removing migration record %s: %w", target.Version, err)
	}

	return tx.Commit()
}

// Status prints all migrations and their applied/not-applied state.
func (m *Migrator) Status() error {
	if err := m.Init(); err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}

	migrations, err := m.LoadMigrations()
	if err != nil {
		return err
	}

	applied, err := m.appliedVersions()
	if err != nil {
		return fmt.Errorf("checking applied versions: %w", err)
	}

	for _, mig := range migrations {
		state := "pending"
		if applied[mig.Version] {
			state = "applied"
		}
		fmt.Printf("  [%s] %s_%s\n", state, mig.Version, mig.Description)
	}
	return nil
}

// Connect opens a PostgreSQL connection and verifies it with a ping.
func Connect(dsn string, maxOpen, maxIdle int) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}
	if maxOpen > 0 {
		db.SetMaxOpenConns(maxOpen)
	}
	if maxIdle > 0 {
		db.SetMaxIdleConns(maxIdle)
	}
	db.SetConnMaxLifetime(5 * time.Minute)
	return db, nil
}
