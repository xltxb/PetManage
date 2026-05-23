package shift

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// ShiftRecord represents a shift handover reconciliation record.
type ShiftRecord struct {
	ID                  int64           `json:"id"`
	MerchantID          int64           `json:"merchant_id"`
	EmployeeID          int64           `json:"employee_id"`
	EmployeeName        string          `json:"employee_name"`
	ShiftDate           string          `json:"shift_date"`
	ExpectedTotalCents  int             `json:"expected_total_cents"`
	ExpectedBreakdown   json.RawMessage `json:"expected_breakdown"`
	ActualTotalCents    int             `json:"actual_total_cents"`
	ActualBreakdown     json.RawMessage `json:"actual_breakdown"`
	DifferenceCents     int             `json:"difference_cents"`
	DifferenceBreakdown json.RawMessage `json:"difference_breakdown"`
	OrderCount          int             `json:"order_count"`
	Status              string          `json:"status"`
	ConfirmedBy         *int64          `json:"confirmed_by"`
	ConfirmedByName     string          `json:"confirmed_by_name,omitempty"`
	ConfirmedAt         *time.Time      `json:"confirmed_at"`
	Notes               string          `json:"notes"`
	CreatedAt           time.Time       `json:"created_at"`
}

// BreakDown represents payment method breakdown (cents).
type BreakDown map[string]int

// ListParams holds filters for listing shift records.
type ListParams struct {
	StartDate string
	EndDate   string
	Status    string
	Page      int
	PageSize  int
}

