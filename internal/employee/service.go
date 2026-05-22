package employee

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Employee represents an employee record.
type Employee struct {
	ID         int64      `json:"id"`
	MerchantID int64      `json:"merchant_id"`
	Name       string     `json:"name"`
	EmployeeNo string     `json:"employee_no"`
	Position   string     `json:"position"`
	Phone      string     `json:"phone"`
	Email      string     `json:"email"`
	HireDate   *string    `json:"hire_date"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}

// CreateEmployeeRequest is the request body for creating an employee.
type CreateEmployeeRequest struct {
	Name       string `json:"name"`
	EmployeeNo string `json:"employee_no"`
	Position   string `json:"position"`
	Phone      string `json:"phone"`
	Email      string `json:"email"`
	HireDate   string `json:"hire_date"`
}

// UpdateEmployeeRequest is the request body for partial update.
type UpdateEmployeeRequest struct {
	Name       *string `json:"name"`
	EmployeeNo *string `json:"employee_no"`
	Position   *string `json:"position"`
	Phone      *string `json:"phone"`
	Email      *string `json:"email"`
	HireDate   *string `json:"hire_date"`
}

// ListParams holds optional filters and pagination for listing employees.
type ListParams struct {
	Status   string
	Position string
	Keyword  string
	Page     int
	PageSize int
}

// ListResult wraps the employees list with pagination info.
type ListResult struct {
	Employees []Employee `json:"employees"`
	Total     int        `json:"total"`
	Page      int        `json:"page"`
	PageSize  int        `json:"page_size"`
}

// Service provides employee management operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new employee Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

const employeeColumns = `id, merchant_id, name, employee_no, position, phone, email, hire_date, status, created_at, updated_at`

func scanEmployeeRow(row *sql.Row) (*Employee, error) {
	e := &Employee{}
	var hireDate sql.NullString
	err := row.Scan(
		&e.ID, &e.MerchantID, &e.Name, &e.EmployeeNo, &e.Position,
		&e.Phone, &e.Email, &hireDate, &e.Status,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if hireDate.Valid {
		e.HireDate = &hireDate.String
	}
	return e, err
}

func scanEmployeeRows(rows *sql.Rows) (*Employee, error) {
	e := &Employee{}
	var hireDate sql.NullString
	err := rows.Scan(
		&e.ID, &e.MerchantID, &e.Name, &e.EmployeeNo, &e.Position,
		&e.Phone, &e.Email, &hireDate, &e.Status,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if hireDate.Valid {
		e.HireDate = &hireDate.String
	}
	return e, err
}

// Create adds a new employee for a merchant.
func (s *Service) Create(ctx context.Context, merchantID int64, req CreateEmployeeRequest) (*Employee, error) {
	name := strings.TrimSpace(req.Name)
	employeeNo := strings.TrimSpace(req.EmployeeNo)
	position := strings.TrimSpace(req.Position)

	var missing []string
	if name == "" {
		missing = append(missing, "name")
	}
	if employeeNo == "" {
		missing = append(missing, "employee_no")
	}
	if position == "" {
		missing = append(missing, "position")
	}
	if len(missing) > 0 {
		return nil, apperrors.NewValidationError("missing required fields: " + strings.Join(missing, ", "))
	}

	// Check employee_no uniqueness within merchant.
	var existingID int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM employees WHERE merchant_id = $1 AND employee_no = $2 AND deleted_at IS NULL`,
		merchantID, employeeNo,
	).Scan(&existingID)
	if err == nil {
		return nil, apperrors.NewConflictError("employee number already exists: " + employeeNo)
	}
	if err != sql.ErrNoRows {
		return nil, apperrors.NewInternalError("failed to check employee_no uniqueness", err)
	}

	var hireDate interface{}
	if strings.TrimSpace(req.HireDate) != "" {
		hireDate = req.HireDate
	}

	e := &Employee{}
	var hd sql.NullString
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO employees (merchant_id, name, employee_no, position, phone, email, hire_date)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING `+employeeColumns,
		merchantID, name, employeeNo, position, req.Phone, req.Email, hireDate,
	).Scan(
		&e.ID, &e.MerchantID, &e.Name, &e.EmployeeNo, &e.Position,
		&e.Phone, &e.Email, &hd, &e.Status,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create employee", err)
	}
	if hd.Valid {
		e.HireDate = &hd.String
	}

	return e, nil
}

// List returns a filtered and paginated list of employees for a merchant.
func (s *Service) List(ctx context.Context, merchantID int64, params ListParams) (*ListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}

	args := []interface{}{merchantID}
	argIdx := 2

	where := "WHERE merchant_id = $1 AND deleted_at IS NULL"

	if params.Status != "" {
		where += " AND status = $" + itoa(argIdx)
		args = append(args, params.Status)
		argIdx++
	}
	if params.Position != "" {
		where += " AND position = $" + itoa(argIdx)
		args = append(args, params.Position)
		argIdx++
	}
	if params.Keyword != "" {
		where += " AND (name ILIKE $" + itoa(argIdx) + " OR employee_no ILIKE $" + itoa(argIdx) +
			" OR phone ILIKE $" + itoa(argIdx) + ")"
		kw := "%" + params.Keyword + "%"
		args = append(args, kw)
		argIdx++
	}

	// Count total.
	var total int
	countQuery := "SELECT COUNT(*) FROM employees " + where
	ctArgs := make([]interface{}, len(args))
	copy(ctArgs, args)
	if err := s.db.QueryRowContext(ctx, countQuery, ctArgs...).Scan(&total); err != nil {
		return nil, apperrors.NewInternalError("failed to count employees", err)
	}

	// Fetch page.
	offset := (params.Page - 1) * params.PageSize
	query := "SELECT " + employeeColumns + " FROM employees " + where +
		" ORDER BY created_at DESC LIMIT $" + itoa(argIdx) + " OFFSET $" + itoa(argIdx+1)
	args = append(args, params.PageSize, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query employees", err)
	}
	defer rows.Close()

	var employees []Employee
	for rows.Next() {
		e, err := scanEmployeeRows(rows)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to scan employee row", err)
		}
		employees = append(employees, *e)
	}
	if employees == nil {
		employees = []Employee{}
	}

	return &ListResult{
		Employees: employees,
		Total:     total,
		Page:      params.Page,
		PageSize:  params.PageSize,
	}, nil
}

// GetByID returns a single employee with merchant ownership verification.
func (s *Service) GetByID(ctx context.Context, merchantID, employeeID int64) (*Employee, error) {
	e, err := scanEmployeeRow(s.db.QueryRowContext(ctx,
		`SELECT `+employeeColumns+` FROM employees
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		employeeID, merchantID,
	))
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("employee not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get employee", err)
	}
	return e, nil
}

