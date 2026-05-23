package memberlevel

import (
	"context"
	"database/sql"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// LevelRule represents a member level tier rule.
type LevelRule struct {
	ID               int64      `json:"id"`
	MerchantID       int64      `json:"merchant_id"`
	Name             string     `json:"name"`
	LevelOrder       int        `json:"level_order"`
	UpgradeType      string     `json:"upgrade_type"`
	UpgradeValue     int64      `json:"upgrade_value"`
	DiscountPercent  int        `json:"discount_percent"`
	PointsMultiplier int        `json:"points_multiplier"`
	DowngradeDays    int        `json:"downgrade_days"`
	Icon             string     `json:"icon"`
	Color            string     `json:"color"`
	Description      string     `json:"description"`
	Status           string     `json:"status"`
	IsDefault        bool       `json:"is_default"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

// CreateLevelRuleRequest is the request body for creating a level rule.
type CreateLevelRuleRequest struct {
	Name             string `json:"name"`
	LevelOrder       int    `json:"level_order"`
	UpgradeType      string `json:"upgrade_type"`
	UpgradeValue     int64  `json:"upgrade_value"`
	DiscountPercent  int    `json:"discount_percent"`
	PointsMultiplier int    `json:"points_multiplier"`
	DowngradeDays    int    `json:"downgrade_days"`
	Icon             string `json:"icon"`
	Color            string `json:"color"`
	Description      string `json:"description"`
	IsDefault        bool   `json:"is_default"`
}

// UpdateLevelRuleRequest is the request body for updating a level rule.
type UpdateLevelRuleRequest struct {
	Name             *string `json:"name,omitempty"`
	LevelOrder       *int    `json:"level_order,omitempty"`
	UpgradeType      *string `json:"upgrade_type,omitempty"`
	UpgradeValue     *int64  `json:"upgrade_value,omitempty"`
	DiscountPercent  *int    `json:"discount_percent,omitempty"`
	PointsMultiplier *int    `json:"points_multiplier,omitempty"`
	DowngradeDays    *int    `json:"downgrade_days,omitempty"`
	Icon             *string `json:"icon,omitempty"`
	Color            *string `json:"color,omitempty"`
	Description      *string `json:"description,omitempty"`
	IsDefault        *bool   `json:"is_default,omitempty"`
}

// LevelLog represents a level change history record.
type LevelLog struct {
	ID           int64     `json:"id"`
	MemberID     int64     `json:"member_id"`
	MerchantID   int64     `json:"merchant_id"`
	OldLevelID   *int64    `json:"old_level_id,omitempty"`
	OldLevelName string    `json:"old_level_name,omitempty"`
	NewLevelID   int64     `json:"new_level_id"`
	NewLevelName string    `json:"new_level_name"`
	ChangeType   string    `json:"change_type"`
	ChangeReason string    `json:"change_reason"`
	CreatedAt    time.Time `json:"created_at"`
}

// MemberLevelInfo holds a member's current level information.
type MemberLevelInfo struct {
	LevelID        int64  `json:"level_id"`
	Name           string `json:"name"`
	LevelOrder     int    `json:"level_order"`
	DiscountPercent  int  `json:"discount_percent"`
	PointsMultiplier int  `json:"points_multiplier"`
	Icon           string `json:"icon"`
	Color          string `json:"color"`
}

// Service provides member level operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new member level Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// ----------------------- Level Rule CRUD -----------------------

// CreateRule creates a new member level rule.
func (s *Service) CreateRule(ctx context.Context, merchantID int64, req CreateLevelRuleRequest) (*LevelRule, error) {
	if req.Name == "" {
		return nil, apperrors.NewValidationError("level name is required")
	}
	if req.UpgradeType != "total_spending" && req.UpgradeType != "total_recharge" && req.UpgradeType != "order_count" {
		return nil, apperrors.NewValidationError("upgrade_type must be total_spending, total_recharge, or order_count")
	}
	if req.UpgradeValue < 0 {
		return nil, apperrors.NewValidationError("upgrade_value must be non-negative")
	}
	if req.DiscountPercent < 1 || req.DiscountPercent > 100 {
		return nil, apperrors.NewValidationError("discount_percent must be between 1 and 100")
	}
	if req.PointsMultiplier < 0 {
		return nil, apperrors.NewValidationError("points_multiplier must be non-negative")
	}
	if req.DowngradeDays < 0 {
		return nil, apperrors.NewValidationError("downgrade_days must be non-negative")
	}

	// If this is set as default, clear existing default.
	if req.IsDefault {
		_, _ = s.db.ExecContext(ctx,
			`UPDATE member_level_rules SET is_default = false, updated_at = NOW()
			 WHERE merchant_id = $1 AND deleted_at IS NULL`,
			merchantID,
		)
	}

	var rule LevelRule
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO member_level_rules (merchant_id, name, level_order, upgrade_type, upgrade_value,
		 discount_percent, points_multiplier, downgrade_days, icon, color, description, status, is_default)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'active',$12)
		 RETURNING id, merchant_id, name, level_order, upgrade_type, upgrade_value, discount_percent,
		           points_multiplier, downgrade_days, icon, color, description, status, is_default,
		           created_at, updated_at`,
		merchantID, req.Name, req.LevelOrder, req.UpgradeType, req.UpgradeValue,
		req.DiscountPercent, req.PointsMultiplier, req.DowngradeDays, req.Icon, req.Color,
		req.Description, req.IsDefault,
	).Scan(&rule.ID, &rule.MerchantID, &rule.Name, &rule.LevelOrder, &rule.UpgradeType,
		&rule.UpgradeValue, &rule.DiscountPercent, &rule.PointsMultiplier, &rule.DowngradeDays,
		&rule.Icon, &rule.Color, &rule.Description, &rule.Status, &rule.IsDefault,
		&rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create level rule", err)
	}
	return &rule, nil
}

