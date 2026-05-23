package promotion

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Service handles promotion activity business logic.
type Service struct {
	db *sql.DB
}

// NewService creates a new promotion service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// ActivityType constants.
const (
	TypeFullReduction = "full_reduction"
	TypeDiscount      = "discount"
	TypeFlashSale     = "flash_sale"
	TypeGroupBuy      = "group_buy"
)

// Activity represents a promotion activity.
type Activity struct {
	ID                int64           `json:"id"`
	MerchantID        int64           `json:"merchant_id"`
	Name              string          `json:"name"`
	Type              string          `json:"type"`
	Rules             json.RawMessage `json:"rules"`
	StartTime         string          `json:"start_time"`
	EndTime           string          `json:"end_time"`
	TotalOrderCount   int             `json:"total_order_count"`
	TotalDiscountCents int64          `json:"total_discount_cents"`
	Status            string          `json:"status"`
	CreatedAt         string          `json:"created_at"`
	UpdatedAt         string          `json:"updated_at"`
}

// FullReductionRules defines rules for a full_reduction activity.
type FullReductionRules struct {
	ThresholdCents int `json:"threshold_cents"`
	ReduceCents    int `json:"reduce_cents"`
}

// DiscountRules defines rules for a discount activity.
type DiscountRules struct {
	DiscountPercent int `json:"discount_percent"` // e.g. 85 means 85折 (15% off)
}

// FlashSaleRules defines rules for a flash_sale activity.
type FlashSaleRules struct {
	ProductID      int64  `json:"product_id"`
	ProductName    string `json:"product_name"`
	ProductBarcode string `json:"product_barcode,omitempty"`
	FlashPriceCents int   `json:"flash_price_cents"`
	FlashStock     int    `json:"flash_stock"`
	RemainingStock int    `json:"remaining_stock"`
}

// GroupBuyRules defines rules for a group_buy activity.
type GroupBuyRules struct {
	ProductID       int64  `json:"product_id"`
	ProductName     string `json:"product_name"`
	ProductBarcode  string `json:"product_barcode,omitempty"`
	GroupPriceCents int    `json:"group_price_cents"`
	RequiredCount   int    `json:"required_count"`
}

// CreateActivityRequest is the input for creating an activity.
type CreateActivityRequest struct {
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Rules     json.RawMessage `json:"rules"`
	StartTime string          `json:"start_time"`
	EndTime   string          `json:"end_time"`
}

// UpdateActivityRequest is the input for updating an activity.
type UpdateActivityRequest struct {
	Name      *string          `json:"name,omitempty"`
	Rules     *json.RawMessage `json:"rules,omitempty"`
	StartTime *string          `json:"start_time,omitempty"`
	EndTime   *string          `json:"end_time,omitempty"`
}

// ListParams holds filtering and pagination parameters.
type ListParams struct {
	Type     string
	Status   string
	Page     int
	PageSize int
}

// ListResult holds paginated activity results.
type ListResult struct {
	Activities []Activity `json:"activities"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
}

// Stats holds activity statistics.
type Stats struct {
	TotalActivities   int   `json:"total_activities"`
	ActiveActivities  int   `json:"active_activities"`
	TotalOrders       int   `json:"total_orders"`
	TotalDiscountCents int64 `json:"total_discount_cents"`
}

// ActivePromotion holds a currently active promotion for checkout calculation.
type ActivePromotion struct {
	ID    int64           `json:"id"`
	Name  string          `json:"name"`
	Type  string          `json:"type"`
	Rules json.RawMessage `json:"rules"`
}

// validateType checks if the activity type is valid.
func validateType(t string) bool {
	switch t {
	case TypeFullReduction, TypeDiscount, TypeFlashSale, TypeGroupBuy:
		return true
	}
	return false
}

// CreateActivity creates a new promotion activity.
func (s *Service) CreateActivity(ctx context.Context, merchantID int64, req CreateActivityRequest) (*Activity, error) {
	if req.Name == "" {
		return nil, apperrors.NewValidationError("activity name is required")
	}
	if !validateType(req.Type) {
		return nil, apperrors.NewValidationError("invalid activity type: must be full_reduction, discount, flash_sale, or group_buy")
	}
	if req.StartTime == "" || req.EndTime == "" {
		return nil, apperrors.NewValidationError("start_time and end_time are required")
	}

	// Validate time format.
	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		return nil, apperrors.NewValidationError("invalid start_time format, use RFC3339")
	}
	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		return nil, apperrors.NewValidationError("invalid end_time format, use RFC3339")
	}
	if !endTime.After(startTime) {
		return nil, apperrors.NewValidationError("end_time must be after start_time")
	}

	// Validate rules according to type.
	if err := ValidateRules(req.Type, req.Rules); err != nil {
		return nil, err
	}

	// Handle flash_sale remaining_stock initialization.
	rules := req.Rules
	if req.Type == TypeFlashSale {
		var fsr FlashSaleRules
		json.Unmarshal(req.Rules, &fsr)
		fsr.RemainingStock = fsr.FlashStock
		rules, _ = json.Marshal(fsr)
	}

	var a Activity
	var rulesRaw json.RawMessage
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO promotion_activities (merchant_id, name, type, rules, start_time, end_time, status)
		 VALUES ($1, $2, $3, $4, $5, $6, 'active')
		 RETURNING id, merchant_id, name, type, rules, start_time, end_time, total_order_count, total_discount_cents, status, created_at, updated_at`,
		merchantID, req.Name, req.Type, rules, req.StartTime, req.EndTime,
	).Scan(&a.ID, &a.MerchantID, &a.Name, &a.Type, &rulesRaw, &a.StartTime, &a.EndTime,
		&a.TotalOrderCount, &a.TotalDiscountCents, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create promotion activity", err)
	}

	a.Rules = rulesRaw
	a.StartTime = formatTimeStr(a.StartTime)
	a.EndTime = formatTimeStr(a.EndTime)
	a.CreatedAt = formatTimeStr(a.CreatedAt)
	a.UpdatedAt = formatTimeStr(a.UpdatedAt)
	return &a, nil
}