// ListResult wraps the shift records list with pagination.
type ListResult struct {
	Records  []ShiftRecord `json:"records"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

// Service provides shift reconciliation operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new shift Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

const shiftColumns = `sr.id, sr.merchant_id, sr.employee_id, e.name AS employee_name,
	sr.shift_date, sr.expected_total_cents, sr.expected_breakdown,
	sr.actual_total_cents, sr.actual_breakdown,
	sr.difference_cents, sr.difference_breakdown,
	sr.order_count, sr.status, sr.confirmed_by,
	COALESCE(ce.name, '') AS confirmed_by_name,
	sr.confirmed_at, sr.notes, sr.created_at`

const shiftScan = `&sr.ID, &sr.MerchantID, &sr.EmployeeID, &sr.EmployeeName,
	&sr.ShiftDate, &sr.ExpectedTotalCents, &sr.ExpectedBreakdown,
	&sr.ActualTotalCents, &sr.ActualBreakdown,
	&sr.DifferenceCents, &sr.DifferenceBreakdown,
	&sr.OrderCount, &sr.Status, &sr.ConfirmedBy,
	&sr.ConfirmedByName,
	&sr.ConfirmedAt, &sr.Notes, &sr.CreatedAt`

// scanShift scans a shift record row.
func scanShift(sr *ShiftRecord, scanner func(dest ...interface{}) error) error {
	var confirmedBy sql.NullInt64
	var confirmedAt sql.NullTime
	if err := scanner(
		&sr.ID, &sr.MerchantID, &sr.EmployeeID, &sr.EmployeeName,
		&sr.ShiftDate, &sr.ExpectedTotalCents, &sr.ExpectedBreakdown,
		&sr.ActualTotalCents, &sr.ActualBreakdown,
		&sr.DifferenceCents, &sr.DifferenceBreakdown,
		&sr.OrderCount, &sr.Status, &confirmedBy,
		&sr.ConfirmedByName,
		&confirmedAt, &sr.Notes, &sr.CreatedAt,
	); err != nil {
		return err
	}
	if confirmedBy.Valid {
		sr.ConfirmedBy = &confirmedBy.Int64
	}
	if confirmedAt.Valid {
		sr.ConfirmedAt = &confirmedAt.Time
	}
	return nil
}

func queryShiftRow(ctx context.Context, db *sql.DB, query string, args ...interface{}) (*ShiftRecord, error) {
	var sr ShiftRecord
	row := db.QueryRowContext(ctx, query, args...)
	if err := scanShift(&sr, row.Scan); err != nil {
		return nil, err
	}
	return &sr, nil
}

// CreateShiftReport generates a shift reconciliation report for the cashier.
func (s *Service) CreateShiftReport(ctx context.Context, merchantID, employeeID int64) (*ShiftRecord, error) {
	today := time.Now().Format("2006-01-02")

	// Check if there's already a shift for this employee today.
	var existingCount int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM shift_records
		 WHERE merchant_id = $1 AND employee_id = $2 AND shift_date = $3 AND deleted_at IS NULL`,
		merchantID, employeeID, today,
	).Scan(&existingCount)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to check existing shift", err)
	}
	if existingCount > 0 {
		return nil, apperrors.NewConflictError("shift report already exists for today")
	}

	// Verify employee belongs to the merchant and is active.
	var empStatus string
	err = s.db.QueryRowContext(ctx,
		`SELECT status FROM employees WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		employeeID, merchantID,
	).Scan(&empStatus)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("employee not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query employee", err)
	}
	if empStatus != "active" {
		return nil, apperrors.NewValidationError("employee is not active")
	}

	// Aggregate today's payments grouped by method (actual collections).
	type methodTotal struct {
		Method string
		Total  int
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT p.method, SUM(p.amount_cents)::int
		 FROM payments p
		 JOIN orders o ON o.id = p.order_id
		 WHERE o.merchant_id = $1
		   AND o.created_at::date = $2
		   AND o.status = 'completed'
		 GROUP BY p.method`,
		merchantID, today,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to aggregate payments", err)
	}
	defer rows.Close()

	actualBreakdown := make(BreakDown)
	actualTotal := 0
	for rows.Next() {
		var mt methodTotal
		if err := rows.Scan(&mt.Method, &mt.Total); err != nil {
			return nil, apperrors.NewInternalError("failed to scan payment row", err)
		}
		actualBreakdown[mt.Method] = mt.Total
		actualTotal += mt.Total
	}

	// Expected = sum of order total_cents for completed orders today.
	var orderCount int
	var expectedTotal int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(total_cents), 0)::int
		 FROM orders
		 WHERE merchant_id = $1
		   AND created_at::date = $2
		   AND status = 'completed'`,
		merchantID, today,
	).Scan(&orderCount, &expectedTotal)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to aggregate orders", err)
	}

	// Payment breakdown by method for expected (same source - payments).
	// Expected breakdown mirrors actual since both come from payments.
	expectedBreakdown := make(BreakDown)
	for k, v := range actualBreakdown {
		expectedBreakdown[k] = v
	}

	// Difference = actual - expected (positive = over, negative = short).
	diffCents := actualTotal - expectedTotal
	diffBreakdown := make(BreakDown)
	diffBreakdown["total"] = diffCents

	expectedJSON, _ := json.Marshal(expectedBreakdown)
	actualJSON, _ := json.Marshal(actualBreakdown)
	diffJSON, _ := json.Marshal(diffBreakdown)

	// Begin transaction: insert shift record + lock employee.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	var shiftID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO shift_records
		 (merchant_id, employee_id, shift_date, expected_total_cents, expected_breakdown,
		  actual_total_cents, actual_breakdown, difference_cents, difference_breakdown,
		  order_count, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'pending')
		 RETURNING id`,
		merchantID, employeeID, today, expectedTotal, expectedJSON,
		actualTotal, actualJSON, diffCents, diffJSON, orderCount,
	).Scan(&shiftID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create shift record", err)
	}

	// Lock the cashier's employee — must re-login after shift.
	_, err = tx.ExecContext(ctx,
		`UPDATE employees SET shift_locked = true, updated_at = NOW()
		 WHERE id = $1 AND merchant_id = $2`,
		employeeID, merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to lock employee", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit transaction", err)
	}

	// Query the created record with joins.
	return s.GetShiftReport(ctx, merchantID, shiftID)
}

// GetShiftReport returns a single shift record by ID.
func (s *Service) GetShiftReport(ctx context.Context, merchantID, id int64) (*ShiftRecord, error) {
	sr, err := queryShiftRow(ctx, s.db,
		`SELECT `+shiftColumns+`
		 FROM shift_records sr
		 JOIN employees e ON e.id = sr.employee_id
		 LEFT JOIN employees ce ON ce.id = sr.confirmed_by
		 WHERE sr.id = $1 AND sr.merchant_id = $2 AND sr.deleted_at IS NULL`,
		id, merchantID,
	)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("shift record not found")
	}
	return sr, err
}