// ListRules returns all level rules for a merchant, ordered by level_order DESC.
func (s *Service) ListRules(ctx context.Context, merchantID int64) ([]LevelRule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, merchant_id, name, level_order, upgrade_type, upgrade_value,
		        discount_percent, points_multiplier, downgrade_days, icon, color, description,
		        status, is_default, created_at, updated_at
		 FROM member_level_rules
		 WHERE merchant_id = $1 AND deleted_at IS NULL
		 ORDER BY level_order DESC`,
		merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list level rules", err)
	}
	defer rows.Close()

	var rules []LevelRule
	for rows.Next() {
		var r LevelRule
		if err := rows.Scan(&r.ID, &r.MerchantID, &r.Name, &r.LevelOrder, &r.UpgradeType,
			&r.UpgradeValue, &r.DiscountPercent, &r.PointsMultiplier, &r.DowngradeDays,
			&r.Icon, &r.Color, &r.Description, &r.Status, &r.IsDefault,
			&r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan level rule", err)
		}
		rules = append(rules, r)
	}
	if rules == nil {
		rules = []LevelRule{}
	}
	return rules, nil
}

// GetRule returns a single level rule by ID.
func (s *Service) GetRule(ctx context.Context, merchantID, id int64) (*LevelRule, error) {
	var r LevelRule
	err := s.db.QueryRowContext(ctx,
		`SELECT id, merchant_id, name, level_order, upgrade_type, upgrade_value,
		        discount_percent, points_multiplier, downgrade_days, icon, color, description,
		        status, is_default, created_at, updated_at
		 FROM member_level_rules
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		id, merchantID,
	).Scan(&r.ID, &r.MerchantID, &r.Name, &r.LevelOrder, &r.UpgradeType,
		&r.UpgradeValue, &r.DiscountPercent, &r.PointsMultiplier, &r.DowngradeDays,
		&r.Icon, &r.Color, &r.Description, &r.Status, &r.IsDefault,
		&r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("level rule not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get level rule", err)
	}
	return &r, nil
}

