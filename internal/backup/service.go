package backup

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/xltxb/PetManage/internal/config"
	"github.com/xltxb/PetManage/internal/cron"
	"go.uber.org/zap"
)

// Service manages database backups.
type Service struct {
	db     *sql.DB
	cfg    config.BackupConfig
	dsn    string
	logger *zap.Logger
}

// Record represents a backup record from the database.
type Record struct {
	ID           int64      `json:"id"`
	BackupType   string     `json:"backup_type"`
	FilePath     string     `json:"file_path"`
	FileSize     int64      `json:"file_size"`
	Status       string     `json:"status"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	ErrorMessage *string    `json:"error_message,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// NewService creates a new backup Service.
func NewService(db *sql.DB, cfg config.BackupConfig, dsn string, logger *zap.Logger) (*Service, error) {
	if err := os.MkdirAll(cfg.BackupDir, 0755); err != nil {
		return nil, fmt.Errorf("creating backup dir: %w", err)
	}
	return &Service{db: db, cfg: cfg, dsn: dsn, logger: logger}, nil
}

// FullBackup performs a full database backup using pg_dump.
func (s *Service) FullBackup() (*Record, error) {
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("full_%s.dump.gz", timestamp)
	filePath := filepath.Join(s.cfg.BackupDir, filename)

	rec, err := s.insertRecord("full", filePath)
	if err != nil {
		return nil, fmt.Errorf("inserting record: %w", err)
	}

	if err := s.pgDump(filePath); err != nil {
		s.updateRecord(rec.ID, "failed", err.Error())
		return s.getRecord(rec.ID)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		s.updateRecord(rec.ID, "failed", err.Error())
		return s.getRecord(rec.ID)
	}

	now := time.Now()
	_, err = s.db.Exec(
		`UPDATE backup_records SET file_size=$1, status='completed', completed_at=$2 WHERE id=$3`,
		info.Size(), now, rec.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("updating record: %w", err)
	}

	s.logger.Info("Full backup completed",
		zap.Int64("id", rec.ID),
		zap.String("file", filename),
		zap.Int64("size", info.Size()),
	)

	rec.FileSize = info.Size()
	rec.Status = "completed"
	rec.CompletedAt = &now
	return rec, nil
}

// IncrementalBackup captures data changed since the last backup.
func (s *Service) IncrementalBackup() (*Record, error) {
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("incr_%s.sql.gz", timestamp)
	filePath := filepath.Join(s.cfg.BackupDir, filename)

	rec, err := s.insertRecord("incremental", filePath)
	if err != nil {
		return nil, fmt.Errorf("inserting record: %w", err)
	}

	lastBackup, err := s.getLastCompletedBackupTime()
	if err != nil {
		s.updateRecord(rec.ID, "failed", err.Error())
		return s.getRecord(rec.ID)
	}

	var buf bytes.Buffer
	writer := io.Writer(&buf)

	if s.cfg.Compression {
		gzWriter := gzip.NewWriter(&buf)
		defer gzWriter.Close()
		writer = gzWriter
	}

	header := fmt.Sprintf("-- Incremental backup: %s\n-- Changes since: %s\n\n",
		time.Now().Format(time.RFC3339),
		lastBackup.Format(time.RFC3339),
	)
	writer.Write([]byte(header))

	count, err := s.dumpChangedRows(writer, lastBackup)
	if err != nil {
		if gzWriter, ok := writer.(*gzip.Writer); ok {
			gzWriter.Close()
		}
		s.updateRecord(rec.ID, "failed", err.Error())
		return s.getRecord(rec.ID)
	}

	if gzW, ok := writer.(*gzip.Writer); ok {
		gzW.Close()
	}

	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		s.updateRecord(rec.ID, "failed", err.Error())
		return s.getRecord(rec.ID)
	}

	info, _ := os.Stat(filePath)
	now := time.Now()
	_, err = s.db.Exec(
		`UPDATE backup_records SET file_size=$1, status='completed', completed_at=$2 WHERE id=$3`,
		info.Size(), now, rec.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("updating record: %w", err)
	}

	s.logger.Info("Incremental backup completed",
		zap.Int64("id", rec.ID),
		zap.String("file", filename),
		zap.Int64("size", info.Size()),
		zap.Int("tables_affected", count),
	)

	rec.FileSize = info.Size()
	rec.Status = "completed"
	rec.CompletedAt = &now
	return rec, nil
}

