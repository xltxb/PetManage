package risk

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Service handles risk control logic.
type Service struct {
	db *sql.DB
}

// NewService creates a new risk control service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// GetDB returns the underlying database connection.
func (s *Service) GetDB() *sql.DB {
	return s.db
}

// RiskRule represents a risk control rule.
type RiskRule struct {
	ID                int64     `json:"id"`
	Name              string    `json:"name"`
	RuleType          string    `json:"rule_type"`
	ThresholdCents    int       `json:"threshold_cents"`
	ThresholdCount    int       `json:"threshold_count"`
	TimeWindowMinutes int       `json:"time_window_minutes"`
	Enabled           bool      `json:"enabled"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// RiskAlert represents a triggered risk alert.
type RiskAlert struct {
	ID          int64           `json:"id"`
	RuleID      int64           `json:"rule_id"`
	RuleName    string          `json:"rule_name,omitempty"`
	MerchantID  int64           `json:"merchant_id"`
	OrderID     *int64          `json:"order_id"`
	MemberID    *int64          `json:"member_id"`
	AlertType   string          `json:"alert_type"`
	Description string          `json:"description"`
	Detail      json.RawMessage `json:"detail"`
	Status      string          `json:"status"`
	HandledBy   *int64          `json:"handled_by"`
	HandledAt   *time.Time      `json:"handled_at"`
	CreatedAt   time.Time       `json:"created_at"`
}

// CreateRuleRequest is the request to create a risk rule.
type CreateRuleRequest struct {
	Name              string `json:"name"`
	RuleType          string `json:"rule_type"`
	ThresholdCents    int    `json:"threshold_cents"`
	ThresholdCount    int    `json:"threshold_count"`
	TimeWindowMinutes int    `json:"time_window_minutes"`
	Enabled           *bool  `json:"enabled"`
}

// UpdateRuleRequest is the request to update a risk rule.
type UpdateRuleRequest struct {
	Name              *string `json:"name"`
	RuleType          *string `json:"rule_type"`
	ThresholdCents    *int    `json:"threshold_cents"`
	ThresholdCount    *int    `json:"threshold_count"`
	TimeWindowMinutes *int    `json:"time_window_minutes"`
	Enabled           *bool   `json:"enabled"`
}

// AlertListParams holds filter parameters for alert listing.
type AlertListParams struct {
	MerchantID *int64
	AlertType  string
	Status     string
	Page       int
	PageSize   int
}

// AlertListResponse is a paginated list of alerts.
type AlertListResponse struct {
	Alerts   []RiskAlert `json:"alerts"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// UpdateAlertStatusRequest is the request to update alert status.
type UpdateAlertStatusRequest struct {
	Status string `json:"status"`
}

func (s *Service) validateRuleRequest(name, ruleType string) error {
	if name == "" {
		return apperrors.NewValidationError("rule name is required")
	}
	if ruleType != "large_refund" && ruleType != "high_frequency" {
		return apperrors.NewValidationError("rule_type must be 'large_refund' or 'high_frequency'")
	}
	return nil
}

// CreateRule creates a new risk rule.
func (s *Service) CreateRule(ctx context.Context, req *CreateRuleRequest) (*RiskRule, error) {
	if err := s.validateRuleRequest(req.Name, req.RuleType); err != nil {
		return nil, err
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	rule := &RiskRule{}
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO risk_rules (name, rule_type, threshold_cents, threshold_count, time_window_minutes, enabled)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, rule_type, threshold_cents, threshold_count, time_window_minutes, enabled, created_at, updated_at`,
		req.Name, req.RuleType, req.ThresholdCents, req.ThresholdCount, req.TimeWindowMinutes, enabled,
	).Scan(&rule.ID, &rule.Name, &rule.RuleType, &rule.ThresholdCents, &rule.ThresholdCount,
		&rule.TimeWindowMinutes, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting risk rule: %w", err)
	}
	return rule, nil
}

// ListRules returns all risk rules.
func (s *Service) ListRules(ctx context.Context) ([]RiskRule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, rule_type, threshold_cents, threshold_count, time_window_minutes, enabled, created_at, updated_at
		FROM risk_rules
		ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("querying risk rules: %w", err)
	}
	defer rows.Close()

	var rules []RiskRule
	for rows.Next() {
		var r RiskRule
		if err := rows.Scan(&r.ID, &r.Name, &r.RuleType, &r.ThresholdCents, &r.ThresholdCount,
			&r.TimeWindowMinutes, &r.Enabled, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning risk rule: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// GetRule returns a single risk rule by ID.
func (s *Service) GetRule(ctx context.Context, id int64) (*RiskRule, error) {
	rule := &RiskRule{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, rule_type, threshold_cents, threshold_count, time_window_minutes, enabled, created_at, updated_at
		FROM risk_rules WHERE id = $1`, id,
	).Scan(&rule.ID, &rule.Name, &rule.RuleType, &rule.ThresholdCents, &rule.ThresholdCount,
		&rule.TimeWindowMinutes, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("risk rule not found")
	}
	if err != nil {
		return nil, fmt.Errorf("querying risk rule: %w", err)
	}
	return rule, nil
}

// UpdateRule updates a risk rule.
func (s *Service) UpdateRule(ctx context.Context, id int64, req *UpdateRuleRequest) (*RiskRule, error) {
	existing, err := s.GetRule(ctx, id)
	if err != nil {
		return nil, err
	}

	name := existing.Name
	if req.Name != nil {
		name = *req.Name
	}
	ruleType := existing.RuleType
	if req.RuleType != nil {
		ruleType = *req.RuleType
	}
	thresholdCents := existing.ThresholdCents
	if req.ThresholdCents != nil {
		thresholdCents = *req.ThresholdCents
	}
	thresholdCount := existing.ThresholdCount
	if req.ThresholdCount != nil {
		thresholdCount = *req.ThresholdCount
	}
	timeWindow := existing.TimeWindowMinutes
	if req.TimeWindowMinutes != nil {
		timeWindow = *req.TimeWindowMinutes
	}
	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	if err := s.validateRuleRequest(name, ruleType); err != nil {
		return nil, err
	}

	rule := &RiskRule{}
	err = s.db.QueryRowContext(ctx, `
		UPDATE risk_rules
		SET name=$1, rule_type=$2, threshold_cents=$3, threshold_count=$4,
		    time_window_minutes=$5, enabled=$6, updated_at=NOW()
		WHERE id=$7
		RETURNING id, name, rule_type, threshold_cents, threshold_count, time_window_minutes, enabled, created_at, updated_at`,
		name, ruleType, thresholdCents, thresholdCount, timeWindow, enabled, id,
	).Scan(&rule.ID, &rule.Name, &rule.RuleType, &rule.ThresholdCents, &rule.ThresholdCount,
		&rule.TimeWindowMinutes, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating risk rule: %w", err)
	}
	return rule, nil
}

// DeleteRule deletes a risk rule.
func (s *Service) DeleteRule(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM risk_rules WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting risk rule: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return apperrors.NewNotFoundError("risk rule not found")
	}
	return nil
}