// UpdateRule partially updates a level rule.
func (s *Service) UpdateRule(ctx context.Context, merchantID, id int64, req UpdateLevelRuleRequest) (*LevelRule, error) {
	// Verify ownership.
	if err := s.checkRuleOwnership(ctx, merchantID, id); err != nil {
		return nil, err
	}

	// If setting as default, clear existing default.
	if req.IsDefault != nil && *req.IsDefault {
		_, _ = s.db.ExecContext(ctx,
			`UPDATE member_level_rules SET is_default = false, updated_at = NOW()
			 WHERE merchant_id = $1 AND id != $2 AND deleted_at IS NULL`,
			merchantID, id,
		)
	}

	// Fetch current to merge.
	current, err := s.GetRule(ctx, merchantID, id)
	if err != nil {
		return nil, err
	}

	name := current.Name
	levelOrder := current.LevelOrder
	upgradeType := current.UpgradeType
	upgradeValue := current.UpgradeValue
	discountPercent := current.DiscountPercent
	pointsMultiplier := current.PointsMultiplier
	downgradeDays := current.DowngradeDays
	icon := current.Icon
	color := current.Color
	description := current.Description
	isDefault := current.IsDefault

	if req.Name != nil {
		name = *req.Name
	}
	if req.LevelOrder != nil {
		levelOrder = *req.LevelOrder
	}
	if req.UpgradeType != nil {
		upgradeType = *req.UpgradeType
	}
	if req.UpgradeValue != nil {
		upgradeValue = *req.UpgradeValue
	}
	if req.DiscountPercent != nil {
		discountPercent = *req.DiscountPercent
	}
	if req.PointsMultiplier != nil {
		pointsMultiplier = *req.PointsMultiplier
	}
	if req.DowngradeDays != nil {
		downgradeDays = *req.DowngradeDays
	}
	if req.Icon != nil {
		icon = *req.Icon
	}
	if req.Color != nil {
		color = *req.Color
	}
	if req.Description != nil {
		description = *req.Description
	}
	if req.IsDefault != nil {
		isDefault = *req.IsDefault
	}

	var r LevelRule
	err = s.db.QueryRowContext(ctx,
		`UPDATE member_level_rules SET
		 name=$1, level_order=$2, upgrade_type=$3, upgrade_value=$4,
		 discount_percent=$5, points_multiplier=$6, downgrade_days=$7,
		 icon=$8, color=$9, description=$10, is_default=$11, updated_at=NOW()
		 WHERE id=$12 AND merchant_id=$13 AND deleted_at IS NULL
		 RETURNING id, merchant_id, name, level_order, upgrade_type, upgrade_value,
		           discount_percent, points_multiplier, downgrade_days, icon, color, description,
		           status, is_default, created_at, updated_at`,
		name, levelOrder, upgradeType, upgradeValue,
		discountPercent, pointsMultiplier, downgradeDays,
		icon, color, description, isDefault,
		id, merchantID,
	).Scan(&r.ID, &r.MerchantID, &r.Name, &r.LevelOrder, &r.UpgradeType,
		&r.UpgradeValue, &r.DiscountPercent, &r.PointsMultiplier, &r.DowngradeDays,
		&r.Icon, &r.Color, &r.Description, &r.Status, &r.IsDefault,
		&r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update level rule", err)
	}
	return &r, nil
}

// DeleteRule soft-deletes a level rule. Members currently at this level keep their level_id.
func (s *Service) DeleteRule(ctx context.Context, merchantID, id int64) error {
	if err := s.checkRuleOwnership(ctx, merchantID, id); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE member_level_rules SET deleted_at = NOW(), updated_at = NOW()
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		id, merchantID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to delete level rule", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return apperrors.NewNotFoundError("level rule not found")
	}
	return nil
}

// ToggleRuleStatus toggles a level rule's status between active/inactive.
func (s *Service) ToggleRuleStatus(ctx context.Context, merchantID, id int64) (*LevelRule, error) {
	if err := s.checkRuleOwnership(ctx, merchantID, id); err != nil {
		return nil, err
	}
	var r LevelRule
	err := s.db.QueryRowContext(ctx,
		`UPDATE member_level_rules SET
		 status = CASE WHEN status = 'active' THEN 'inactive' ELSE 'active' END,
		 updated_at = NOW()
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL
		 RETURNING id, merchant_id, name, level_order, upgrade_type, upgrade_value,
		           discount_percent, points_multiplier, downgrade_days, icon, color, description,
		           status, is_default, created_at, updated_at`,
		id, merchantID,
	).Scan(&r.ID, &r.MerchantID, &r.Name, &r.LevelOrder, &r.UpgradeType,
		&r.UpgradeValue, &r.DiscountPercent, &r.PointsMultiplier, &r.DowngradeDays,
		&r.Icon, &r.Color, &r.Description, &r.Status, &r.IsDefault,
		&r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to toggle level rule", err)
	}
	return &r, nil
}

// ----------------------- Member Level Operations -----------------------