// Restore restores the database from a backup file.
func (s *Service) Restore(backupID int64) error {
	rec, err := s.getRecord(backupID)
	if err != nil {
		return fmt.Errorf("finding record %d: %w", backupID, err)
	}
	if rec == nil {
		return fmt.Errorf("backup record %d not found", backupID)
	}
	if rec.Status != "completed" {
		return fmt.Errorf("backup %d is not completed (status: %s)", backupID, rec.Status)
	}

	if _, err := os.Stat(rec.FilePath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", rec.FilePath)
	}

	if rec.BackupType == "full" {
		return s.pgRestore(rec.FilePath)
	}

	// For incremental backups, restore via psql.
	return s.psqlRestore(rec.FilePath)
}

// ListBackups returns all backup records, most recent first.
func (s *Service) ListBackups() ([]Record, error) {
	rows, err := s.db.Query(
		`SELECT id, backup_type, file_path, file_size, status, started_at, completed_at, error_message, created_at
		 FROM backup_records ORDER BY started_at DESC LIMIT 100`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.ID, &r.BackupType, &r.FilePath, &r.FileSize,
			&r.Status, &r.StartedAt, &r.CompletedAt, &r.ErrorMessage, &r.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// CleanOldBackups removes backup files and records older than retention days.
func (s *Service) CleanOldBackups() (int, error) {
	cutoff := time.Now().AddDate(0, 0, -s.cfg.RetentionDays)

	rows, err := s.db.Query(
		`SELECT id, file_path FROM backup_records WHERE created_at < $1`, cutoff,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var toDelete []struct {
		id   int64
		path string
	}
	for rows.Next() {
		var d struct {
			id   int64
			path string
		}
		if err := rows.Scan(&d.id, &d.path); err != nil {
			return 0, err
		}
		toDelete = append(toDelete, d)
	}

	count := 0
	for _, d := range toDelete {
		os.Remove(d.path)
		if _, err := s.db.Exec(`DELETE FROM backup_records WHERE id=$1`, d.id); err == nil {
			count++
		}
	}

	return count, nil
}

// RegisterSchedulerJobs registers backup jobs with the cron scheduler.
func (s *Service) RegisterSchedulerJobs(scheduler *cron.Scheduler) {
	fullJob := cron.Job{
		Name: "full_backup",
		NextRun: func(now time.Time) time.Time {
			target, _ := time.Parse("15:04", s.cfg.FullBackupTime)
			next := time.Date(now.Year(), now.Month(), now.Day(),
				target.Hour(), target.Minute(), 0, 0, now.Location())
			if next.Before(now) {
				next = next.Add(24 * time.Hour)
			}
			return next
		},
		Fn: func() error {
			_, err := s.FullBackup()
			return err
		},
	}
	scheduler.AddJob(fullJob)

	incrJob := cron.Job{
		Name:     "incremental_backup",
		Interval: time.Duration(s.cfg.IncrementalIntervalMin) * time.Minute,
		Fn: func() error {
			_, err := s.IncrementalBackup()
			return err
		},
	}
	scheduler.AddJob(incrJob)
}

func (s *Service) pgDump(filePath string) error {
	parts := strings.Fields(s.dsn)
	args := []string{"-Fc", "--no-owner", "--no-acl"}
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "host":
			args = append(args, "-h", kv[1])
		case "port":
			args = append(args, "-p", kv[1])
		case "user":
			args = append(args, "-U", kv[1])
		case "dbname":
			args = append(args, "-d", kv[1])
		}
	}

	// Set PGPASSWORD for non-interactive auth.
	password := ""
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) == 2 && kv[0] == "password" {
			password = kv[1]
			break
		}
	}

	cmd := exec.Command("pg_dump", args...)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+password)

	if s.cfg.Compression || strings.HasSuffix(filePath, ".gz") {
		var buf bytes.Buffer
		cmd.Stdout = &buf
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("pg_dump failed: %w, stderr: %s", err, stderr.String())
		}

		gzFile, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("creating gz file: %w", err)
		}
		defer gzFile.Close()

		gzWriter := gzip.NewWriter(gzFile)
		defer gzWriter.Close()

		if _, err := gzWriter.Write(buf.Bytes()); err != nil {
			return fmt.Errorf("writing gz: %w", err)
		}
		return nil
	}

	outFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dump failed: %w, stderr: %s", err, stderr.String())
	}
	return nil
}

