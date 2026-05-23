package membertag

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Tag represents a member tag definition.
type Tag struct {
	ID          int64      `json:"id"`
	MerchantID  int64      `json:"merchant_id"`
	Name        string     `json:"name"`
	Color       string     `json:"color"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

// MemberTag represents a tag attached to a member.
type MemberTag struct {
	TagID      int64     `json:"tag_id"`
	MemberID   int64     `json:"member_id"`
	TagName    string    `json:"tag_name"`
	Color      string    `json:"color"`
	CreatedAt  time.Time `json:"created_at"`
}

// TagRule represents an auto-tagging rule.
type TagRule struct {
	ID             int64      `json:"id"`
	MerchantID     int64      `json:"merchant_id"`
	TagID          int64      `json:"tag_id"`
	TagName        string     `json:"tag_name,omitempty"`
	RuleType       string     `json:"rule_type"`
	Operator       string     `json:"operator"`
	ThresholdValue float64    `json:"threshold_value"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}

// CreateTagRequest is the request body for creating a tag.
type CreateTagRequest struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

// UpdateTagRequest is the request body for updating a tag (partial).
type UpdateTagRequest struct {
	Name        *string `json:"name"`
	Color       *string `json:"color"`
	Description *string `json:"description"`
}

// CreateRuleRequest is the request body for creating an auto-tag rule.
type CreateRuleRequest struct {
	TagID          int64   `json:"tag_id"`
	RuleType       string  `json:"rule_type"`
	Operator       string  `json:"operator"`
	ThresholdValue float64 `json:"threshold_value"`
}

// UpdateRuleRequest is the request body for updating an auto-tag rule.
type UpdateRuleRequest struct {
	RuleType       *string  `json:"rule_type"`
	Operator       *string  `json:"operator"`
	ThresholdValue *float64 `json:"threshold_value"`
}

// AddMemberTagRequest is the request body for adding tags to a member.
type AddMemberTagRequest struct {
	TagIDs []int64 `json:"tag_ids"`
}

// Service provides member tag management operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// --- Tag CRUD ---

// CreateTag creates a new tag.
func (s *Service) CreateTag(ctx context.Context, merchantID int64, req CreateTagRequest) (*Tag, error) {
	if req.Name == "" {
		return nil, apperrors.NewValidationError("tag name is required")
	}
	if req.Color == "" {
		req.Color = "#3B82F6"
	}
	t := &Tag{}
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO member_tags (merchant_id, name, color, description)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, merchant_id, name, color, description, status, created_at, updated_at`,
		merchantID, req.Name, req.Color, req.Description,
	).Scan(&t.ID, &t.MerchantID, &t.Name, &t.Color, &t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create tag", err)
	}
	return t, nil
}

// ListTags returns all tags for a merchant.
func (s *Service) ListTags(ctx context.Context, merchantID int64) ([]Tag, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, merchant_id, name, color, description, status, created_at, updated_at
		 FROM member_tags
		 WHERE merchant_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at DESC`,
		merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list tags", err)
	}
	defer rows.Close()

	tags := make([]Tag, 0)
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.MerchantID, &t.Name, &t.Color, &t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan tag", err)
		}
		tags = append(tags, t)
	}
	return tags, nil
}

// GetTag returns a single tag.
func (s *Service) GetTag(ctx context.Context, merchantID, tagID int64) (*Tag, error) {
	t := &Tag{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, merchant_id, name, color, description, status, created_at, updated_at
		 FROM member_tags
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		tagID, merchantID,
	).Scan(&t.ID, &t.MerchantID, &t.Name, &t.Color, &t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("tag not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get tag", err)
	}
	return t, nil
}

