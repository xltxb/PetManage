package balance

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// RechargePackage represents a stored value package definition.
type RechargePackage struct {
	ID             int64      `json:"id"`
	MerchantID     int64      `json:"merchant_id"`
	Name           string     `json:"name"`
	PrincipalCents int64      `json:"principal_cents"`
	BonusCents     int64      `json:"bonus_cents"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}

// CreatePackageRequest is the request body for creating a recharge package.
type CreatePackageRequest struct {
	Name           string `json:"name"`
	PrincipalCents int64  `json:"principal_cents"`
	BonusCents     int64  `json:"bonus_cents"`
}

// UpdatePackageRequest is the request body for updating a recharge package.
type UpdatePackageRequest struct {
	Name           *string `json:"name"`
	PrincipalCents *int64  `json:"principal_cents"`
	BonusCents     *int64  `json:"bonus_cents"`
}

// RechargeRequest is the request body for member recharge.
type RechargeRequest struct {
	PackageID     *int64 `json:"package_id"`
	PrincipalCents int64 `json:"principal_cents"`
	BonusCents    int64 `json:"bonus_cents"`
	PaymentMethod string `json:"payment_method"`
	Notes         string `json:"notes"`
}

// RechargeResponse is the response after a successful recharge.
type RechargeResponse struct {
	TransactionID   int64 `json:"transaction_id"`
	PrincipalBefore int64 `json:"principal_before"`
	PrincipalAfter  int64 `json:"principal_after"`
	BonusBefore     int64 `json:"bonus_before"`
	BonusAfter      int64 `json:"bonus_after"`
	BalanceBefore   int64 `json:"balance_before"`
	BalanceAfter    int64 `json:"balance_after"`
}

// BalanceTransaction represents a balance change record.
type BalanceTransaction struct {
	ID              int64     `json:"id"`
	MerchantID      int64     `json:"merchant_id"`
	MemberID        int64     `json:"member_id"`
	Type            string    `json:"type"`
	AmountCents     int64     `json:"amount_cents"`
	PrincipalBefore int64     `json:"principal_before"`
	PrincipalAfter  int64     `json:"principal_after"`
	BonusBefore     int64     `json:"bonus_before"`
	BonusAfter      int64     `json:"bonus_after"`
	ReferenceType   string    `json:"reference_type"`
	ReferenceID     *int64    `json:"reference_id,omitempty"`
	OperatorID      *int64    `json:"operator_id,omitempty"`
	PaymentMethod   string    `json:"payment_method"`
	Notes           string    `json:"notes"`
	CreatedAt       time.Time `json:"created_at"`
}

// TransactionListParams holds optional filters for listing balance transactions.
type TransactionListParams struct {
	Type       string
	StartTime  string
	EndTime    string
	Page       int
	PageSize   int
}

// TransactionListResult wraps the transactions list with pagination info.
type TransactionListResult struct {
	Transactions []BalanceTransaction `json:"transactions"`
	Total        int                  `json:"total"`
	Page         int                  `json:"page"`
	PageSize     int                  `json:"page_size"`
}

// MemberBalance holds the detailed balance breakdown for a member.
type MemberBalance struct {
	TotalBalanceCents     int64 `json:"total_balance_cents"`
	PrincipalBalanceCents int64 `json:"principal_balance_cents"`
	BonusBalanceCents     int64 `json:"bonus_balance_cents"`
}

// Service provides stored value management operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new balance Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

const packageColumns = `id, merchant_id, name, principal_cents, bonus_cents, status, created_at, updated_at`

func scanPackageRow(row *sql.Row) (*RechargePackage, error) {
	p := &RechargePackage{}
	err := row.Scan(&p.ID, &p.MerchantID, &p.Name, &p.PrincipalCents, &p.BonusCents, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func scanPackageRows(rows *sql.Rows) (*RechargePackage, error) {
	p := &RechargePackage{}
	err := rows.Scan(&p.ID, &p.MerchantID, &p.Name, &p.PrincipalCents, &p.BonusCents, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// CreatePackage creates a new recharge package.
func (s *Service) CreatePackage(ctx context.Context, merchantID int64, req CreatePackageRequest) (*RechargePackage, error) {
	if req.Name == "" {
		return nil, apperrors.NewValidationError("package name is required")
	}
	if req.PrincipalCents <= 0 {
		return nil, apperrors.NewValidationError("principal amount must be positive")
	}
	if req.BonusCents < 0 {
		return nil, apperrors.NewValidationError("bonus amount must be non-negative")
	}
	p, err := scanPackageRow(s.db.QueryRowContext(ctx,
		`INSERT INTO recharge_packages (merchant_id, name, principal_cents, bonus_cents, status)
		 VALUES ($1, $2, $3, $4, 'active')
		 RETURNING `+packageColumns,
		merchantID, req.Name, req.PrincipalCents, req.BonusCents,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create recharge package", err)
	}
	return p, nil
}

// ListPackages lists all recharge packages for a merchant.
func (s *Service) ListPackages(ctx context.Context, merchantID int64) ([]RechargePackage, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+packageColumns+` FROM recharge_packages
		 WHERE merchant_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at DESC`,
		merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list recharge packages", err)
	}
	defer rows.Close()

	packages := make([]RechargePackage, 0)
	for rows.Next() {
		p, err := scanPackageRows(rows)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to scan package", err)
		}
		packages = append(packages, *p)
	}
	return packages, nil
}

// GetPackage retrieves a single recharge package by ID.
func (s *Service) GetPackage(ctx context.Context, packageID, merchantID int64) (*RechargePackage, error) {
	p, err := scanPackageRow(s.db.QueryRowContext(ctx,
		`SELECT `+packageColumns+` FROM recharge_packages
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		packageID, merchantID,
	))
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("recharge package not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get recharge package", err)
	}
	return p, nil
}