// Update partially updates an employee's fields.
func (s *Service) Update(ctx context.Context, merchantID, employeeID int64, req UpdateEmployeeRequest) (*Employee, error) {
	// First check the employee exists and belongs to this merchant.
	_, err := s.GetByID(ctx, merchantID, employeeID)
	if err != nil {
		return nil, err
	}

	// Build dynamic update.
	var sets []string
	args := []interface{}{employeeID, merchantID}
	argIdx := 3

	if req.Name != nil {
		sets = append(sets, "name = $"+itoa(argIdx))
		args = append(args, *req.Name)
		argIdx++
	}
	if req.EmployeeNo != nil {
		no := strings.TrimSpace(*req.EmployeeNo)
		if no == "" {
			return nil, apperrors.NewValidationError("employee_no cannot be empty")
		}
		// Check uniqueness excluding current employee.
		var existingID int64
		err := s.db.QueryRowContext(ctx,
			`SELECT id FROM employees WHERE merchant_id = $1 AND employee_no = $2 AND deleted_at IS NULL AND id != $3`,
			merchantID, no, employeeID,
		).Scan(&existingID)
		if err == nil {
			return nil, apperrors.NewConflictError("employee number already exists: " + no)
		}
		if err != sql.ErrNoRows {
			return nil, apperrors.NewInternalError("failed to check employee_no uniqueness", err)
		}
		sets = append(sets, "employee_no = $"+itoa(argIdx))
		args = append(args, no)
		argIdx++
	}
	if req.Position != nil {
		sets = append(sets, "position = $"+itoa(argIdx))
		args = append(args, *req.Position)
		argIdx++
	}
	if req.Phone != nil {
		sets = append(sets, "phone = $"+itoa(argIdx))
		args = append(args, *req.Phone)
		argIdx++
	}
	if req.Email != nil {
		sets = append(sets, "email = $"+itoa(argIdx))
		args = append(args, *req.Email)
		argIdx++
	}
	if req.HireDate != nil {
		sets = append(sets, "hire_date = $"+itoa(argIdx))
		args = append(args, *req.HireDate)
		argIdx++
	}

	if len(sets) == 0 {
		return s.GetByID(ctx, merchantID, employeeID)
	}

	sets = append(sets, "updated_at = NOW()")

	query := "UPDATE employees SET " + strings.Join(sets, ", ") +
		" WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL RETURNING " + employeeColumns

	e := &Employee{}
	var hd sql.NullString
	err = s.db.QueryRowContext(ctx, query, args...).Scan(
		&e.ID, &e.MerchantID, &e.Name, &e.EmployeeNo, &e.Position,
		&e.Phone, &e.Email, &hd, &e.Status,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update employee", err)
	}
	if hd.Valid {
		e.HireDate = &hd.String
	}

	return e, nil
}