// GetMemberLevel returns a member's current level information.
func (s *Service) GetMemberLevel(ctx context.Context, merchantID, memberID int64) (*MemberLevelInfo, error) {
	var info MemberLevelInfo
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(mlr.id, 0), COALESCE(mlr.name, ''), COALESCE(mlr.level_order, 0),
		        COALESCE(mlr.discount_percent, 100), COALESCE(mlr.points_multiplier, 100),
		        COALESCE(mlr.icon, ''), COALESCE(mlr.color, '')
		 FROM members m
		 LEFT JOIN member_level_rules mlr ON mlr.id = m.level_id AND mlr.deleted_at IS NULL
		 WHERE m.id = $1 AND m.merchant_id = $2 AND m.deleted_at IS NULL`,
		memberID, merchantID,
	).Scan(&info.LevelID, &info.Name, &info.LevelOrder, &info.DiscountPercent,
		&info.PointsMultiplier, &info.Icon, &info.Color)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("member not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get member level", err)
	}
	return &info, nil
}

// CheckAndUpgrade checks if a member qualifies for a higher level after a purchase,
// and performs the upgrade if so. Returns the log if upgrade happened, nil if not.
func (s *Service) CheckAndUpgrade(ctx context.Context, merchantID, memberID int64) (*LevelLog, error) {
	// Get member's total stats.
	stats, err := s.getMemberStats(ctx, merchantID, memberID)
	if err != nil {
		return nil, err
	}

	// Find the highest active rule (by level_order) that the member qualifies for.
	var newRule *LevelRule
	rules, err := s.ListRules(ctx, merchantID)
	if err != nil {
		return nil, err
	}

	var currentLevelID int64
	_ = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(level_id, 0) FROM members WHERE id = $1 AND deleted_at IS NULL`,
		memberID,
	).Scan(&currentLevelID)

	for _, rule := range rules {
		if rule.Status != "active" {
			continue
		}
		if s.meetsThreshold(stats, rule) {
			if rule.LevelOrder > 0 && (newRule == nil || rule.LevelOrder > newRule.LevelOrder) {
				newRule = &rule
				break // Rules are already ordered DESC by level_order, so the first match is the highest.
			}
		}
	}

	if newRule == nil || newRule.ID == currentLevelID {
		return nil, nil // No upgrade needed or already at this level.
	}

	// Check we're not downgrading (only upgrade here).
	if newRule.LevelOrder <= 0 {
		return nil, nil
	}

	// Get current level order.
	var currentLevelOrder int
	if currentLevelID > 0 {
		for _, rule := range rules {
			if rule.ID == currentLevelID {
				currentLevelOrder = rule.LevelOrder
				break
			}
		}
	}

	if newRule.LevelOrder <= currentLevelOrder {
		return nil, nil
	}

	// Get previous level name.
	var oldLevelName string
	var oldLevelID *int64
	if currentLevelID > 0 {
		oldLevelID = &currentLevelID
		_ = s.db.QueryRowContext(ctx,
			`SELECT name FROM member_level_rules WHERE id = $1`, currentLevelID,
		).Scan(&oldLevelName)
	}

	// Perform upgrade in transaction.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`UPDATE members SET level_id = $1, updated_at = NOW() WHERE id = $2`,
		newRule.ID, memberID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update member level", err)
	}

	reason := "accumulated " + newRule.UpgradeType + " reached " + formatUpgradeValue(newRule.UpgradeType, newRule.UpgradeValue) + ", upgraded to " + newRule.Name

	var log LevelLog
	err = tx.QueryRowContext(ctx,
		`INSERT INTO member_level_logs (member_id, merchant_id, old_level_id, new_level_id, change_type, change_reason)
		 VALUES ($1, $2, $3, $4, 'upgrade', $5)
		 RETURNING id, member_id, merchant_id, old_level_id, new_level_id, change_type, change_reason, created_at`,
		memberID, merchantID, oldLevelID, newRule.ID, reason,
	).Scan(&log.ID, &log.MemberID, &log.MerchantID, &log.OldLevelID, &log.NewLevelID,
		&log.ChangeType, &log.ChangeReason, &log.CreatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create level log", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit upgrade", err)
	}

	log.OldLevelName = oldLevelName
	log.NewLevelName = newRule.Name
	return &log, nil
}