// UpdatePackage updates a recharge package's details.
func (s *Service) UpdatePackage(ctx context.Context, packageID, merchantID int64, req UpdatePackageRequest) (*RechargePackage, error) {
	existing, err := s.GetPackage(ctx, packageID, merchantID)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		if *req.Name == "" {
			return nil, apperrors.NewValidationError("package name is required")
		}
		existing.Name = *req.Name
	}
	if req.PrincipalCents != nil {
		if *req.PrincipalCents <= 0 {
			return nil, apperrors.NewValidationError("principal amount must be positive")
		}
		existing.PrincipalCents = *req.PrincipalCents
	}
	if req.BonusCents != nil {
		if *req.BonusCents < 0 {
			return nil, apperrors.NewValidationError("bonus amount must be non-negative")
		}
		existing.BonusCents = *req.BonusCents
	}

	p, err := scanPackageRow(s.db.QueryRowContext(ctx,
		`UPDATE recharge_packages SET name=$1, principal_cents=$2, bonus_cents=$3, updated_at=NOW()
		 WHERE id=$4 AND merchant_id=$5 AND deleted_at IS NULL
		 RETURNING `+packageColumns,
		existing.Name, existing.PrincipalCents, existing.BonusCents, packageID, merchantID,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update recharge package", err)
	}
	return p, nil
}

// DeletePackage soft-deletes a recharge package.
func (s *Service) DeletePackage(ctx context.Context, packageID, merchantID int64) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE recharge_packages SET deleted_at = NOW() WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		packageID, merchantID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to delete recharge package", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return apperrors.NewNotFoundError("recharge package not found")
	}
	return nil
}