// GetActivity retrieves a single activity by ID.
func (s *Service) GetActivity(ctx context.Context, merchantID, activityID int64) (*Activity, error) {
	var a Activity
	var rulesRaw json.RawMessage
	err := s.db.QueryRowContext(ctx,
		`SELECT id, merchant_id, name, type, rules, start_time, end_time, total_order_count, total_discount_cents, status, created_at, updated_at
		 FROM promotion_activities
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		activityID, merchantID,
	).Scan(&a.ID, &a.MerchantID, &a.Name, &a.Type, &rulesRaw, &a.StartTime, &a.EndTime,
		&a.TotalOrderCount, &a.TotalDiscountCents, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("promotion activity not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query promotion activity", err)
	}

	a.Rules = rulesRaw
	a.StartTime = formatTimeStr(a.StartTime)
	a.EndTime = formatTimeStr(a.EndTime)
	a.CreatedAt = formatTimeStr(a.CreatedAt)
	a.UpdatedAt = formatTimeStr(a.UpdatedAt)
	return &a, nil
}

// ListActivities queries activities with filtering and pagination.
func (s *Service) ListActivities(ctx context.Context, merchantID int64, params ListParams) (*ListResult, error) {
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
	conditions := []string{"merchant_id = $1", "deleted_at IS NULL"}
	argIdx := 2

	if params.Type != "" {
		conditions = append(conditions, "type = $"+strconv.Itoa(argIdx))
		args = append(args, params.Type)
		argIdx++
	}
	if params.Status != "" {
		conditions = append(conditions, "status = $"+strconv.Itoa(argIdx))
		args = append(args, params.Status)
		argIdx++
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM promotion_activities WHERE " + buildConditions(conditions)
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count promotion activities", err)
	}

	offset := (params.Page - 1) * params.PageSize
	dataQuery := `SELECT id, merchant_id, name, type, rules, start_time, end_time, total_order_count, total_discount_cents, status, created_at, updated_at
		FROM promotion_activities WHERE ` + buildConditions(conditions) +
		` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, params.PageSize, offset)

	rows, err := s.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query promotion activities", err)
	}
	defer rows.Close()

	activities := []Activity{}
	for rows.Next() {
		var a Activity
		var rulesRaw json.RawMessage
		if err := rows.Scan(&a.ID, &a.MerchantID, &a.Name, &a.Type, &rulesRaw, &a.StartTime, &a.EndTime,
			&a.TotalOrderCount, &a.TotalDiscountCents, &a.Status, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan promotion activity", err)
		}
		a.Rules = rulesRaw
		a.StartTime = formatTimeStr(a.StartTime)
		a.EndTime = formatTimeStr(a.EndTime)
		a.CreatedAt = formatTimeStr(a.CreatedAt)
		a.UpdatedAt = formatTimeStr(a.UpdatedAt)
		activities = append(activities, a)
	}

	return &ListResult{
		Activities: activities,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
	}, nil
}