// UpdateTag updates a tag (partial).
func (s *Service) UpdateTag(ctx context.Context, merchantID, tagID int64, req UpdateTagRequest) (*Tag, error) {
	_, err := s.GetTag(ctx, merchantID, tagID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		if *req.Name == "" {
			return nil, apperrors.NewValidationError("tag name is required")
		}
		_, err = s.db.ExecContext(ctx,
			`UPDATE member_tags SET name = $1, updated_at = NOW() WHERE id = $2 AND merchant_id = $3 AND deleted_at IS NULL`,
			*req.Name, tagID, merchantID,
		)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to update tag name", err)
		}
	}
	if req.Color != nil {
		_, err = s.db.ExecContext(ctx,
			`UPDATE member_tags SET color = $1, updated_at = NOW() WHERE id = $2 AND merchant_id = $3 AND deleted_at IS NULL`,
			*req.Color, tagID, merchantID,
		)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to update tag color", err)
		}
	}
	if req.Description != nil {
		_, err = s.db.ExecContext(ctx,
			`UPDATE member_tags SET description = $1, updated_at = NOW() WHERE id = $2 AND merchant_id = $3 AND deleted_at IS NULL`,
			*req.Description, tagID, merchantID,
		)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to update tag description", err)
		}
	}

	return s.GetTag(ctx, merchantID, tagID)
}

// DeleteTag soft-deletes a tag.
func (s *Service) DeleteTag(ctx context.Context, merchantID, tagID int64) error {
	_, err := s.GetTag(ctx, merchantID, tagID)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE member_tags SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		tagID, merchantID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to delete tag", err)
	}
	return nil
}

// ToggleTag toggles a tag's active/inactive status.
func (s *Service) ToggleTag(ctx context.Context, merchantID, tagID int64) (*Tag, error) {
	t, err := s.GetTag(ctx, merchantID, tagID)
	if err != nil {
		return nil, err
	}
	newStatus := "inactive"
	if t.Status == "inactive" {
		newStatus = "active"
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE member_tags SET status = $1, updated_at = NOW() WHERE id = $2 AND merchant_id = $3 AND deleted_at IS NULL`,
		newStatus, tagID, merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to toggle tag status", err)
	}
	return s.GetTag(ctx, merchantID, tagID)
}

// --- Member Tag Relations ---

// AddTags adds tags to a member.
func (s *Service) AddTags(ctx context.Context, merchantID, memberID int64, tagIDs []int64) ([]MemberTag, error) {
	for _, tagID := range tagIDs {
		// Validate tag exists and belongs to merchant.
		_, err := s.GetTag(ctx, merchantID, tagID)
		if err != nil {
			return nil, err
		}
		// Insert relation (ignore duplicates via ON CONFLICT DO NOTHING).
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO member_tag_relations (merchant_id, member_id, tag_id)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (member_id, tag_id) DO NOTHING`,
			merchantID, memberID, tagID,
		)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to add tag to member", err)
		}
	}
	return s.GetMemberTags(ctx, merchantID, memberID)
}

// RemoveTag removes a tag from a member.
func (s *Service) RemoveTag(ctx context.Context, merchantID, memberID, tagID int64) error {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM member_tag_relations
		 WHERE merchant_id = $1 AND member_id = $2 AND tag_id = $3`,
		merchantID, memberID, tagID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to remove tag from member", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.NewNotFoundError("tag relation not found")
	}
	return nil
}

// GetMemberTags returns all tags for a member.
func (s *Service) GetMemberTags(ctx context.Context, merchantID, memberID int64) ([]MemberTag, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, mtr.member_id, t.name, t.color, mtr.created_at
		 FROM member_tag_relations mtr
		 JOIN member_tags t ON t.id = mtr.tag_id AND t.deleted_at IS NULL
		 WHERE mtr.merchant_id = $1 AND mtr.member_id = $2
		 ORDER BY mtr.created_at DESC`,
		merchantID, memberID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get member tags", err)
	}
	defer rows.Close()

	tags := make([]MemberTag, 0)
	for rows.Next() {
		var mt MemberTag
		if err := rows.Scan(&mt.TagID, &mt.MemberID, &mt.TagName, &mt.Color, &mt.CreatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan member tag", err)
		}
		tags = append(tags, mt)
	}
	return tags, nil
}