// TogglePackage toggles a recharge package's status between active/inactive.
func (s *Service) TogglePackage(ctx context.Context, packageID, merchantID int64) (*RechargePackage, error) {
	existing, err := s.GetPackage(ctx, packageID, merchantID)
	if err != nil {
		return nil, err
	}
	newStatus := "inactive"
	if existing.Status == "inactive" {
		newStatus = "active"
	}
	p, err := scanPackageRow(s.db.QueryRowContext(ctx,
		`UPDATE recharge_packages SET status=$1, updated_at=NOW()
		 WHERE id=$2 AND merchant_id=$3 AND deleted_at IS NULL
		 RETURNING `+packageColumns,
		newStatus, packageID, merchantID,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to toggle package status", err)
	}
	return p, nil
}

// Recharge tops up a member's balance using a package or manual amount.
func (s *Service) Recharge(ctx context.Context, merchantID, memberID, operatorID int64, req RechargeRequest) (*RechargeResponse, error) {
	var principalCents, bonusCents int64
	var referenceType string
	var packageID *int64

	if req.PackageID != nil && *req.PackageID > 0 {
		pkg, err := s.GetPackage(ctx, *req.PackageID, merchantID)
		if err != nil {
			return nil, err
		}
		if pkg.Status != "active" {
			return nil, apperrors.NewValidationError("recharge package is not active")
		}
		principalCents = pkg.PrincipalCents
		bonusCents = pkg.BonusCents
		referenceType = "package"
		packageID = &pkg.ID
	} else {
		if req.PrincipalCents <= 0 {
			return nil, apperrors.NewValidationError("principal amount must be positive")
		}
		principalCents = req.PrincipalCents
		bonusCents = req.BonusCents
		referenceType = "manual"
	}

	paymentMethod := req.PaymentMethod
	if paymentMethod == "" {
		paymentMethod = "cash"
	}
	notes := req.Notes

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	// Lock member row and get current balances.
	var principalBefore, bonusBefore int64
	err = tx.QueryRowContext(ctx,
		`SELECT COALESCE(principal_balance_cents, 0), COALESCE(bonus_balance_cents, 0)
		 FROM members WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL
		 FOR UPDATE`,
		memberID, merchantID,
	).Scan(&principalBefore, &bonusBefore)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("member not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to lock member", err)
	}

	principalAfter := principalBefore + principalCents
	bonusAfter := bonusBefore + bonusCents
	totalAfter := principalAfter + bonusAfter

	// Update member balances.
	_, err = tx.ExecContext(ctx,
		`UPDATE members SET
		 principal_balance_cents = $1, bonus_balance_cents = $2,
		 balance_cents = $3, updated_at = NOW()
		 WHERE id = $4`,
		principalAfter, bonusAfter, totalAfter, memberID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update member balance", err)
	}

	// Insert balance transaction record.
	var txID int64
	totalAmount := principalCents + bonusCents
	rID := sql.NullInt64{Valid: packageID != nil}
	if packageID != nil {
		rID.Int64 = *packageID
	}
	opID := sql.NullInt64{Int64: operatorID, Valid: operatorID > 0}

	err = tx.QueryRowContext(ctx,
		`INSERT INTO balance_transactions
		 (merchant_id, member_id, type, amount_cents,
		  principal_before, principal_after, bonus_before, bonus_after,
		  reference_type, reference_id, operator_id, payment_method, notes)
		 VALUES ($1, $2, 'recharge', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 RETURNING id`,
		merchantID, memberID, totalAmount,
		principalBefore, principalAfter, bonusBefore, bonusAfter,
		referenceType, rID, opID, paymentMethod, notes,
	).Scan(&txID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to record balance transaction", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit recharge", err)
	}

	return &RechargeResponse{
		TransactionID:   txID,
		PrincipalBefore: principalBefore,
		PrincipalAfter:  principalAfter,
		BonusBefore:     bonusBefore,
		BonusAfter:      bonusAfter,
		BalanceBefore:   principalBefore + bonusBefore,
		BalanceAfter:    totalAfter,
	}, nil
}