// CheckAndDowngrade checks if a member should be downgraded due to inactivity.
func (s *Service) CheckAndDowngrade(ctx context.Context, merchantID, memberID int64) (*LevelLog, error) {
	var currentLevelID int64
	var lastOrderAt sql.NullTime
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(m.level_id, 0), MAX(o.created_at)
		 FROM members m
		 LEFT JOIN orders o ON o.member_id = m.id
		 WHERE m.id = $1 AND m.merchant_id = $2 AND m.deleted_at IS NULL
		 GROUP BY m.id`,
		memberID, merchantID,
	).Scan(&currentLevelID, &lastOrderAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("member not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to check member status", err)
	}

	if currentLevelID <= 0 {
		return nil, nil // No level to downgrade from.
	}

	// Get current rule.
	currentRule, err := s.GetRule(ctx, merchantID, currentLevelID)
	if err != nil {
		return nil, err
	}

	// Check if inactive period exceeds downgrade_days.
	if currentRule.DowngradeDays <= 0 {
		return nil, nil // No auto-downgrade configured.
	}

	if lastOrderAt.Valid {
		daysSinceLastOrder := int(time.Since(lastOrderAt.Time).Hours() / 24)
		if daysSinceLastOrder < currentRule.DowngradeDays {
			return nil, nil // Within grace period.
		}
	}

	// Find the next lower active level.
	rules, err := s.ListRules(ctx, merchantID)
	if err != nil {
		return nil, err
	}

	var lowerRule *LevelRule
	for _, rule := range rules {
		if rule.Status != "active" || rule.ID == currentLevelID {
			continue
		}
		if rule.LevelOrder < currentRule.LevelOrder {
			if lowerRule == nil || rule.LevelOrder > lowerRule.LevelOrder {
				lowerRule = &rule
			}
		}
	}

	if lowerRule == nil {
		return nil, nil // No lower level to downgrade to.
	}

	// Perform downgrade.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`UPDATE members SET level_id = $1, updated_at = NOW() WHERE id = $2`,
		lowerRule.ID, memberID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update member level", err)
	}

	reason := "no purchase for over " + formatDays(currentRule.DowngradeDays) + ", downgraded from " + currentRule.Name + " to " + lowerRule.Name

	var log LevelLog
	err = tx.QueryRowContext(ctx,
		`INSERT INTO member_level_logs (member_id, merchant_id, old_level_id, new_level_id, change_type, change_reason)
		 VALUES ($1, $2, $3, $4, 'downgrade', $5)
		 RETURNING id, member_id, merchant_id, old_level_id, new_level_id, change_type, change_reason, created_at`,
		memberID, merchantID, currentLevelID, lowerRule.ID, reason,
	).Scan(&log.ID, &log.MemberID, &log.MerchantID, &log.OldLevelID, &log.NewLevelID,
		&log.ChangeType, &log.ChangeReason, &log.CreatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create level log", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit downgrade", err)
	}

	log.OldLevelName = currentRule.Name
	log.NewLevelName = lowerRule.Name
	return &log, nil
}

// SetDefaultLevel assigns the default level to a new member.
func (s *Service) SetDefaultLevel(ctx context.Context, merchantID, memberID int64) error {
	var defaultID int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM member_level_rules
		 WHERE merchant_id = $1 AND is_default = true AND status = 'active' AND deleted_at IS NULL
		 LIMIT 1`,
		merchantID,
	).Scan(&defaultID)
	if err == sql.ErrNoRows {
		return nil // No default level configured.
	}
	if err != nil {
		return apperrors.NewInternalError("failed to find default level", err)
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE members SET level_id = $1, updated_at = NOW() WHERE id = $2`,
		defaultID, memberID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to set default level", err)
	}

	// Log it.
	var ruleName string
	_ = s.db.QueryRowContext(ctx,
		`SELECT name FROM member_level_rules WHERE id = $1`, defaultID,
	).Scan(&ruleName)

	_, _ = s.db.ExecContext(ctx,
		`INSERT INTO member_level_logs (member_id, merchant_id, old_level_id, new_level_id, change_type, change_reason)
		 VALUES ($1, $2, NULL, $3, 'set_default', $4)`,
		memberID, merchantID, defaultID, "assigned default level: "+ruleName,
	)
	return nil
}

// GetLevelLogs returns the level change history for a member.
func (s *Service) GetLevelLogs(ctx context.Context, merchantID, memberID int64) ([]LevelLog, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT mll.id, mll.member_id, mll.merchant_id, mll.old_level_id,
		        COALESCE(old_r.name, ''), mll.new_level_id, COALESCE(new_r.name, ''),
		        mll.change_type, mll.change_reason, mll.created_at
		 FROM member_level_logs mll
		 JOIN member_level_rules new_r ON new_r.id = mll.new_level_id
		 LEFT JOIN member_level_rules old_r ON old_r.id = mll.old_level_id
		 WHERE mll.member_id = $1 AND mll.merchant_id = $2
		 ORDER BY mll.created_at DESC`,
		memberID, merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query level logs", err)
	}
	defer rows.Close()

	var logs []LevelLog
	for rows.Next() {
		var l LevelLog
		if err := rows.Scan(&l.ID, &l.MemberID, &l.MerchantID, &l.OldLevelID,
			&l.OldLevelName, &l.NewLevelID, &l.NewLevelName,
			&l.ChangeType, &l.ChangeReason, &l.CreatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan level log", err)
		}
		logs = append(logs, l)
	}
	if logs == nil {
		logs = []LevelLog{}
	}
	return logs, nil
}