// GetMembersByTag returns member IDs that have a specific tag.
func (s *Service) GetMembersByTag(ctx context.Context, merchantID, tagID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT member_id FROM member_tag_relations
		 WHERE merchant_id = $1 AND tag_id = $2`,
		merchantID, tagID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get members by tag", err)
	}
	defer rows.Close()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, apperrors.NewInternalError("failed to scan member id", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// --- Auto-Tag Rules ---

// CreateRule creates an auto-tag rule.
func (s *Service) CreateRule(ctx context.Context, merchantID int64, req CreateRuleRequest) (*TagRule, error) {
	// Validate tag exists.
	_, err := s.GetTag(ctx, merchantID, req.TagID)
	if err != nil {
		return nil, err
	}
	validRuleTypes := map[string]bool{"total_consumption": true, "order_count": true, "pet_count": true, "last_visit_days": true}
	if !validRuleTypes[req.RuleType] {
		return nil, apperrors.NewValidationError("invalid rule_type: must be total_consumption, order_count, pet_count, or last_visit_days")
	}
	validOps := map[string]bool{"gte": true, "lte": true, "eq": true}
	if !validOps[req.Operator] {
		return nil, apperrors.NewValidationError("invalid operator: must be gte, lte, or eq")
	}

	r := &TagRule{}
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO member_tag_rules (merchant_id, tag_id, rule_type, operator, threshold_value)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, merchant_id, tag_id, rule_type, operator, threshold_value, status, created_at, updated_at`,
		merchantID, req.TagID, req.RuleType, req.Operator, req.ThresholdValue,
	).Scan(&r.ID, &r.MerchantID, &r.TagID, &r.RuleType, &r.Operator, &r.ThresholdValue, &r.Status, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create tag rule", err)
	}

	// Fill tag name.
	var tagName string
	s.db.QueryRowContext(ctx, `SELECT name FROM member_tags WHERE id = $1`, r.TagID).Scan(&tagName)
	r.TagName = tagName
	return r, nil
}