// GetMemberBalance returns the detailed balance breakdown for a member.
func (s *Service) GetMemberBalance(ctx context.Context, memberID, merchantID int64) (*MemberBalance, error) {
	var principal, bonus int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(principal_balance_cents, 0), COALESCE(bonus_balance_cents, 0)
		 FROM members WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		memberID, merchantID,
	).Scan(&principal, &bonus)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("member not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get member balance", err)
	}
	return &MemberBalance{
		TotalBalanceCents:     principal + bonus,
		PrincipalBalanceCents: principal,
		BonusBalanceCents:     bonus,
	}, nil
}

const txColumns = `id, merchant_id, member_id, type, amount_cents,
	principal_before, principal_after, bonus_before, bonus_after,
	reference_type, reference_id, operator_id, payment_method, notes, created_at`

func scanTransactionRows(rows *sql.Rows) (*BalanceTransaction, error) {
	t := &BalanceTransaction{}
	var refID, opID sql.NullInt64
	err := rows.Scan(
		&t.ID, &t.MerchantID, &t.MemberID, &t.Type, &t.AmountCents,
		&t.PrincipalBefore, &t.PrincipalAfter, &t.BonusBefore, &t.BonusAfter,
		&t.ReferenceType, &refID, &opID, &t.PaymentMethod, &t.Notes, &t.CreatedAt,
	)
	if refID.Valid {
		t.ReferenceID = &refID.Int64
	}
	if opID.Valid {
		t.OperatorID = &opID.Int64
	}
	return t, err
}

// ListTransactions lists balance transactions for a member with optional filters.
func (s *Service) ListTransactions(ctx context.Context, merchantID, memberID int64, params TransactionListParams) (*TransactionListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}

	args := []interface{}{merchantID, memberID}
	whereClause := `merchant_id = $1 AND member_id = $2`
	argIdx := 3

	if params.Type != "" {
		whereClause += ` AND type = $` + itoa(argIdx)
		args = append(args, params.Type)
		argIdx++
	}
	if params.StartTime != "" {
		whereClause += ` AND created_at >= $` + itoa(argIdx) + `::timestamptz`
		args = append(args, params.StartTime)
		argIdx++
	}
	if params.EndTime != "" {
		whereClause += ` AND created_at <= $` + itoa(argIdx) + `::timestamptz`
		args = append(args, params.EndTime)
		argIdx++
	}

	// Count total.
	var total int
	countQuery := `SELECT COUNT(*) FROM balance_transactions WHERE ` + whereClause
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count transactions", err)
	}

	// Query with pagination.
	offset := (params.Page - 1) * params.PageSize
	dataQuery := `SELECT ` + txColumns + ` FROM balance_transactions
		WHERE ` + whereClause + `
		ORDER BY created_at DESC
		LIMIT $` + itoa(argIdx) + ` OFFSET $` + itoa(argIdx+1)
	args = append(args, params.PageSize, offset)

	rows, err := s.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query transactions", err)
	}
	defer rows.Close()

	transactions := make([]BalanceTransaction, 0)
	for rows.Next() {
		tx, err := scanTransactionRows(rows)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to scan transaction", err)
		}
		transactions = append(transactions, *tx)
	}
	if transactions == nil {
		transactions = make([]BalanceTransaction, 0)
	}

	return &TransactionListResult{
		Transactions: transactions,
		Total:        total,
		Page:         params.Page,
		PageSize:     params.PageSize,
	}, nil
}

