package fixedexpense

import (
	"context"
	"database/sql"
	"strings"
	"strconv"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// FixedExpense represents a fixed expense entry.
type FixedExpense struct {
	ID          int64  `json:"id"`
	MerchantID  int64  `json:"merchant_id"`
	Name        string `json:"name"`
	AmountCents int    `json:"amount_cents"`
	Category    string `json:"category"`
	Notes       string `json:"notes"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// CreateRequest is the request body for creating a fixed expense.
type CreateRequest struct {
	Name        string `json:"name"`
	AmountCents int    `json:"amount_cents"`
	Category    string `json:"category"`
	Notes       string `json:"notes"`
}

// UpdateRequest is the request body for updating a fixed expense.
type UpdateRequest struct {
	Name        *string `json:"name"`
	AmountCents *int    `json:"amount_cents"`
	Category    *string `json:"category"`
	Notes       *string `json:"notes"`
}

// ListParams holds filters for listing fixed expenses.
type ListParams struct {
	Category string
	Page     int
	PageSize int
}

// ListResult wraps paginated fixed expense results.
type ListResult struct {
	Items    []FixedExpense `json:"items"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

// Service handles fixed expense management.
type Service struct {
	db *sql.DB
}

// NewService creates a new fixed expense Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

const expColumns = `id, merchant_id, name, amount_cents, category, notes, created_at, updated_at`

func scanExp(row *sql.Row) (*FixedExpense, error) {
	e := &FixedExpense{}
	var createdAt, updatedAt time.Time
	err := row.Scan(&e.ID, &e.MerchantID, &e.Name, &e.AmountCents, &e.Category, &e.Notes, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	e.CreatedAt = createdAt.Format(time.RFC3339)
	e.UpdatedAt = updatedAt.Format(time.RFC3339)
	return e, nil
}

func scanExpRows(rows *sql.Rows) (*FixedExpense, error) {
	e := &FixedExpense{}
	var createdAt, updatedAt time.Time
	err := rows.Scan(&e.ID, &e.MerchantID, &e.Name, &e.AmountCents, &e.Category, &e.Notes, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	e.CreatedAt = createdAt.Format(time.RFC3339)
	e.UpdatedAt = updatedAt.Format(time.RFC3339)
	return e, nil
}

// Create creates a new fixed expense for a merchant.
func (s *Service) Create(ctx context.Context, merchantID int64, req CreateRequest) (*FixedExpense, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperrors.NewValidationError("expense name is required")
	}
	if req.AmountCents < 0 {
		return nil, apperrors.NewValidationError("amount must be non-negative")
	}
	cat := strings.TrimSpace(req.Category)
	if cat == "" {
		cat = "other"
	}
	if cat != "rent" && cat != "utilities" && cat != "salary" && cat != "other" {
		return nil, apperrors.NewValidationError("category must be rent, utilities, salary, or other")
	}

	return scanExp(s.db.QueryRowContext(ctx,
		`INSERT INTO fixed_expenses (merchant_id, name, amount_cents, category, notes)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+expColumns,
		merchantID, req.Name, req.AmountCents, cat, req.Notes,
	))
}

// List returns paginated fixed expenses for a merchant.
func (s *Service) List(ctx context.Context, merchantID int64, params ListParams) (*ListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}

	args := []interface{}{merchantID}
	argIdx := 2
	where := "WHERE merchant_id = $1 AND deleted_at IS NULL"

	if params.Category != "" {
		where += " AND category = $" + strconv.Itoa(argIdx)
		args = append(args, params.Category)
		argIdx++
	}

	var total int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM fixed_expenses `+where, args...).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count expenses", err)
	}

	offset := (params.Page - 1) * params.PageSize
	queryArgs := append(args, params.PageSize, offset)
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+expColumns+` FROM fixed_expenses `+where+
			` ORDER BY created_at DESC LIMIT $`+strconv.Itoa(argIdx)+` OFFSET $`+strconv.Itoa(argIdx+1),
		queryArgs...,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list expenses", err)
	}
	defer rows.Close()

	var items []FixedExpense
	for rows.Next() {
		e, err := scanExpRows(rows)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to scan expense", err)
		}
		items = append(items, *e)
	}
	if items == nil {
		items = []FixedExpense{}
	}

	return &ListResult{
		Items:    items,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, rows.Err()
}

// Update updates a fixed expense.
func (s *Service) Update(ctx context.Context, expenseID, merchantID int64, req UpdateRequest) (*FixedExpense, error) {
	existing, err := s.getByID(ctx, expenseID, merchantID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			return nil, apperrors.NewValidationError("expense name is required")
		}
		existing.Name = *req.Name
	}
	if req.AmountCents != nil {
		if *req.AmountCents < 0 {
			return nil, apperrors.NewValidationError("amount must be non-negative")
		}
		existing.AmountCents = *req.AmountCents
	}
	if req.Category != nil {
		cat := strings.TrimSpace(*req.Category)
		if cat == "" {
			cat = "other"
		}
		if cat != "rent" && cat != "utilities" && cat != "salary" && cat != "other" {
			return nil, apperrors.NewValidationError("category must be rent, utilities, salary, or other")
		}
		existing.Category = cat
	}
	if req.Notes != nil {
		existing.Notes = *req.Notes
	}

	return scanExp(s.db.QueryRowContext(ctx,
		`UPDATE fixed_expenses SET name = $1, amount_cents = $2, category = $3, notes = $4, updated_at = NOW()
		 WHERE id = $5 AND merchant_id = $6 AND deleted_at IS NULL
		 RETURNING `+expColumns,
		existing.Name, existing.AmountCents, existing.Category, existing.Notes, expenseID, merchantID,
	))
}

// Delete soft-deletes a fixed expense.
func (s *Service) Delete(ctx context.Context, expenseID, merchantID int64) error {
	_, err := s.getByID(ctx, expenseID, merchantID)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE fixed_expenses SET deleted_at = NOW() WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		expenseID, merchantID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to delete expense", err)
	}
	return nil
}

func (s *Service) getByID(ctx context.Context, expenseID, merchantID int64) (*FixedExpense, error) {
	e, err := scanExp(s.db.QueryRowContext(ctx,
		`SELECT `+expColumns+` FROM fixed_expenses WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		expenseID, merchantID,
	))
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("fixed expense not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get expense", err)
	}
	return e, nil
}