// UpdateActivity partially updates an existing activity.
func (s *Service) UpdateActivity(ctx context.Context, merchantID, activityID int64, req UpdateActivityRequest) (*Activity, error) {
	// Get existing activity first.
	existing, err := s.GetActivity(ctx, merchantID, activityID)
	if err != nil {
		return nil, err
	}

	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	if req.Name != nil {
		if *req.Name == "" {
			return nil, apperrors.NewValidationError("activity name cannot be empty")
		}
		setClauses = append(setClauses, "name = $"+strconv.Itoa(argIdx))
		args = append(args, *req.Name)
		argIdx++
	}

	if req.Rules != nil {
		if err := ValidateRules(existing.Type, *req.Rules); err != nil {
			return nil, err
		}
		setClauses = append(setClauses, "rules = $"+strconv.Itoa(argIdx))
		args = append(args, *req.Rules)
		argIdx++
	}

	if req.StartTime != nil {
		_, err := time.Parse(time.RFC3339, *req.StartTime)
		if err != nil {
			return nil, apperrors.NewValidationError("invalid start_time format, use RFC3339")
		}
		setClauses = append(setClauses, "start_time = $"+strconv.Itoa(argIdx))
		args = append(args, *req.StartTime)
		argIdx++
	}

	if req.EndTime != nil {
		_, err := time.Parse(time.RFC3339, *req.EndTime)
		if err != nil {
			return nil, apperrors.NewValidationError("invalid end_time format, use RFC3339")
		}
		setClauses = append(setClauses, "end_time = $"+strconv.Itoa(argIdx))
		args = append(args, *req.EndTime)
		argIdx++
	}

	if len(setClauses) == 0 {
		return existing, nil
	}

	setClauses = append(setClauses, "updated_at = NOW()")
	query := `UPDATE promotion_activities SET ` + buildConditions(setClauses) +
		` WHERE id = $` + strconv.Itoa(argIdx) + ` AND merchant_id = $` + strconv.Itoa(argIdx+1) +
		` AND deleted_at IS NULL RETURNING id, merchant_id, name, type, rules, start_time, end_time, total_order_count, total_discount_cents, status, created_at, updated_at`
	args = append(args, activityID, merchantID)

	var a Activity
	var rulesRaw json.RawMessage
	err = s.db.QueryRowContext(ctx, query, args...).Scan(&a.ID, &a.MerchantID, &a.Name, &a.Type, &rulesRaw, &a.StartTime, &a.EndTime,
		&a.TotalOrderCount, &a.TotalDiscountCents, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("promotion activity not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update promotion activity", err)
	}

	a.Rules = rulesRaw
	a.StartTime = formatTimeStr(a.StartTime)
	a.EndTime = formatTimeStr(a.EndTime)
	a.CreatedAt = formatTimeStr(a.CreatedAt)
	a.UpdatedAt = formatTimeStr(a.UpdatedAt)
	return &a, nil
}

// DeleteActivity soft-deletes an activity.
func (s *Service) DeleteActivity(ctx context.Context, merchantID, activityID int64) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE promotion_activities SET deleted_at = NOW() WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		activityID, merchantID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to delete promotion activity", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return apperrors.NewNotFoundError("promotion activity not found")
	}
	return nil
}

// ToggleActivity toggles between active and disabled.
func (s *Service) ToggleActivity(ctx context.Context, merchantID, activityID int64) (*Activity, error) {
	var a Activity
	var rulesRaw json.RawMessage
	err := s.db.QueryRowContext(ctx,
		`UPDATE promotion_activities
		 SET status = CASE WHEN status = 'active' THEN 'disabled' ELSE 'active' END, updated_at = NOW()
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL AND status != 'ended'
		 RETURNING id, merchant_id, name, type, rules, start_time, end_time, total_order_count, total_discount_cents, status, created_at, updated_at`,
		activityID, merchantID,
	).Scan(&a.ID, &a.MerchantID, &a.Name, &a.Type, &rulesRaw, &a.StartTime, &a.EndTime,
		&a.TotalOrderCount, &a.TotalDiscountCents, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("promotion activity not found or already ended")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to toggle promotion activity", err)
	}

	a.Rules = rulesRaw
	a.StartTime = formatTimeStr(a.StartTime)
	a.EndTime = formatTimeStr(a.EndTime)
	a.CreatedAt = formatTimeStr(a.CreatedAt)
	a.UpdatedAt = formatTimeStr(a.UpdatedAt)
	return &a, nil
}

// GetStats returns activity statistics for a merchant.
func (s *Service) GetStats(ctx context.Context, merchantID int64) (*Stats, error) {
	var stats Stats
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(total_order_count), 0), COALESCE(SUM(total_discount_cents), 0)
		 FROM promotion_activities WHERE merchant_id = $1 AND deleted_at IS NULL`,
		merchantID,
	).Scan(&stats.TotalActivities, &stats.TotalOrders, &stats.TotalDiscountCents)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get promotion stats", err)
	}

	_ = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM promotion_activities
		 WHERE merchant_id = $1 AND status = 'active' AND deleted_at IS NULL`, merchantID,
	).Scan(&stats.ActiveActivities)

	return &stats, nil
}