// pgRestore restores from a full backup using pg_restore.
func (s *Service) pgRestore(filePath string) error {
	parts := strings.Fields(s.dsn)
	args := []string{"-c", "--if-exists", "--no-owner", "--no-acl"}
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "host":
			args = append(args, "-h", kv[1])
		case "port":
			args = append(args, "-p", kv[1])
		case "user":
			args = append(args, "-U", kv[1])
		case "dbname":
			args = append(args, "-d", kv[1])
		}
	}

	password := ""
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) == 2 && kv[0] == "password" {
			password = kv[1]
			break
		}
	}

	// Handle compressed files.
	var input io.Reader
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening backup file: %w", err)
	}
	defer f.Close()

	if strings.HasSuffix(filePath, ".gz") {
		gzReader, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("creating gzip reader: %w", err)
		}
		defer gzReader.Close()
		input = gzReader
	} else {
		input = f
	}

	cmd := exec.Command("pg_restore", args...)
	cmd.Stdin = input
	cmd.Env = append(os.Environ(), "PGPASSWORD="+password)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_restore failed: %w, stderr: %s", err, stderr.String())
	}
	return nil
}

// psqlRestore restores from an incremental backup SQL file.
func (s *Service) psqlRestore(filePath string) error {
	parts := strings.Fields(s.dsn)
	args := []string{}
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "host":
			args = append(args, "-h", kv[1])
		case "port":
			args = append(args, "-p", kv[1])
		case "user":
			args = append(args, "-U", kv[1])
		case "dbname":
			args = append(args, "-d", kv[1])
		}
	}

	password := ""
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) == 2 && kv[0] == "password" {
			password = kv[1]
			break
		}
	}

	var input io.Reader
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening SQL file: %w", err)
	}
	defer f.Close()

	if strings.HasSuffix(filePath, ".gz") {
		gzReader, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("creating gzip reader: %w", err)
		}
		defer gzReader.Close()
		input = gzReader
	} else {
		input = f
	}

	cmd := exec.Command("psql", args...)
	cmd.Stdin = input
	cmd.Env = append(os.Environ(), "PGPASSWORD="+password)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("psql restore failed: %w, stderr: %s", err, stderr.String())
	}
	return nil
}

// dumpChangedRows writes SQL INSERT statements for rows changed since the given time.
func (s *Service) dumpChangedRows(w io.Writer, since time.Time) (int, error) {
	tables, err := s.getTablesWithTimestamp()
	if err != nil {
		return 0, err
	}

	ctx := context.Background()
	count := 0

	for _, t := range tables {
		cols, err := s.getTableColumns(t)
		if err != nil {
			s.logger.Warn("Skipping table, cannot get columns", zap.String("table", t), zap.Error(err))
			continue
		}

		colList := strings.Join(cols, ", ")
		placeholders := make([]string, len(cols))
		for i := range placeholders {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}
		phList := strings.Join(placeholders, ", ")

		query := fmt.Sprintf(
			`SELECT %s FROM %s WHERE updated_at > $1 ORDER BY updated_at`,
			colList, t,
		)

		rows, err := s.db.QueryContext(ctx, query, since)
		if err != nil {
			s.logger.Debug("Skipping table (query error)",
				zap.String("table", t),
				zap.Error(err),
			)
			continue
		}

		rowCount := 0
		for rows.Next() {
			vals := make([]interface{}, len(cols))
			valPtrs := make([]interface{}, len(cols))
			for i := range vals {
				valPtrs[i] = &vals[i]
			}

			if err := rows.Scan(valPtrs...); err != nil {
				rows.Close()
				return count, fmt.Errorf("scanning %s: %w", t, err)
			}

			insert := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);\n", t, colList, phList)
			// Render values into the INSERT by replacing placeholders.
			for i, v := range vals {
				insert = strings.Replace(insert, placeholders[i], formatValue(v), 1)
			}

			w.Write([]byte(insert))
			rowCount++
		}
		rows.Close()

		if rowCount > 0 {
			count++
			s.logger.Debug("Dumped changed rows",
				zap.String("table", t),
				zap.Int("rows", rowCount),
			)
		}
	}

	return count, nil
}

