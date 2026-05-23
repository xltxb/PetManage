package points

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// PointsRule represents a configurable points earning rule.
type PointsRule struct {
	ID               int64      `json:"id"`
	MerchantID       int64      `json:"merchant_id"`
	Name             string     `json:"name"`
	RuleType         string     `json:"rule_type"`
	EarnType         string     `json:"earn_type"`
	EarnValue        int        `json:"earn_value"`
	PointsToCentRate int        `json:"points_to_cent_rate"`
	MaxDeductRatio   int        `json:"max_deduct_ratio"`
	ExpiryDays       int        `json:"expiry_days"`
	Status           string     `json:"status"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

// CreateRuleRequest is the request body for creating a points rule.
type CreateRuleRequest struct {
	Name             string `json:"name"`
	RuleType         string `json:"rule_type"`
	EarnType         string `json:"earn_type"`
	EarnValue        int    `json:"earn_value"`
	PointsToCentRate int    `json:"points_to_cent_rate"`
	MaxDeductRatio   int    `json:"max_deduct_ratio"`
	ExpiryDays       int    `json:"expiry_days"`
}

// UpdateRuleRequest is the request body for updating a points rule.
type UpdateRuleRequest struct {
	Name             *string `json:"name"`
	EarnType         *string `json:"earn_type"`
	EarnValue        *int    `json:"earn_value"`
	PointsToCentRate *int    `json:"points_to_cent_rate"`
	MaxDeductRatio   *int    `json:"max_deduct_ratio"`
	ExpiryDays       *int    `json:"expiry_days"`
}

// PointTransaction represents a point change record.
type PointTransaction struct {
	ID            int64     `json:"id"`
	MerchantID    int64     `json:"merchant_id"`
	MemberID      int64     `json:"member_id"`
	Type          string    `json:"type"`
	Points        int       `json:"points"`
	PointsBefore  int       `json:"points_before"`
	PointsAfter   int       `json:"points_after"`
	ReferenceType string    `json:"reference_type"`
	ReferenceID   *int64    `json:"reference_id,omitempty"`
	RuleID        *int64    `json:"rule_id,omitempty"`
	OperatorID    *int64    `json:"operator_id,omitempty"`
	Notes         string    `json:"notes"`
	ExpireAt      *time.Time `json:"expire_at,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// TransactionListParams holds optional filters for listing point transactions.
type TransactionListParams struct {
	Type      string
	StartTime string
	EndTime   string
	Page      int
	PageSize  int
}

// TransactionListResult wraps the transactions list with pagination info.
type TransactionListResult struct {
	Transactions []PointTransaction `json:"transactions"`
	Total        int                `json:"total"`
	Page         int                `json:"page"`
	PageSize     int                `json:"page_size"`
}

// ExpiryAlert represents a member whose points are about to expire.
type ExpiryAlert struct {
	MemberID   int64     `json:"member_id"`
	MemberName string   `json:"member_name"`
	CardNo     string   `json:"card_no"`
	Points     int      `json:"points"`
	ExpireAt   time.Time `json:"expire_at"`
	DaysLeft   int       `json:"days_left"`
}

// Config holds the current effective points configuration for a merchant.
type Config struct {
	PointsToCentRate int `json:"points_to_cent_rate"`
	MaxDeductRatio   int `json:"max_deduct_ratio"`
	ExpiryDays       int `json:"expiry_days"`
}

// Service provides points management operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new points Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

const ruleColumns = `id, merchant_id, name, rule_type, earn_type, earn_value, points_to_cent_rate, max_deduct_ratio, expiry_days, status, created_at, updated_at`

func scanRuleRow(row *sql.Row) (*PointsRule, error) {
	r := &PointsRule{}
	err := row.Scan(&r.ID, &r.MerchantID, &r.Name, &r.RuleType, &r.EarnType, &r.EarnValue,
		&r.PointsToCentRate, &r.MaxDeductRatio, &r.ExpiryDays, &r.Status, &r.CreatedAt, &r.UpdatedAt)
	return r, err
}

func scanRuleRows(rows *sql.Rows) (*PointsRule, error) {
	r := &PointsRule{}
	err := rows.Scan(&r.ID, &r.MerchantID, &r.Name, &r.RuleType, &r.EarnType, &r.EarnValue,
		&r.PointsToCentRate, &r.MaxDeductRatio, &r.ExpiryDays, &r.Status, &r.CreatedAt, &r.UpdatedAt)
	return r, err
}

// CreateRule creates a new points rule.
func (s *Service) CreateRule(ctx context.Context, merchantID int64, req CreateRuleRequest) (*PointsRule, error) {
	if req.Name == "" {
		return nil, apperrors.NewValidationError("rule name is required")
	}
	if req.RuleType != "consume" && req.RuleType != "signin" && req.RuleType != "recharge" && req.RuleType != "referral" {
		return nil, apperrors.NewValidationError("rule_type must be consume, signin, recharge, or referral")
	}
	if req.EarnType != "fixed" && req.EarnType != "percent" {
		return nil, apperrors.NewValidationError("earn_type must be fixed or percent")
	}
	if req.EarnValue <= 0 {
		return nil, apperrors.NewValidationError("earn_value must be positive")
	}
	if req.PointsToCentRate <= 0 {
		req.PointsToCentRate = 100
	}
	if req.MaxDeductRatio < 0 || req.MaxDeductRatio > 100 {
		req.MaxDeductRatio = 50
	}
	if req.ExpiryDays <= 0 {
		req.ExpiryDays = 365
	}

	r, err := scanRuleRow(s.db.QueryRowContext(ctx,
		`INSERT INTO points_rules (merchant_id, name, rule_type, earn_type, earn_value,
		 points_to_cent_rate, max_deduct_ratio, expiry_days, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'active')
		 RETURNING `+ruleColumns,
		merchantID, req.Name, req.RuleType, req.EarnType, req.EarnValue,
		req.PointsToCentRate, req.MaxDeductRatio, req.ExpiryDays,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create points rule", err)
	}
	return r, nil
}

// ListRules lists all points rules for a merchant.
func (s *Service) ListRules(ctx context.Context, merchantID int64) ([]PointsRule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+ruleColumns+` FROM points_rules
		 WHERE merchant_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at DESC`,
		merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list points rules", err)
	}
	defer rows.Close()

	rules := make([]PointsRule, 0)
	for rows.Next() {
		r, err := scanRuleRows(rows)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to scan points rule", err)
		}
		rules = append(rules, *r)
	}
	return rules, nil
}