// GetActivePromotions returns currently active promotions for a merchant whose
// time windows are in effect.
func (s *Service) GetActivePromotions(ctx context.Context, merchantID int64) ([]ActivePromotion, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, type, rules FROM promotion_activities
		 WHERE merchant_id = $1 AND status = 'active' AND deleted_at IS NULL
		   AND start_time <= $2 AND end_time >= $2
		 ORDER BY type`,
		merchantID, now,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query active promotions", err)
	}
	defer rows.Close()

	var promotions []ActivePromotion
	for rows.Next() {
		var p ActivePromotion
		var rulesRaw json.RawMessage
		if err := rows.Scan(&p.ID, &p.Name, &p.Type, &rulesRaw); err != nil {
			return nil, apperrors.NewInternalError("failed to scan active promotion", err)
		}
		p.Rules = rulesRaw
		promotions = append(promotions, p)
	}
	return promotions, nil
}

// RecordPromotionUsage increments the usage stats for a promotion activity.
// This is called from within the checkout transaction.
func (s *Service) RecordPromotionUsageTx(ctx context.Context, tx *sql.Tx, activityID int64, discountCents int) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE promotion_activities
		 SET total_order_count = total_order_count + 1,
		     total_discount_cents = total_discount_cents + $1,
		     updated_at = NOW()
		 WHERE id = $2`,
		discountCents, activityID,
	)
	return err
}

// DeductFlashSaleStock deducts flash sale stock within a transaction.
func (s *Service) DeductFlashSaleStockTx(ctx context.Context, tx *sql.Tx, activityID int64, quantity int) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE promotion_activities
		 SET rules = jsonb_set(rules, '{remaining_stock}', to_jsonb(
		   (COALESCE((rules->>'remaining_stock')::int, (rules->>'flash_stock')::int) - $1)
		 ))
		 WHERE id = $2 AND COALESCE((rules->>'remaining_stock')::int, (rules->>'flash_stock')::int) >= $1`,
		quantity, activityID,
	)
	return err
}

// ValidateRules validates rules JSON for a given activity type.
func ValidateRules(activityType string, rules json.RawMessage) error {
	switch activityType {
	case TypeFullReduction:
		var fr FullReductionRules
		if err := json.Unmarshal(rules, &fr); err != nil {
			return apperrors.NewValidationError("invalid rules format for full_reduction")
		}
		if fr.ThresholdCents <= 0 {
			return apperrors.NewValidationError("threshold_cents must be positive")
		}
		if fr.ReduceCents <= 0 {
			return apperrors.NewValidationError("reduce_cents must be positive")
		}
		if fr.ReduceCents >= fr.ThresholdCents {
			return apperrors.NewValidationError("reduce_cents must be less than threshold_cents")
		}
	case TypeDiscount:
		var d DiscountRules
		if err := json.Unmarshal(rules, &d); err != nil {
			return apperrors.NewValidationError("invalid rules format for discount")
		}
		if d.DiscountPercent <= 0 || d.DiscountPercent >= 100 {
			return apperrors.NewValidationError("discount_percent must be between 1 and 99")
		}
	case TypeFlashSale:
		var fs FlashSaleRules
		if err := json.Unmarshal(rules, &fs); err != nil {
			return apperrors.NewValidationError("invalid rules format for flash_sale")
		}
		if fs.ProductID <= 0 {
			return apperrors.NewValidationError("product_id is required for flash_sale")
		}
		if fs.FlashPriceCents <= 0 {
			return apperrors.NewValidationError("flash_price_cents must be positive")
		}
		if fs.FlashStock <= 0 {
			return apperrors.NewValidationError("flash_stock must be positive")
		}
	case TypeGroupBuy:
		var gb GroupBuyRules
		if err := json.Unmarshal(rules, &gb); err != nil {
			return apperrors.NewValidationError("invalid rules format for group_buy")
		}
		if gb.ProductID <= 0 {
			return apperrors.NewValidationError("product_id is required for group_buy")
		}
		if gb.GroupPriceCents <= 0 {
			return apperrors.NewValidationError("group_price_cents must be positive")
		}
		if gb.RequiredCount <= 1 {
			return apperrors.NewValidationError("required_count must be at least 2")
		}
	}
	return nil
}

func buildConditions(clauses []string) string {
	res := ""
	for i, c := range clauses {
		if i > 0 {
			res += " AND "
		}
		res += c
	}
	return res
}

func formatTimeStr(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Format(time.RFC3339)
}