// getTablesWithTimestamp returns all tables that have an updated_at column.
func (s *Service) getTablesWithTimestamp() ([]string, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT table_name FROM information_schema.columns
		WHERE column_name = 'updated_at'
		AND table_schema = 'public'
		ORDER BY table_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

// getTableColumns returns the column names for a table.
func (s *Service) getTableColumns(table string) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT column_name FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position
	`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

func (s *Service) insertRecord(backupType, filePath string) (*Record, error) {
	var r Record
	err := s.db.QueryRow(
		`INSERT INTO backup_records (backup_type, file_path, status)
		 VALUES ($1, $2, 'running')
		 RETURNING id, backup_type, file_path, file_size, status, started_at, completed_at, error_message, created_at`,
		backupType, filePath,
	).Scan(&r.ID, &r.BackupType, &r.FilePath, &r.FileSize,
		&r.Status, &r.StartedAt, &r.CompletedAt, &r.ErrorMessage, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Service) updateRecord(id int64, status, errMsg string) {
	s.db.Exec(
		`UPDATE backup_records SET status=$1, error_message=$2, completed_at=$3 WHERE id=$4`,
		status, errMsg, time.Now(), id,
	)
}

func (s *Service) getRecord(id int64) (*Record, error) {
	var r Record
	err := s.db.QueryRow(
		`SELECT id, backup_type, file_path, file_size, status, started_at, completed_at, error_message, created_at
		 FROM backup_records WHERE id = $1`, id,
	).Scan(&r.ID, &r.BackupType, &r.FilePath, &r.FileSize,
		&r.Status, &r.StartedAt, &r.CompletedAt, &r.ErrorMessage, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Service) getLastCompletedBackupTime() (time.Time, error) {
	var t sql.NullTime
	err := s.db.QueryRow(
		`SELECT MAX(started_at) FROM backup_records WHERE status = 'completed'`,
	).Scan(&t)
	if err != nil {
		return time.Time{}, err
	}
	if !t.Valid {
		return time.Now().Add(-24 * time.Hour), nil
	}
	return t.Time, nil
}

// formatValue formats a value for SQL INSERT output.
func formatValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case []byte:
		return fmt.Sprintf("'%s'", escapeSQL(string(val)))
	case string:
		return fmt.Sprintf("'%s'", escapeSQL(val))
	case time.Time:
		return fmt.Sprintf("'%s'", val.Format("2006-01-02 15:04:05"))
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%g", val)
	default:
		return fmt.Sprintf("'%s'", escapeSQL(fmt.Sprintf("%v", val)))
	}
}

func escapeSQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// Tables for incremental backup — tables we care about tracking changes for.
var incrementalTables = []string{
	"merchants", "users", "merchant_roles", "roles", "platform_roles",
	"products", "product_skus", "categories",
	"suppliers", "purchases", "purchase_items",
	"members", "pets", "member_tags", "member_levels",
	"employees", "appointments", "service_records",
	"orders", "order_items",
	"coupons", "promotions", "service_cards",
	"inventory", "inventory_logs",
	"services", "service_packages",
	"reviews", "complaints",
	"contracts", "announcements",
	"receipt_templates", "commission_rules",
	"balance_logs", "points_logs",
}

// JoinTables returns the list of tracked tables for incremental backup.
func JoinTables() []string {
	return incrementalTables
}

// GetAvailableTables returns tables that have updated_at and are in the tracked set.
func (s *Service) GetAvailableTables() ([]string, error) {
	allTables, err := s.getTablesWithTimestamp()
	if err != nil {
		return nil, err
	}

	tracked := make(map[string]bool, len(incrementalTables))
	for _, t := range incrementalTables {
		tracked[t] = true
	}

	var result []string
	for _, t := range allTables {
		if tracked[t] {
			result = append(result, t)
		}
	}
	sort.Strings(result)
	return result, nil
}