// ListRules returns all rules for a merchant.
func (s *Service) ListRules(ctx context.Context, merchantID int64) ([]TagRule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT r.id, r.merchant_id, r.tag_id, COALESCE(t.name, ''), r.rule_type, r.operator, r.threshold_value, r.status, r.created_at, r.updated_at
		 FROM member_tag_rules r
		 LEFT JOIN member_tags t ON t.id = r.tag_id
		 WHERE r.merchant_id = $1 AND r.deleted_at IS NULL
		 ORDER BY r.created_at DESC`,
		merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list tag rules", err)
	}
	defer rows.Close()

	rules := make([]TagRule, 0)
	for rows.Next() {
		var r TagRule
		if err := rows.Scan(&r.ID, &r.MerchantID, &r.TagID, &r.TagName, &r.RuleType, &r.Operator, &r.ThresholdValue, &r.Status, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan tag rule", err)
		}
		rules = append(rules, r)
	}
	return rules, nil
}

// GetRule returns a single rule.
func (s *Service) GetRule(ctx context.Context, merchantID, ruleID int64) (*TagRule, error) {
	r := &TagRule{}
	err := s.db.QueryRowContext(ctx,
		`SELECT r.id, r.merchant_id, r.tag_id, COALESCE(t.name, ''), r.rule_type, r.operator, r.threshold_value, r.status, r.created_at, r.updated_at
		 FROM member_tag_rules r
		 LEFT JOIN member_tags t ON t.id = r.tag_id
		 WHERE r.id = $1 AND r.merchant_id = $2 AND r.deleted_at IS NULL`,
		ruleID, merchantID,
	).Scan(&r.ID, &r.MerchantID, &r.TagID, &r.TagName, &r.RuleType, &r.Operator, &r.ThresholdValue, &r.Status, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("tag rule not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get tag rule", err)
	}
	return r, nil
}

// UpdateRule updates an auto-tag rule.
func (s *Service) UpdateRule(ctx context.Context, merchantID, ruleID int64, req UpdateRuleRequest) (*TagRule, error) {
	_, err := s.GetRule(ctx, merchantID, ruleID)
	if err != nil {
		return nil, err
	}

	if req.RuleType != nil {
		validRuleTypes := map[string]bool{"total_consumption": true, "order_count": true, "pet_count": true, "last_visit_days": true}
		if !validRuleTypes[*req.RuleType] {
			return nil, apperrors.NewValidationError("invalid rule_type")
		}
		_, err = s.db.ExecContext(ctx,
			`UPDATE member_tag_rules SET rule_type = $1, updated_at = NOW() WHERE id = $2 AND merchant_id = $3 AND deleted_at IS NULL`,
			*req.RuleType, ruleID, merchantID,
		)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to update rule", err)
		}
	}
	if req.Operator != nil {
		validOps := map[string]bool{"gte": true, "lte": true, "eq": true}
		if !validOps[*req.Operator] {
			return nil, apperrors.NewValidationError("invalid operator")
		}
		_, err = s.db.ExecContext(ctx,
			`UPDATE member_tag_rules SET operator = $1, updated_at = NOW() WHERE id = $2 AND merchant_id = $3 AND deleted_at IS NULL`,
			*req.Operator, ruleID, merchantID,
		)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to update rule", err)
		}
	}
	if req.ThresholdValue != nil {
		_, err = s.db.ExecContext(ctx,
			`UPDATE member_tag_rules SET threshold_value = $1, updated_at = NOW() WHERE id = $2 AND merchant_id = $3 AND deleted_at IS NULL`,
			*req.ThresholdValue, ruleID, merchantID,
		)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to update rule", err)
		}
	}

	return s.GetRule(ctx, merchantID, ruleID)
}

// DeleteRule soft-deletes a rule.
func (s *Service) DeleteRule(ctx context.Context, merchantID, ruleID int64) error {
	_, err := s.GetRule(ctx, merchantID, ruleID)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE member_tag_rules SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		ruleID, merchantID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to delete tag rule", err)
	}
	return nil
}

// ToggleRule toggles a rule's active/inactive status.
func (s *Service) ToggleRule(ctx context.Context, merchantID, ruleID int64) (*TagRule, error) {
	r, err := s.GetRule(ctx, merchantID, ruleID)
	if err != nil {
		return nil, err
	}
	newStatus := "inactive"
	if r.Status == "inactive" {
		newStatus = "active"
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE member_tag_rules SET status = $1, updated_at = NOW() WHERE id = $2 AND merchant_id = $3 AND deleted_at IS NULL`,
		newStatus, ruleID, merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to toggle rule status", err)
	}
	return s.GetRule(ctx, merchantID, ruleID)
}

// CheckAndApplyRules evaluates active auto-tag rules for a member and applies matching tags.
// Called after checkout or when member data changes.
func (s *Service) CheckAndApplyRules(ctx context.Context, merchantID, memberID int64) ([]MemberTag, error) {
	// Get all active rules for the merchant.
	rows, err := s.db.QueryContext(ctx,
		`SELECT r.id, r.rule_type, r.operator, r.threshold_value, r.tag_id
		 FROM member_tag_rules r
		 JOIN member_tags t ON t.id = r.tag_id AND t.status = 'active' AND t.deleted_at IS NULL
		 WHERE r.merchant_id = $1 AND r.status = 'active' AND r.deleted_at IS NULL`,
		merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load tag rules", err)
	}
	defer rows.Close()

	type ruleDef struct {
		id             int64
		ruleType       string
		operator       string
		thresholdValue float64
		tagID          int64
	}
	var rules []ruleDef
	for rows.Next() {
		var r ruleDef
		if err := rows.Scan(&r.id, &r.ruleType, &r.operator, &r.thresholdValue, &r.tagID); err != nil {
			return nil, apperrors.NewInternalError("failed to scan rule", err)
		}
		rules = append(rules, r)
	}
	if len(rules) == 0 {
		return s.GetMemberTags(ctx, merchantID, memberID)
	}

	// Gather member metrics.
	metrics := s.gatherMetrics(ctx, merchantID, memberID)

	// Check each rule and auto-tag if matched.
	for _, rule := range rules {
		val, ok := metrics[rule.ruleType]
		if !ok {
			continue
		}
		matched := false
		switch rule.operator {
		case "gte":
			matched = val >= rule.thresholdValue
		case "lte":
			matched = val <= rule.thresholdValue
		case "eq":
			matched = val == rule.thresholdValue
		}
		if matched {
			// Add tag (ON CONFLICT DO NOTHING for idempotency).
			_, _ = s.db.ExecContext(ctx,
				`INSERT INTO member_tag_relations (merchant_id, member_id, tag_id)
				 VALUES ($1, $2, $3)
				 ON CONFLICT (member_id, tag_id) DO NOTHING`,
				merchantID, memberID, rule.tagID,
			)
		}
	}

	return s.GetMemberTags(ctx, merchantID, memberID)
}