// Resign marks an employee as resigned (inactive) and disables their platform account if one exists.
func (s *Service) Resign(ctx context.Context, merchantID, employeeID int64) (*Employee, error) {
	// Verify employee exists.
	_, err := s.GetByID(ctx, merchantID, employeeID)
	if err != nil {
		return nil, err
	}

	// Update employee status to inactive.
	e := &Employee{}
	var hd sql.NullString
	err = s.db.QueryRowContext(ctx,
		`UPDATE employees SET status = 'inactive', updated_at = NOW()
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL
		 RETURNING `+employeeColumns,
		employeeID, merchantID,
	).Scan(
		&e.ID, &e.MerchantID, &e.Name, &e.EmployeeNo, &e.Position,
		&e.Phone, &e.Email, &hd, &e.Status,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to resign employee", err)
	}
	if hd.Valid {
		e.HireDate = &hd.String
	}

	// Disable associated platform_users account if exists.
	// Match by username pattern: e_{merchantID}_{employeeNo}
	username := "e_" + itoa(int(merchantID)) + "_" + e.EmployeeNo
	_, _ = s.db.ExecContext(ctx,
		`UPDATE platform_users SET status = 'disabled', updated_at = NOW()
		 WHERE username = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		username, merchantID,
	)

	return e, nil
}

// ToggleStatus toggles an employee between active and inactive.
func (s *Service) ToggleStatus(ctx context.Context, merchantID, employeeID int64) (*Employee, error) {
	e, err := s.GetByID(ctx, merchantID, employeeID)
	if err != nil {
		return nil, err
	}

	newStatus := "inactive"
	if e.Status == "inactive" {
		newStatus = "active"
	}

	var hd sql.NullString
	err = s.db.QueryRowContext(ctx,
		`UPDATE employees SET status = $1, updated_at = NOW()
		 WHERE id = $2 AND merchant_id = $3 AND deleted_at IS NULL
		 RETURNING `+employeeColumns,
		newStatus, employeeID, merchantID,
	).Scan(
		&e.ID, &e.MerchantID, &e.Name, &e.EmployeeNo, &e.Position,
		&e.Phone, &e.Email, &hd, &e.Status,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to toggle employee status", err)
	}
	if hd.Valid {
		e.HireDate = &hd.String
	}

	return e, nil
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