// DeductBalance deducts from bonus first, then principal. Must be called within a transaction.
func DeductBalance(ctx context.Context, tx *sql.Tx, memberID, merchantID, amountCents int64, operatorID int64) (*BalanceTransaction, error) {
	// Lock member row and get current balances.
	var principalBefore, bonusBefore int64
	err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(principal_balance_cents, 0), COALESCE(bonus_balance_cents, 0)
		 FROM members WHERE id = $1 AND merchant_id = $2
		 FOR UPDATE`,
		memberID, merchantID,
	).Scan(&principalBefore, &bonusBefore)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to lock member for deduction", err)
	}

	totalBefore := principalBefore + bonusBefore
	if totalBefore < amountCents {
		return nil, apperrors.NewValidationError("insufficient balance")
	}

	// Deduct from bonus first, then principal.
	remaining := amountCents
	bonusDeducted := int64(0)
	principalDeducted := int64(0)

	if bonusBefore > 0 {
		if bonusBefore >= remaining {
			bonusDeducted = remaining
			remaining = 0
		} else {
			bonusDeducted = bonusBefore
			remaining -= bonusBefore
		}
	}
	if remaining > 0 {
		principalDeducted = remaining
	}

	principalAfter := principalBefore - principalDeducted
	bonusAfter := bonusBefore - bonusDeducted
	totalAfter := principalAfter + bonusAfter

	_, err = tx.ExecContext(ctx,
		`UPDATE members SET
		 principal_balance_cents = $1, bonus_balance_cents = $2,
		 balance_cents = $3, updated_at = NOW()
		 WHERE id = $4`,
		principalAfter, bonusAfter, totalAfter, memberID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update balances", err)
	}

	opID := sql.NullInt64{Int64: operatorID, Valid: operatorID > 0}
	var txID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO balance_transactions
		 (merchant_id, member_id, type, amount_cents,
		  principal_before, principal_after, bonus_before, bonus_after,
		  reference_type, operator_id, payment_method, notes)
		 VALUES ($1, $2, 'payment', $3, $4, $5, $6, $7, 'order', $8, 'balance', '')
		 RETURNING id`,
		merchantID, memberID, amountCents,
		principalBefore, principalAfter, bonusBefore, bonusAfter,
		opID,
	).Scan(&txID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to record deduction transaction", err)
	}

	return &BalanceTransaction{
		ID:              txID,
		MerchantID:      merchantID,
		MemberID:        memberID,
		Type:            "payment",
		AmountCents:     amountCents,
		PrincipalBefore: principalBefore,
		PrincipalAfter:  principalAfter,
		BonusBefore:     bonusBefore,
		BonusAfter:      bonusAfter,
	}, nil
}

// RefundBalance adds balance back (principal_balance only) for a refund.
func RefundBalance(ctx context.Context, tx *sql.Tx, memberID, merchantID, amountCents int64, operatorID int64) (*BalanceTransaction, error) {
	var principalBefore, bonusBefore int64
	err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(principal_balance_cents, 0), COALESCE(bonus_balance_cents, 0)
		 FROM members WHERE id = $1 AND merchant_id = $2
		 FOR UPDATE`,
		memberID, merchantID,
	).Scan(&principalBefore, &bonusBefore)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to lock member for refund", err)
	}

	principalAfter := principalBefore + amountCents
	totalAfter := principalAfter + bonusBefore

	_, err = tx.ExecContext(ctx,
		`UPDATE members SET
		 principal_balance_cents = $1, balance_cents = $2, updated_at = NOW()
		 WHERE id = $3`,
		principalAfter, totalAfter, memberID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to refund balance", err)
	}

	opID := sql.NullInt64{Int64: operatorID, Valid: operatorID > 0}
	var txID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO balance_transactions
		 (merchant_id, member_id, type, amount_cents,
		  principal_before, principal_after, bonus_before, bonus_after,
		  reference_type, operator_id, payment_method, notes)
		 VALUES ($1, $2, 'refund', $3, $4, $5, $6, $7, 'order', $8, 'balance', 'refund to stored value')
		 RETURNING id`,
		merchantID, memberID, amountCents,
		principalBefore, principalAfter, bonusBefore, bonusBefore,
		opID,
	).Scan(&txID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to record refund transaction", err)
	}

	return &BalanceTransaction{
		ID:              txID,
		MerchantID:      merchantID,
		MemberID:        memberID,
		Type:            "refund",
		AmountCents:     amountCents,
		PrincipalBefore: principalBefore,
		PrincipalAfter:  principalAfter,
		BonusBefore:     bonusBefore,
		BonusAfter:      bonusBefore,
	}, nil
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