// gatherMetrics collects all relevant member metrics for rule evaluation.
func (s *Service) gatherMetrics(ctx context.Context, merchantID, memberID int64) map[string]float64 {
	metrics := make(map[string]float64)

	// total_consumption: sum of paid_cents from orders.
	var totalConsumption float64
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(paid_cents), 0) FROM orders
		 WHERE merchant_id = $1 AND member_id = $2 AND status NOT IN ('voided', 'refunded')`,
		merchantID, memberID,
	).Scan(&totalConsumption)
	if err == nil {
		// Convert cents to yuan for threshold comparison.
		metrics["total_consumption"] = totalConsumption / 100.0
	}

	// order_count: count of orders.
	var orderCount float64
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM orders
		 WHERE merchant_id = $1 AND member_id = $2 AND status NOT IN ('voided', 'refunded')`,
		merchantID, memberID,
	).Scan(&orderCount)
	if err == nil {
		metrics["order_count"] = orderCount
	}

	// pet_count: number of pets owned.
	var petCount float64
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pets
		 WHERE merchant_id = $1 AND member_id = $2 AND deleted_at IS NULL`,
		merchantID, memberID,
	).Scan(&petCount)
	if err == nil {
		metrics["pet_count"] = petCount
	}

	// last_visit_days: days since last order.
	var lastVisitDays float64
	var lastOrderTime sql.NullTime
	err = s.db.QueryRowContext(ctx,
		`SELECT MAX(created_at) FROM orders
		 WHERE merchant_id = $1 AND member_id = $2`,
		merchantID, memberID,
	).Scan(&lastOrderTime)
	if err == nil && lastOrderTime.Valid {
		lastVisitDays = time.Since(lastOrderTime.Time).Hours() / 24.0
	}
	metrics["last_visit_days"] = lastVisitDays

	return metrics
}

// TagMemberCounts returns the count of members per tag.
type TagWithCount struct {
	Tag
	MemberCount int `json:"member_count"`
}

// ListTagsWithCount returns all tags with member counts.
func (s *Service) ListTagsWithCount(ctx context.Context, merchantID int64) ([]TagWithCount, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.merchant_id, t.name, t.color, t.description, t.status, t.created_at, t.updated_at,
		        COUNT(mtr.id) AS member_count
		 FROM member_tags t
		 LEFT JOIN member_tag_relations mtr ON mtr.tag_id = t.id
		 WHERE t.merchant_id = $1 AND t.deleted_at IS NULL
		 GROUP BY t.id
		 ORDER BY t.created_at DESC`,
		merchantID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags with count: %w", err)
	}
	defer rows.Close()

	result := make([]TagWithCount, 0)
	for rows.Next() {
		var tc TagWithCount
		if err := rows.Scan(&tc.ID, &tc.MerchantID, &tc.Name, &tc.Color, &tc.Description, &tc.Status, &tc.CreatedAt, &tc.UpdatedAt, &tc.MemberCount); err != nil {
			return nil, apperrors.NewInternalError("failed to scan tag with count", err)
		}
		result = append(result, tc)
	}
	return result, nil
}