// GetRule retrieves a single points rule by ID.
func (s *Service) GetRule(ctx context.Context, ruleID, merchantID int64) (*PointsRule, error) {
	r, err := scanRuleRow(s.db.QueryRowContext(ctx,
		`SELECT `+ruleColumns+` FROM points_rules
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		ruleID, merchantID,
	))
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("points rule not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get points rule", err)
	}
	return r, nil
}

// UpdateRule updates a points rule's details.
func (s *Service) UpdateRule(ctx context.Context, ruleID, merchantID int64, req UpdateRuleRequest) (*PointsRule, error) {
	existing, err := s.GetRule(ctx, ruleID, merchantID)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		if *req.Name == "" {
			return nil, apperrors.NewValidationError("rule name is required")
		}
		existing.Name = *req.Name
	}
	if req.EarnType != nil {
		if *req.EarnType != "fixed" && *req.EarnType != "percent" {
			return nil, apperrors.NewValidationError("earn_type must be fixed or percent")
		}
		existing.EarnType = *req.EarnType
	}
	if req.EarnValue != nil {
		if *req.EarnValue <= 0 {
			return nil, apperrors.NewValidationError("earn_value must be positive")
		}
		existing.EarnValue = *req.EarnValue
	}
	if req.PointsToCentRate != nil {
		if *req.PointsToCentRate <= 0 {
			return nil, apperrors.NewValidationError("points_to_cent_rate must be positive")
		}
		existing.PointsToCentRate = *req.PointsToCentRate
	}
	if req.MaxDeductRatio != nil {
		if *req.MaxDeductRatio < 0 || *req.MaxDeductRatio > 100 {
			return nil, apperrors.NewValidationError("max_deduct_ratio must be between 0 and 100")
		}
		existing.MaxDeductRatio = *req.MaxDeductRatio
	}
	if req.ExpiryDays != nil {
		if *req.ExpiryDays <= 0 {
			return nil, apperrors.NewValidationError("expiry_days must be positive")
		}
		existing.ExpiryDays = *req.ExpiryDays
	}

	r, err := scanRuleRow(s.db.QueryRowContext(ctx,
		`UPDATE points_rules SET name=$1, earn_type=$2, earn_value=$3,
		 points_to_cent_rate=$4, max_deduct_ratio=$5, expiry_days=$6, updated_at=NOW()
		 WHERE id=$7 AND merchant_id=$8 AND deleted_at IS NULL
		 RETURNING `+ruleColumns,
		existing.Name, existing.EarnType, existing.EarnValue,
		existing.PointsToCentRate, existing.MaxDeductRatio, existing.ExpiryDays,
		ruleID, merchantID,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update points rule", err)
	}
	return r, nil
}

// DeleteRule soft-deletes a points rule.
func (s *Service) DeleteRule(ctx context.Context, ruleID, merchantID int64) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE points_rules SET deleted_at = NOW() WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		ruleID, merchantID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to delete points rule", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return apperrors.NewNotFoundError("points rule not found")
	}
	return nil
}

// ToggleRule toggles a points rule's status between active/inactive.
func (s *Service) ToggleRule(ctx context.Context, ruleID, merchantID int64) (*PointsRule, error) {
	existing, err := s.GetRule(ctx, ruleID, merchantID)
	if err != nil {
		return nil, err
	}
	newStatus := "inactive"
	if existing.Status == "inactive" {
		newStatus = "active"
	}
	r, err := scanRuleRow(s.db.QueryRowContext(ctx,
		`UPDATE points_rules SET status=$1, updated_at=NOW()
		 WHERE id=$2 AND merchant_id=$3 AND deleted_at IS NULL
		 RETURNING `+ruleColumns,
		newStatus, ruleID, merchantID,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to toggle points rule", err)
	}
	return r, nil
}

// GetActiveConsumeRule returns the active consume-type rule for a merchant.
func (s *Service) GetActiveConsumeRule(ctx context.Context, merchantID int64) (*PointsRule, error) {
	r, err := scanRuleRow(s.db.QueryRowContext(ctx,
		`SELECT `+ruleColumns+` FROM points_rules
		 WHERE merchant_id = $1 AND rule_type = 'consume' AND status = 'active' AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT 1`,
		merchantID,
	))
	if err == sql.ErrNoRows {
		return nil, nil // No active rule, not an error
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get consume rule", err)
	}
	return r, nil
}

// GetEffectiveConfig returns the effective points configuration for a merchant.
func (s *Service) GetEffectiveConfig(ctx context.Context, merchantID int64) *Config {
	cfg := &Config{
		PointsToCentRate: 100,
		MaxDeductRatio:   50,
		ExpiryDays:       365,
	}
	r, err := s.GetActiveConsumeRule(ctx, merchantID)
	if err == nil && r != nil {
		cfg.PointsToCentRate = r.PointsToCentRate
		cfg.MaxDeductRatio = r.MaxDeductRatio
		cfg.ExpiryDays = r.ExpiryDays
	}
	return cfg
}

const txColumns = `id, merchant_id, member_id, type, points, points_before, points_after,
	reference_type, reference_id, rule_id, operator_id, notes, expire_at, created_at`

func scanTxRows(rows *sql.Rows) (*PointTransaction, error) {
	t := &PointTransaction{}
	var refID, ruleID, opID sql.NullInt64
	var expireAt sql.NullTime
	err := rows.Scan(&t.ID, &t.MerchantID, &t.MemberID, &t.Type, &t.Points,
		&t.PointsBefore, &t.PointsAfter,
		&t.ReferenceType, &refID, &ruleID, &opID, &t.Notes, &expireAt, &t.CreatedAt)
	if refID.Valid {
		t.ReferenceID = &refID.Int64
	}
	if ruleID.Valid {
		t.RuleID = &ruleID.Int64
	}
	if opID.Valid {
		t.OperatorID = &opID.Int64
	}
	if expireAt.Valid {
		t.ExpireAt = &expireAt.Time
	}
	return t, err
}

// EarnPointsAfterCheckout awards points to a member after a successful checkout (non-transactional).
func (s *Service) EarnPointsAfterCheckout(ctx context.Context, merchantID, memberID, orderID, paidCents int64) (*PointTransaction, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction for points earn", err)
	}
	defer tx.Rollback()

	pt, err := EarnPoints(ctx, tx, merchantID, memberID, orderID, paidCents)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit points earn", err)
	}

	return pt, nil
}

// EarnPoints awards points to a member based on the consume rule. Must be called within a transaction.
func EarnPoints(ctx context.Context, tx *sql.Tx, merchantID, memberID, orderID, paidCents int64) (*PointTransaction, error) {
	// Find active consume rule.
	var ruleID sql.NullInt64
	var earnType string
	var earnValue, expiryDays int
	err := tx.QueryRowContext(ctx,
		`SELECT id, earn_type, earn_value, expiry_days FROM points_rules
		 WHERE merchant_id = $1 AND rule_type = 'consume' AND status = 'active' AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT 1`,
		merchantID,
	).Scan(&ruleID, &earnType, &earnValue, &expiryDays)
	if err == sql.ErrNoRows {
		return nil, nil // No active consume rule, nothing to earn
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get consume rule", err)
	}

	// Calculate points to earn.
	var pointsEarned int
	if earnType == "fixed" {
		pointsEarned = earnValue
	} else {
		// Percent: earn_value points per 100 cents (i.e., earn_value=100 means 1 point per cent)
		pointsEarned = int(int64(paidCents) * int64(earnValue) / 10000)
	}
	if pointsEarned <= 0 {
		return nil, nil
	}

	// Lock member and get current points.
	var pointsBefore int
	var expireAt sql.NullTime
	err = tx.QueryRowContext(ctx,
		`SELECT COALESCE(points, 0), COALESCE(points_expire_at, NULL)
		 FROM members WHERE id = $1 AND merchant_id = $2
		 FOR UPDATE`,
		memberID, merchantID,
	).Scan(&pointsBefore, &expireAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to lock member for points earn", err)
	}

	pointsAfter := pointsBefore + pointsEarned

	// Calculate new expire_at: use the rule's expiry_days from now.
	newExpireAt := time.Now().AddDate(0, 0, expiryDays)
	// If existing expire_at is sooner and has points, use the further one; otherwise use new one.
	// Simplification: set expire_at to the new expiry time; existing points follow the same clock.
	updateExpireAt := sql.NullTime{Time: newExpireAt, Valid: true}
	if expireAt.Valid && pointsBefore > 0 && expireAt.Time.Before(newExpireAt) {
		// Weighted average: existing points keep their expiry, new points get new expiry.
		// Simplified: use the later expiry for all points (generous).
		updateExpireAt = sql.NullTime{Time: newExpireAt, Valid: true}
	}

	// Update member points.
	var effectiveExpire interface{}
	if updateExpireAt.Valid {
		effectiveExpire = updateExpireAt.Time
	} else {
		effectiveExpire = nil
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE members SET points = $1, points_expire_at = $2, updated_at = NOW() WHERE id = $3`,
		pointsAfter, effectiveExpire, memberID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update member points", err)
	}

	// Record point transaction.
	refID := sql.NullInt64{Int64: orderID, Valid: true}
	var txID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO point_transactions
		 (merchant_id, member_id, type, points, points_before, points_after,
		  reference_type, reference_id, rule_id, notes, expire_at)
		 VALUES ($1, $2, 'earn', $3, $4, $5, 'order', $6, $7, $8, $9)
		 RETURNING id`,
		merchantID, memberID, pointsEarned, pointsBefore, pointsAfter,
		refID, ruleID, "", effectiveExpire,
	).Scan(&txID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to record point transaction", err)
	}

	return &PointTransaction{
		ID:            txID,
		MerchantID:    merchantID,
		MemberID:      memberID,
		Type:          "earn",
		Points:        pointsEarned,
		PointsBefore:  pointsBefore,
		PointsAfter:   pointsAfter,
		ReferenceType: "order",
		ReferenceID:   &orderID,
		RuleID:        &ruleID.Int64,
		ExpireAt:      &newExpireAt,
	}, nil
}

// DeductPoints deducts points from a member and records the transaction.
// Must be called within a transaction. Called during checkout.
func DeductPoints(ctx context.Context, tx *sql.Tx, merchantID, memberID, orderID, pointsToDeduct int64, operatorID int64) (*PointTransaction, error) {
	var pointsBefore int
	err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(points, 0) FROM members WHERE id = $1 AND merchant_id = $2 FOR UPDATE`,
		memberID, merchantID,
	).Scan(&pointsBefore)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to lock member for points deduction", err)
	}

	if int64(pointsBefore) < pointsToDeduct {
		return nil, apperrors.NewValidationError("insufficient points")
	}

	pointsAfter := pointsBefore - int(pointsToDeduct)

	_, err = tx.ExecContext(ctx,
		`UPDATE members SET points = $1, updated_at = NOW() WHERE id = $2`,
		pointsAfter, memberID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update member points", err)
	}

	refID := sql.NullInt64{Int64: orderID, Valid: true}
	opID := sql.NullInt64{Int64: operatorID, Valid: operatorID > 0}
	var txID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO point_transactions
		 (merchant_id, member_id, type, points, points_before, points_after,
		  reference_type, reference_id, operator_id, notes)
		 VALUES ($1, $2, 'deduct', $3, $4, $5, 'order', $6, $7, '')
		 RETURNING id`,
		merchantID, memberID, -int(pointsToDeduct), pointsBefore, pointsAfter,
		refID, opID,
	).Scan(&txID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to record point deduction transaction", err)
	}

	return &PointTransaction{
		ID:            txID,
		MerchantID:    merchantID,
		MemberID:      memberID,
		Type:          "deduct",
		Points:        -int(pointsToDeduct),
		PointsBefore:  pointsBefore,
		PointsAfter:   pointsAfter,
		ReferenceType: "order",
		ReferenceID:   &orderID,
		OperatorID:    &operatorID,
	}, nil
}

// RecordPointsDeduction records a point deduction transaction after checkout.
func (s *Service) RecordPointsDeduction(ctx context.Context, merchantID, memberID, orderID, pointsDeducted int64, operatorID int64) error {
	var pointsBefore int
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(points, 0) FROM members WHERE id = $1 AND merchant_id = $2`,
		memberID, merchantID,
	).Scan(&pointsBefore)
	if err != nil {
		return err
	}

	refID := sql.NullInt64{Int64: orderID, Valid: true}
	opID := sql.NullInt64{Int64: operatorID, Valid: operatorID > 0}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO point_transactions
		 (merchant_id, member_id, type, points, points_before, points_after,
		  reference_type, reference_id, operator_id, notes)
		 VALUES ($1, $2, 'deduct', $3, $4, $5, 'order', $6, $7, '')`,
		merchantID, memberID, -int(pointsDeducted), pointsBefore, pointsBefore-int(pointsDeducted),
		refID, opID,
	)
	return err
}

// ListTransactions lists point transactions for a member with optional filters.
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

	var total int
	countQuery := `SELECT COUNT(*) FROM point_transactions WHERE ` + whereClause
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count point transactions", err)
	}

	offset := (params.Page - 1) * params.PageSize
	dataQuery := `SELECT ` + txColumns + ` FROM point_transactions
		WHERE ` + whereClause + `
		ORDER BY created_at DESC
		LIMIT $` + itoa(argIdx) + ` OFFSET $` + itoa(argIdx+1)
	args = append(args, params.PageSize, offset)

	rows, err := s.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query point transactions", err)
	}
	defer rows.Close()

	transactions := make([]PointTransaction, 0)
	for rows.Next() {
		tx, err := scanTxRows(rows)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to scan point transaction", err)
		}
		transactions = append(transactions, *tx)
	}
	if transactions == nil {
		transactions = make([]PointTransaction, 0)
	}

	return &TransactionListResult{
		Transactions: transactions,
		Total:        total,
		Page:         params.Page,
		PageSize:     params.PageSize,
	}, nil
}

// GetExpiryAlerts returns members whose points will expire within the given days.
func (s *Service) GetExpiryAlerts(ctx context.Context, merchantID int64, withinDays int) ([]ExpiryAlert, error) {
	if withinDays <= 0 {
		withinDays = 30
	}
	cutoff := time.Now().AddDate(0, 0, withinDays)

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, card_no, COALESCE(points, 0), points_expire_at
		 FROM members
		 WHERE merchant_id = $1 AND points > 0
		   AND points_expire_at IS NOT NULL
		   AND points_expire_at <= $2
		   AND points_expire_at > NOW()
		   AND deleted_at IS NULL
		 ORDER BY points_expire_at ASC`,
		merchantID, cutoff,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get expiry alerts", err)
	}
	defer rows.Close()

	alerts := make([]ExpiryAlert, 0)
	for rows.Next() {
		var a ExpiryAlert
		var expireAt time.Time
		if err := rows.Scan(&a.MemberID, &a.MemberName, &a.CardNo, &a.Points, &expireAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan expiry alert", err)
		}
		a.ExpireAt = expireAt
		a.DaysLeft = int(time.Until(expireAt).Hours() / 24)
		if a.DaysLeft < 0 {
			a.DaysLeft = 0
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