// ToggleRule toggles a risk rule's enabled status.
func (s *Service) ToggleRule(ctx context.Context, id int64) (*RiskRule, error) {
	rule, err := s.GetRule(ctx, id)
	if err != nil {
		return nil, err
	}

	newEnabled := !rule.Enabled
	err = s.db.QueryRowContext(ctx, `
		UPDATE risk_rules SET enabled=$1, updated_at=NOW() WHERE id=$2
		RETURNING id, name, rule_type, threshold_cents, threshold_count, time_window_minutes, enabled, created_at, updated_at`,
		newEnabled, id,
	).Scan(&rule.ID, &rule.Name, &rule.RuleType, &rule.ThresholdCents, &rule.ThresholdCount,
		&rule.TimeWindowMinutes, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("toggling risk rule: %w", err)
	}
	return rule, nil
}

// ListAlerts returns paginated risk alerts with optional filters.
func (s *Service) ListAlerts(ctx context.Context, params *AlertListParams) (*AlertListResponse, error) {
	page := params.Page
	if page < 1 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	where := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if params.MerchantID != nil {
		where += fmt.Sprintf(" AND a.merchant_id = $%d", argIdx)
		args = append(args, *params.MerchantID)
		argIdx++
	}
	if params.AlertType != "" {
		where += fmt.Sprintf(" AND a.alert_type = $%d", argIdx)
		args = append(args, params.AlertType)
		argIdx++
	}
	if params.Status != "" {
		where += fmt.Sprintf(" AND a.status = $%d", argIdx)
		args = append(args, params.Status)
		argIdx++
	}

	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM risk_alerts a %s`, where)
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting risk alerts: %w", err)
	}

	offset := (page - 1) * pageSize
	dataQuery := fmt.Sprintf(`
		SELECT a.id, a.rule_id, COALESCE(r.name, ''), a.merchant_id, a.order_id, a.member_id,
		       a.alert_type, a.description, a.detail, a.status, a.handled_by, a.handled_at, a.created_at
		FROM risk_alerts a
		LEFT JOIN risk_rules r ON a.rule_id = r.id
		%s
		ORDER BY a.created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := s.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("querying risk alerts: %w", err)
	}
	defer rows.Close()

	var alerts []RiskAlert
	for rows.Next() {
		var a RiskAlert
		if err := rows.Scan(&a.ID, &a.RuleID, &a.RuleName, &a.MerchantID, &a.OrderID, &a.MemberID,
			&a.AlertType, &a.Description, &a.Detail, &a.Status, &a.HandledBy, &a.HandledAt, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning risk alert: %w", err)
		}
		alerts = append(alerts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &AlertListResponse{
		Alerts:   alerts,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetAlert returns a single alert by ID.
func (s *Service) GetAlert(ctx context.Context, id int64) (*RiskAlert, error) {
	alert := &RiskAlert{}
	err := s.db.QueryRowContext(ctx, `
		SELECT a.id, a.rule_id, COALESCE(r.name, ''), a.merchant_id, a.order_id, a.member_id,
		       a.alert_type, a.description, a.detail, a.status, a.handled_by, a.handled_at, a.created_at
		FROM risk_alerts a
		LEFT JOIN risk_rules r ON a.rule_id = r.id
		WHERE a.id = $1`, id,
	).Scan(&alert.ID, &alert.RuleID, &alert.RuleName, &alert.MerchantID, &alert.OrderID, &alert.MemberID,
		&alert.AlertType, &alert.Description, &alert.Detail, &alert.Status, &alert.HandledBy, &alert.HandledAt, &alert.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("risk alert not found")
	}
	if err != nil {
		return nil, fmt.Errorf("querying risk alert: %w", err)
	}
	return alert, nil
}

// UpdateAlertStatus updates the status of an alert (processed/ignored).
func (s *Service) UpdateAlertStatus(ctx context.Context, id int64, req *UpdateAlertStatusRequest, handledBy int64) (*RiskAlert, error) {
	if req.Status != "processed" && req.Status != "ignored" {
		return nil, apperrors.NewValidationError("status must be 'processed' or 'ignored'")
	}

	alert := &RiskAlert{}
	now := time.Now()
	err := s.db.QueryRowContext(ctx, `
		UPDATE risk_alerts
		SET status=$1, handled_by=$2, handled_at=$3
		WHERE id=$4
		RETURNING id, rule_id, merchant_id, order_id, member_id, alert_type, description, detail,
		          status, handled_by, handled_at, created_at`,
		req.Status, handledBy, now, id,
	).Scan(&alert.ID, &alert.RuleID, &alert.MerchantID, &alert.OrderID, &alert.MemberID,
		&alert.AlertType, &alert.Description, &alert.Detail, &alert.Status, &alert.HandledBy, &alert.HandledAt, &alert.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("risk alert not found")
	}
	if err != nil {
		return nil, fmt.Errorf("updating alert status: %w", err)
	}
	return alert, nil
}

// CheckLargeRefund checks if a refund exceeds any enabled large_refund rule thresholds.
// If triggered, it creates a risk alert and returns it.
func (s *Service) CheckLargeRefund(ctx context.Context, orderID, merchantID int64, refundAmountCents int) (*RiskAlert, error) {
	// Find enabled large_refund rules where the refund exceeds the threshold
	rule, err := s.findMatchingRule(ctx, "large_refund")
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, nil
	}
	if refundAmountCents < rule.ThresholdCents {
		return nil, nil
	}

	detail, _ := json.Marshal(map[string]interface{}{
		"refund_amount_cents": refundAmountCents,
		"threshold_cents":     rule.ThresholdCents,
		"rule_name":           rule.Name,
	})

	description := fmt.Sprintf("单笔退款金额%d元，超过阈值%d元",
		refundAmountCents/100, rule.ThresholdCents/100)

	return s.createAlert(ctx, rule.ID, merchantID, &orderID, nil, "large_refund", description, detail)
}

// CheckHighFrequency checks if a member has exceeded the transaction frequency threshold
// within the configured time window. If triggered, creates a risk alert.
func (s *Service) CheckHighFrequency(ctx context.Context, merchantID, memberID int64) (*RiskAlert, error) {
	rule, err := s.findMatchingRule(ctx, "high_frequency")
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, nil
	}

	// Count orders for this member within the time window
	since := time.Now().Add(-time.Duration(rule.TimeWindowMinutes) * time.Minute)
	var count int
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM orders
		WHERE member_id = $1 AND merchant_id = $2 AND created_at >= $3`,
		memberID, merchantID, since,
	).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("counting member orders: %w", err)
	}

	if count < rule.ThresholdCount {
		return nil, nil
	}

	detail, _ := json.Marshal(map[string]interface{}{
		"transaction_count":    count,
		"threshold_count":      rule.ThresholdCount,
		"time_window_minutes":  rule.TimeWindowMinutes,
		"rule_name":            rule.Name,
	})

	description := fmt.Sprintf("同一用户%d分钟内交易%d笔，超过阈值%d笔",
		rule.TimeWindowMinutes, count, rule.ThresholdCount)

	return s.createAlert(ctx, rule.ID, merchantID, nil, &memberID, "high_frequency", description, detail)
}

// findMatchingRule finds the first enabled rule of the given type.
func (s *Service) findMatchingRule(ctx context.Context, ruleType string) (*RiskRule, error) {
	rule := &RiskRule{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, rule_type, threshold_cents, threshold_count, time_window_minutes, enabled, created_at, updated_at
		FROM risk_rules
		WHERE rule_type = $1 AND enabled = true
		ORDER BY id LIMIT 1`, ruleType,
	).Scan(&rule.ID, &rule.Name, &rule.RuleType, &rule.ThresholdCents, &rule.ThresholdCount,
		&rule.TimeWindowMinutes, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("finding matching rule: %w", err)
	}
	return rule, nil
}

// createAlert inserts a new risk alert and returns it.
func (s *Service) createAlert(ctx context.Context, ruleID, merchantID int64, orderID, memberID *int64,
	alertType, description string, detail json.RawMessage) (*RiskAlert, error) {

	alert := &RiskAlert{}
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO risk_alerts (rule_id, merchant_id, order_id, member_id, alert_type, description, detail)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, rule_id, merchant_id, order_id, member_id, alert_type, description, detail,
		          status, handled_by, handled_at, created_at`,
		ruleID, merchantID, orderID, memberID, alertType, description, detail,
	).Scan(&alert.ID, &alert.RuleID, &alert.MerchantID, &alert.OrderID, &alert.MemberID,
		&alert.AlertType, &alert.Description, &alert.Detail, &alert.Status, &alert.HandledBy, &alert.HandledAt, &alert.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating risk alert: %w", err)
	}
	return alert, nil
}