// ----------------------- Helper types & functions -----------------------

type memberStats struct {
	TotalSpending int
	TotalRecharge int
	OrderCount    int
}

func (s *Service) getMemberStats(ctx context.Context, merchantID, memberID int64) (*memberStats, error) {
	stats := &memberStats{}

	// Total spending from completed/refunded orders.
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(total_cents), 0) FROM orders
		 WHERE member_id = $1 AND merchant_id = $2 AND status IN ('completed', 'refunded')`,
		memberID, merchantID,
	).Scan(&stats.TotalSpending)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query member spending", err)
	}

	// Order count.
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM orders
		 WHERE member_id = $1 AND merchant_id = $2 AND status IN ('completed', 'refunded')`,
		memberID, merchantID,
	).Scan(&stats.OrderCount)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query member order count", err)
	}

	return stats, nil
}

func (s *Service) meetsThreshold(stats *memberStats, rule LevelRule) bool {
	switch rule.UpgradeType {
	case "total_spending":
		return int64(stats.TotalSpending) >= rule.UpgradeValue
	case "order_count":
		return int64(stats.OrderCount) >= rule.UpgradeValue
	default:
		return false
	}
}

func (s *Service) checkRuleOwnership(ctx context.Context, merchantID, id int64) error {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM member_level_rules
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		id, merchantID,
	).Scan(&count)
	if err != nil {
		return apperrors.NewInternalError("failed to check rule ownership", err)
	}
	if count == 0 {
		return apperrors.NewNotFoundError("level rule not found")
	}
	return nil
}

// GetApplicableDiscount returns the discount percent and points multiplier
// for the member's current level. Used by checkout.
func (s *Service) GetApplicableDiscount(ctx context.Context, merchantID, memberID int64) (discountPercent int, pointsMultiplier int) {
	var levelID sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT level_id FROM members WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		memberID, merchantID,
	).Scan(&levelID)
	if err != nil || !levelID.Valid || levelID.Int64 == 0 {
		return 100, 100 // Default: no discount, 1x points.
	}

	err = s.db.QueryRowContext(ctx,
		`SELECT discount_percent, points_multiplier FROM member_level_rules
		 WHERE id = $1 AND status = 'active' AND deleted_at IS NULL`,
		levelID.Int64,
	).Scan(&discountPercent, &pointsMultiplier)
	if err != nil {
		return 100, 100
	}
	return discountPercent, pointsMultiplier
}

func formatUpgradeValue(upgradeType string, value int64) string {
	switch upgradeType {
	case "total_spending":
		return formatCents(value)
	case "order_count":
		return formatCount(value, "orders")
	default:
		return formatCount(value, "")
	}
}

func formatCents(cents int64) string {
	yuan := float64(cents) / 100.0
	return formatFloat(yuan) + " yuan"
}

func formatCount(n int64, unit string) string {
	s := formatFloat(float64(n))
	if unit != "" {
		s += " " + unit
	}
	return s
}

func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return formatInt(int64(f))
	}
	return formatDecimal(f)
}

func formatInt(n int64) string {
	if n < 0 {
		return "-" + formatInt(-n)
	}
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

func formatDecimal(f float64) string {
	intPart := int64(f)
	frac := int64((f - float64(intPart)) * 100)
	if frac < 0 {
		frac = -frac
	}
	return formatInt(intPart) + "." + padLeft(formatInt(frac), 2)
}

func padLeft(s string, n int) string {
	for len(s) < n {
		s = "0" + s
	}
	return s
}

func formatDays(days int) string {
	return formatInt(int64(days)) + " days"
}