// ListShiftReports returns paginated shift records for a merchant.
func (s *Service) ListShiftReports(ctx context.Context, merchantID int64, params ListParams) (*ListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}
	offset := (params.Page - 1) * params.PageSize

	args := []interface{}{merchantID}
	argIdx := 2

	where := `WHERE sr.merchant_id = $1 AND sr.deleted_at IS NULL`
	if params.Status != "" {
		where += ` AND sr.status = $` + strconv.Itoa(argIdx)
		args = append(args, params.Status)
		argIdx++
	}
	if params.StartDate != "" {
		where += ` AND sr.shift_date >= $` + strconv.Itoa(argIdx)
		args = append(args, params.StartDate)
		argIdx++
	}
	if params.EndDate != "" {
		where += ` AND sr.shift_date <= $` + strconv.Itoa(argIdx)
		args = append(args, params.EndDate)
		argIdx++
	}

	var total int
	countQuery := `SELECT COUNT(*) FROM shift_records sr ` + where
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, apperrors.NewInternalError("failed to count shift records", err)
	}

	query := `SELECT ` + shiftColumns + `
		FROM shift_records sr
		JOIN employees e ON e.id = sr.employee_id
		LEFT JOIN employees ce ON ce.id = sr.confirmed_by
		` + where + `
		ORDER BY sr.created_at DESC
		LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, params.PageSize, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query shift records", err)
	}
	defer rows.Close()

	var records []ShiftRecord
	for rows.Next() {
		var sr ShiftRecord
		if err := scanShift(&sr, rows.Scan); err != nil {
			return nil, apperrors.NewInternalError("failed to scan shift record", err)
		}
		records = append(records, sr)
	}

	return &ListResult{
		Records:  records,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, nil
}

// ConfirmShiftReport allows a manager to confirm a pending shift report.
func (s *Service) ConfirmShiftReport(ctx context.Context, merchantID, shiftID, confirmerID int64) (*ShiftRecord, error) {
	// Verify the shift exists and is pending.
	var currentStatus string
	err := s.db.QueryRowContext(ctx,
		`SELECT status FROM shift_records
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		shiftID, merchantID,
	).Scan(&currentStatus)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("shift record not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query shift record", err)
	}
	if currentStatus != "pending" {
		return nil, apperrors.NewValidationError("shift record is not in pending status")
	}

	// Verify confirmer is an active employee of the merchant.
	var confirmerStatus string
	err = s.db.QueryRowContext(ctx,
		`SELECT status FROM employees WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		confirmerID, merchantID,
	).Scan(&confirmerStatus)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("confirmer employee not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query confirmer", err)
	}
	if confirmerStatus != "active" {
		return nil, apperrors.NewValidationError("confirmer employee is not active")
	}

	now := time.Now()
	_, err = s.db.ExecContext(ctx,
		`UPDATE shift_records
		 SET status = 'confirmed', confirmed_by = $3, confirmed_at = $4, updated_at = NOW()
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		shiftID, merchantID, confirmerID, now,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to confirm shift record", err)
	}

	return s.GetShiftReport(ctx, merchantID, shiftID)
}

// GetTodayShiftStatus checks if there's a shift record for today for the given employee.
// Returns nil if no shift exists (not an error).
func (s *Service) GetTodayShiftStatus(ctx context.Context, merchantID, employeeID int64) (*ShiftRecord, error) {
	today := time.Now().Format("2006-01-02")
	sr, err := queryShiftRow(ctx, s.db,
		`SELECT `+shiftColumns+`
		 FROM shift_records sr
		 JOIN employees e ON e.id = sr.employee_id
		 LEFT JOIN employees ce ON ce.id = sr.confirmed_by
		 WHERE sr.merchant_id = $1 AND sr.employee_id = $2
		   AND sr.shift_date = $3 AND sr.deleted_at IS NULL
		 ORDER BY sr.id DESC LIMIT 1`,
		merchantID, employeeID, today,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return sr, err
}

// UnlockEmployee clears the shift lock on an employee (called after re-login).
func (s *Service) UnlockEmployee(ctx context.Context, merchantID, employeeID int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE employees SET shift_locked = false, updated_at = NOW()
		 WHERE id = $1 AND merchant_id = $2 AND shift_locked = true`,
		employeeID, merchantID,
	)
	return err
}

// IsEmployeeShiftLocked checks if an employee is currently shift-locked.
func (s *Service) IsEmployeeShiftLocked(ctx context.Context, merchantID, employeeID int64) (bool, error) {
	var locked bool
	err := s.db.QueryRowContext(ctx,
		`SELECT shift_locked FROM employees WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		employeeID, merchantID,
	).Scan(&locked)
	if err != nil {
		return false, err
	}
	return locked, nil
}
