package servicecard

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// ServiceCardTemplate represents a service card product definition.
type ServiceCardTemplate struct {
	ID            int64  `json:"id"`
	MerchantID    int64  `json:"merchant_id"`
	Name          string `json:"name"`
	ServiceItemID *int64 `json:"service_item_id,omitempty"`
	TotalUses     int    `json:"total_uses"`
	PriceCents    int64  `json:"price_cents"`
	ValidityDays  int    `json:"validity_days"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// CreateTemplateRequest is the request to create a service card template.
type CreateTemplateRequest struct {
	Name          string `json:"name"`
	ServiceItemID *int64 `json:"service_item_id,omitempty"`
	TotalUses     int    `json:"total_uses"`
	PriceCents    int64  `json:"price_cents"`
	ValidityDays  int    `json:"validity_days"`
}

// UpdateTemplateRequest is the request to update a service card template.
type UpdateTemplateRequest struct {
	Name          *string `json:"name,omitempty"`
	ServiceItemID *int64  `json:"service_item_id,omitempty"`
	TotalUses     *int    `json:"total_uses,omitempty"`
	PriceCents    *int64  `json:"price_cents,omitempty"`
	ValidityDays  *int    `json:"validity_days,omitempty"`
}

// ServiceCard represents an individual service card held by a member.
type ServiceCard struct {
	ID             int64   `json:"id"`
	MerchantID     int64   `json:"merchant_id"`
	MemberID       *int64  `json:"member_id,omitempty"`
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	ServiceItemID  *int64  `json:"service_item_id,omitempty"`
	ServiceName    string  `json:"service_name,omitempty"`
	TotalUses      int     `json:"total_uses"`
	UsedCount      int     `json:"used_count"`
	RemainingUses  int     `json:"remaining_uses"`
	Status         string  `json:"status"`
	ValidFrom      *string `json:"valid_from,omitempty"`
	ValidUntil     *string `json:"valid_until,omitempty"`
	MemberName     string  `json:"member_name,omitempty"`
	CreatedAt      string  `json:"created_at"`
}

// PurchaseRequest is the request to purchase a service card for a member.
type PurchaseRequest struct {
	MemberID    int64 `json:"member_id"`
	PaymentCents int64 `json:"payment_cents"`
}

// UsageLog represents a usage record for a service card.
type UsageLog struct {
	ID             int64  `json:"id"`
	MerchantID     int64  `json:"merchant_id"`
	ServiceCardID  int64  `json:"service_card_id"`
	Code           string `json:"code"`
	Name           string `json:"name"`
	VerificationType string `json:"verification_type"`
	Result         string `json:"result"`
	Detail         string `json:"detail"`
	OrderID        *int64 `json:"order_id,omitempty"`
	VerifiedAt     string `json:"verified_at"`
}

// ListParams holds filters for listing templates.
type ListParams struct {
	Status   string
	Keyword  string
	Page     int
	PageSize int
}

// TemplateListResult wraps template list with pagination.
type TemplateListResult struct {
	Templates []ServiceCardTemplate `json:"templates"`
	Total     int                   `json:"total"`
	Page      int                   `json:"page"`
	PageSize  int                   `json:"page_size"`
}

// CardListResult wraps card list with pagination.
type CardListResult struct {
	Cards    []ServiceCard `json:"cards"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

// UsageLogListResult wraps usage log list with pagination.
type UsageLogListResult struct {
	Logs     []UsageLog `json:"logs"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}

// ExpiringCardsResult wraps expiring cards with count.
type ExpiringCardsResult struct {
	Cards []ServiceCard `json:"cards"`
	Total int           `json:"total"`
}

// Service provides service card management operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new service card Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func generateCode() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "SC" + strings.ToUpper(hex.EncodeToString(b))
}

// --- Template CRUD ---

// CreateTemplate creates a new service card template.
func (s *Service) CreateTemplate(ctx context.Context, merchantID int64, req CreateTemplateRequest) (*ServiceCardTemplate, error) {
	if req.Name == "" {
		return nil, apperrors.NewValidationError("template name is required")
	}
	if req.TotalUses <= 0 {
		return nil, apperrors.NewValidationError("total_uses must be greater than 0")
	}
	if req.PriceCents <= 0 {
		return nil, apperrors.NewValidationError("price_cents must be greater than 0")
	}
	if req.ValidityDays <= 0 {
		req.ValidityDays = 365
	}

	var t ServiceCardTemplate
	var createdAt, updatedAt time.Time
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO service_card_templates (merchant_id, name, service_item_id, total_uses, price_cents, validity_days, status)
		 VALUES ($1, $2, $3, $4, $5, $6, 'active')
		 RETURNING id, merchant_id, name, service_item_id, total_uses, price_cents, validity_days, status, created_at, updated_at`,
		merchantID, req.Name, req.ServiceItemID, req.TotalUses, req.PriceCents, req.ValidityDays,
	).Scan(&t.ID, &t.MerchantID, &t.Name, &t.ServiceItemID, &t.TotalUses, &t.PriceCents,
		&t.ValidityDays, &t.Status, &createdAt, &updatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create template", err)
	}
	t.CreatedAt = createdAt.Format(time.RFC3339)
	t.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &t, nil
}

// ListTemplates lists service card templates with filtering and pagination.
func (s *Service) ListTemplates(ctx context.Context, merchantID int64, params ListParams) (*TemplateListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}

	var conditions []string
	var args []interface{}
	conditions = append(conditions, "merchant_id = $1 AND deleted_at IS NULL")
	args = append(args, merchantID)
	argIdx := 2

	if params.Status != "" {
		conditions = append(conditions, "status = $"+strconv.Itoa(argIdx))
		args = append(args, params.Status)
		argIdx++
	}
	if params.Keyword != "" {
		conditions = append(conditions, "name ILIKE $"+strconv.Itoa(argIdx))
		args = append(args, "%"+params.Keyword+"%")
		argIdx++
	}

	whereClause := strings.Join(conditions, " AND ")

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM service_card_templates WHERE `+whereClause, args...,
	).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count templates", err)
	}

	offset := (params.Page - 1) * params.PageSize
	queryArgs := append(args, params.PageSize, offset)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, merchant_id, name, service_item_id, total_uses, price_cents, validity_days, status, created_at, updated_at
		 FROM service_card_templates
		 WHERE `+whereClause+
			` ORDER BY created_at DESC LIMIT $`+strconv.Itoa(argIdx)+` OFFSET $`+strconv.Itoa(argIdx+1),
		queryArgs...,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query templates", err)
	}
	defer rows.Close()

	var templates []ServiceCardTemplate
	for rows.Next() {
		var t ServiceCardTemplate
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&t.ID, &t.MerchantID, &t.Name, &t.ServiceItemID, &t.TotalUses, &t.PriceCents,
			&t.ValidityDays, &t.Status, &createdAt, &updatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan template", err)
		}
		t.CreatedAt = createdAt.Format(time.RFC3339)
		t.UpdatedAt = updatedAt.Format(time.RFC3339)
		templates = append(templates, t)
	}
	if templates == nil {
		templates = []ServiceCardTemplate{}
	}

	return &TemplateListResult{
		Templates: templates,
		Total:     total,
		Page:      params.Page,
		PageSize:  params.PageSize,
	}, rows.Err()
}

// GetTemplate returns a single template by ID.
func (s *Service) GetTemplate(ctx context.Context, merchantID int64, id int64) (*ServiceCardTemplate, error) {
	var t ServiceCardTemplate
	var createdAt, updatedAt time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT id, merchant_id, name, service_item_id, total_uses, price_cents, validity_days, status, created_at, updated_at
		 FROM service_card_templates
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		id, merchantID,
	).Scan(&t.ID, &t.MerchantID, &t.Name, &t.ServiceItemID, &t.TotalUses, &t.PriceCents,
		&t.ValidityDays, &t.Status, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("template not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get template", err)
	}
	t.CreatedAt = createdAt.Format(time.RFC3339)
	t.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &t, nil
}

// UpdateTemplate updates a service card template.
func (s *Service) UpdateTemplate(ctx context.Context, merchantID int64, id int64, req UpdateTemplateRequest) (*ServiceCardTemplate, error) {
	// Verify ownership.
	var current ServiceCardTemplate
	var cCreatedAt, cUpdatedAt time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT id, merchant_id, name, service_item_id, total_uses, price_cents, validity_days, status, created_at, updated_at
		 FROM service_card_templates
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		id, merchantID,
	).Scan(&current.ID, &current.MerchantID, &current.Name, &current.ServiceItemID,
		&current.TotalUses, &current.PriceCents, &current.ValidityDays, &current.Status,
		&cCreatedAt, &cUpdatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("template not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get template", err)
	}

	if req.Name != nil {
		current.Name = *req.Name
	}
	if req.ServiceItemID != nil {
		current.ServiceItemID = req.ServiceItemID
	}
	if req.TotalUses != nil {
		if *req.TotalUses <= 0 {
			return nil, apperrors.NewValidationError("total_uses must be greater than 0")
		}
		current.TotalUses = *req.TotalUses
	}
	if req.PriceCents != nil {
		if *req.PriceCents <= 0 {
			return nil, apperrors.NewValidationError("price_cents must be greater than 0")
		}
		current.PriceCents = *req.PriceCents
	}
	if req.ValidityDays != nil {
		if *req.ValidityDays <= 0 {
			return nil, apperrors.NewValidationError("validity_days must be greater than 0")
		}
		current.ValidityDays = *req.ValidityDays
	}

	var updatedAt time.Time
	err = s.db.QueryRowContext(ctx,
		`UPDATE service_card_templates SET name = $1, service_item_id = $2, total_uses = $3,
		 price_cents = $4, validity_days = $5, updated_at = NOW()
		 WHERE id = $6 AND merchant_id = $7
		 RETURNING updated_at`,
		current.Name, current.ServiceItemID, current.TotalUses, current.PriceCents,
		current.ValidityDays, id, merchantID,
	).Scan(&updatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update template", err)
	}
	current.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &current, nil
}

// ToggleTemplate toggles a template between active and inactive.
func (s *Service) ToggleTemplate(ctx context.Context, merchantID int64, id int64) (*ServiceCardTemplate, error) {
	var t ServiceCardTemplate
	var createdAt, updatedAt time.Time
	err := s.db.QueryRowContext(ctx,
		`UPDATE service_card_templates
		 SET status = CASE WHEN status = 'active' THEN 'inactive' ELSE 'active' END,
		     updated_at = NOW()
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL
		 RETURNING id, merchant_id, name, service_item_id, total_uses, price_cents, validity_days, status, created_at, updated_at`,
		id, merchantID,
	).Scan(&t.ID, &t.MerchantID, &t.Name, &t.ServiceItemID, &t.TotalUses, &t.PriceCents,
		&t.ValidityDays, &t.Status, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("template not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to toggle template", err)
	}
	t.CreatedAt = createdAt.Format(time.RFC3339)
	t.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &t, nil
}

// --- Purchase ---

// Purchase creates a service card for a member based on a template.
func (s *Service) Purchase(ctx context.Context, merchantID int64, templateID int64, req PurchaseRequest) (*ServiceCard, error) {
	if req.MemberID <= 0 {
		return nil, apperrors.NewValidationError("member_id is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	// Get template with row lock.
	var tmpl ServiceCardTemplate
	var tCreatedAt, tUpdatedAt time.Time
	err = tx.QueryRowContext(ctx,
		`SELECT id, merchant_id, name, service_item_id, total_uses, price_cents, validity_days, status, created_at, updated_at
		 FROM service_card_templates
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL
		 FOR UPDATE`,
		templateID, merchantID,
	).Scan(&tmpl.ID, &tmpl.MerchantID, &tmpl.Name, &tmpl.ServiceItemID, &tmpl.TotalUses,
		&tmpl.PriceCents, &tmpl.ValidityDays, &tmpl.Status, &tCreatedAt, &tUpdatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("template not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get template", err)
	}
	if tmpl.Status != "active" {
		return nil, apperrors.NewValidationError("template is not active")
	}

	// Verify member belongs to merchant.
	var memberName string
	err = tx.QueryRowContext(ctx,
		`SELECT name FROM members WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		req.MemberID, merchantID,
	).Scan(&memberName)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("member not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to verify member", err)
	}

	// Generate unique code.
	code := generateCode()
	validFrom := time.Now()
	validUntil := validFrom.AddDate(0, 0, tmpl.ValidityDays)

	var card ServiceCard
	var vfTime, vuTime time.Time
	var siName sql.NullString
	err = tx.QueryRowContext(ctx,
		`INSERT INTO service_cards (merchant_id, member_id, code, name, service_item_id,
		 total_uses, used_count, remaining_uses, status, valid_from, valid_until)
		 VALUES ($1, $2, $3, $4, $5, $6, 0, $6, 'active', $7, $8)
		 RETURNING id, merchant_id, member_id, code, name, service_item_id,
		           total_uses, used_count, remaining_uses, status, valid_from, valid_until, created_at`,
		merchantID, req.MemberID, code, tmpl.Name, tmpl.ServiceItemID,
		tmpl.TotalUses, validFrom, validUntil,
	).Scan(&card.ID, &card.MerchantID, &card.MemberID, &card.Code, &card.Name,
		&card.ServiceItemID, &card.TotalUses, &card.UsedCount, &card.RemainingUses,
		&card.Status, &vfTime, &vuTime, &card.CreatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create service card", err)
	}

	// Get service item name if linked.
	if card.ServiceItemID != nil {
		_ = tx.QueryRowContext(ctx, `SELECT name FROM service_items WHERE id = $1`, *card.ServiceItemID).Scan(&siName)
	}
	if siName.Valid {
		card.ServiceName = siName.String
	}
	card.MemberName = memberName
	card.CreatedAt = vfTime.Format(time.RFC3339)
	vfStr := vfTime.Format(time.RFC3339)
	vuStr := vuTime.Format(time.RFC3339)
	card.ValidFrom = &vfStr
	card.ValidUntil = &vuStr

	// If payment is required, deduct from member balance or record.
	if req.PaymentCents > 0 {
		var balance int64
		err = tx.QueryRowContext(ctx,
			`SELECT COALESCE(balance_cents, 0) FROM members WHERE id = $1 FOR UPDATE`,
			req.MemberID,
		).Scan(&balance)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to check balance", err)
		}
		if balance < req.PaymentCents {
			return nil, apperrors.NewValidationError("insufficient balance: need " + strconv.FormatInt(req.PaymentCents/100, 10) + " yuan, have " + strconv.FormatInt(balance/100, 10) + " yuan")
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE members SET balance_cents = balance_cents - $1, updated_at = NOW()
			 WHERE id = $2`,
			req.PaymentCents, req.MemberID,
		)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to deduct balance", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit", err)
	}

	return &card, nil
}

// --- Member Cards ---

// GetMemberCards returns all service cards for a member.
func (s *Service) GetMemberCards(ctx context.Context, merchantID int64, memberID int64) ([]ServiceCard, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT sc.id, sc.merchant_id, sc.member_id, sc.code, sc.name, sc.service_item_id,
		        COALESCE(si.name, ''), sc.total_uses, sc.used_count, sc.remaining_uses, sc.status,
		        sc.valid_from, sc.valid_until, COALESCE(m.name, ''), sc.created_at
		 FROM service_cards sc
		 LEFT JOIN service_items si ON si.id = sc.service_item_id
		 LEFT JOIN members m ON m.id = sc.member_id
		 WHERE sc.member_id = $1 AND sc.merchant_id = $2 AND sc.status != 'expired'
		 ORDER BY sc.created_at DESC`,
		memberID, merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query member cards", err)
	}
	defer rows.Close()

	var cards []ServiceCard
	for rows.Next() {
		var c ServiceCard
		var vfTime, vuTime sql.NullTime
		if err := rows.Scan(&c.ID, &c.MerchantID, &c.MemberID, &c.Code, &c.Name,
			&c.ServiceItemID, &c.ServiceName, &c.TotalUses, &c.UsedCount, &c.RemainingUses,
			&c.Status, &vfTime, &vuTime, &c.MemberName, &c.CreatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan card", err)
		}
		if vfTime.Valid {
			s := vfTime.Time.Format(time.RFC3339)
			c.ValidFrom = &s
		}
		if vuTime.Valid {
			s := vuTime.Time.Format(time.RFC3339)
			c.ValidUntil = &s
		}
		cards = append(cards, c)
	}
	if cards == nil {
		cards = []ServiceCard{}
	}
	return cards, rows.Err()
}

// --- Usage Logs ---

// GetUsageLogs returns usage records for a service card.
func (s *Service) GetUsageLogs(ctx context.Context, merchantID int64, cardID int64, page, pageSize int) (*UsageLogListResult, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM verification_records
		 WHERE merchant_id = $1 AND reference_id = $2 AND verification_type = 'service_card'`,
		merchantID, cardID,
	).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count logs", err)
	}

	offset := (page - 1) * pageSize
	rows, err := s.db.QueryContext(ctx,
		`SELECT vr.id, vr.merchant_id, vr.reference_id, vr.code, sc.name,
		        vr.verification_type, vr.result, vr.detail, vr.order_id, vr.verified_at
		 FROM verification_records vr
		 LEFT JOIN service_cards sc ON sc.id = vr.reference_id
		 WHERE vr.merchant_id = $1 AND vr.reference_id = $2 AND vr.verification_type = 'service_card'
		 ORDER BY vr.verified_at DESC LIMIT $3 OFFSET $4`,
		merchantID, cardID, pageSize, offset,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query logs", err)
	}
	defer rows.Close()

	var logs []UsageLog
	for rows.Next() {
		var l UsageLog
		var verifiedAt time.Time
		if err := rows.Scan(&l.ID, &l.MerchantID, &l.ServiceCardID, &l.Code, &l.Name,
			&l.VerificationType, &l.Result, &l.Detail, &l.OrderID, &verifiedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan log", err)
		}
		l.VerifiedAt = verifiedAt.Format(time.RFC3339)
		logs = append(logs, l)
	}
	if logs == nil {
		logs = []UsageLog{}
	}

	return &UsageLogListResult{
		Logs:     logs,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, rows.Err()
}

// --- Expiry Reminders ---

// GetExpiringCards returns cards expiring within the specified days.
func (s *Service) GetExpiringCards(ctx context.Context, merchantID int64, withinDays int) (*ExpiringCardsResult, error) {
	if withinDays <= 0 {
		withinDays = 30
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT sc.id, sc.merchant_id, sc.member_id, sc.code, sc.name, sc.service_item_id,
		        COALESCE(si.name, ''), sc.total_uses, sc.used_count, sc.remaining_uses, sc.status,
		        sc.valid_from, sc.valid_until, COALESCE(m.name, ''), sc.created_at
		 FROM service_cards sc
		 LEFT JOIN service_items si ON si.id = sc.service_item_id
		 LEFT JOIN members m ON m.id = sc.member_id
		 WHERE sc.merchant_id = $1 AND sc.status = 'active'
		   AND sc.valid_until IS NOT NULL
		   AND sc.valid_until >= NOW()
		   AND sc.valid_until <= NOW() + ($2 || ' days')::INTERVAL
		 ORDER BY sc.valid_until ASC`,
		merchantID, strconv.Itoa(withinDays),
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query expiring cards", err)
	}
	defer rows.Close()

	var cards []ServiceCard
	for rows.Next() {
		var c ServiceCard
		var vfTime, vuTime sql.NullTime
		if err := rows.Scan(&c.ID, &c.MerchantID, &c.MemberID, &c.Code, &c.Name,
			&c.ServiceItemID, &c.ServiceName, &c.TotalUses, &c.UsedCount, &c.RemainingUses,
			&c.Status, &vfTime, &vuTime, &c.MemberName, &c.CreatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan card", err)
		}
		if vfTime.Valid {
			s := vfTime.Time.Format(time.RFC3339)
			c.ValidFrom = &s
		}
		if vuTime.Valid {
			s := vuTime.Time.Format(time.RFC3339)
			c.ValidUntil = &s
		}
		cards = append(cards, c)
	}
	if cards == nil {
		cards = []ServiceCard{}
	}

	return &ExpiringCardsResult{
		Cards: cards,
		Total: len(cards),
	}, rows.Err()
}

// GetCardByCode returns a service card by its code for verification display.
func (s *Service) GetCardByCode(ctx context.Context, merchantID int64, code string) (*ServiceCard, error) {
	var c ServiceCard
	var vfTime, vuTime sql.NullTime
	var siName sql.NullString
	var memberName sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT sc.id, sc.merchant_id, sc.member_id, sc.code, sc.name, sc.service_item_id,
		        COALESCE(si.name, ''), sc.total_uses, sc.used_count, sc.remaining_uses, sc.status,
		        sc.valid_from, sc.valid_until, COALESCE(m.name, ''), sc.created_at
		 FROM service_cards sc
		 LEFT JOIN service_items si ON si.id = sc.service_item_id
		 LEFT JOIN members m ON m.id = sc.member_id
		 WHERE sc.code = $1 AND sc.merchant_id = $2
		 LIMIT 1`,
		code, merchantID,
	).Scan(&c.ID, &c.MerchantID, &c.MemberID, &c.Code, &c.Name,
		&c.ServiceItemID, &siName, &c.TotalUses, &c.UsedCount, &c.RemainingUses,
		&c.Status, &vfTime, &vuTime, &memberName, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("service card not found: " + code)
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query service card", err)
	}
	if siName.Valid {
		c.ServiceName = siName.String
	}
	if memberName.Valid {
		c.MemberName = memberName.String
	}
	if vfTime.Valid {
		s := vfTime.Time.Format(time.RFC3339)
		c.ValidFrom = &s
	}
	if vuTime.Valid {
		s := vuTime.Time.Format(time.RFC3339)
		c.ValidUntil = &s
	}
	return &c, nil
}

// GetAllCards lists all service cards for a merchant.
func (s *Service) GetAllCards(ctx context.Context, merchantID int64, params ListParams) (*CardListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}

	var conditions []string
	var args []interface{}
	conditions = append(conditions, "sc.merchant_id = $1")
	args = append(args, merchantID)
	argIdx := 2

	if params.Status != "" {
		conditions = append(conditions, "sc.status = $"+strconv.Itoa(argIdx))
		args = append(args, params.Status)
		argIdx++
	}
	if params.Keyword != "" {
		conditions = append(conditions, "(sc.code ILIKE $"+strconv.Itoa(argIdx)+" OR sc.name ILIKE $"+strconv.Itoa(argIdx)+" OR m.name ILIKE $"+strconv.Itoa(argIdx)+")")
		args = append(args, "%"+params.Keyword+"%")
		argIdx++
	}

	whereClause := strings.Join(conditions, " AND ")

	var total int
	countQuery := `SELECT COUNT(*) FROM service_cards sc LEFT JOIN members m ON m.id = sc.member_id WHERE ` + whereClause
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count cards", err)
	}

	offset := (params.Page - 1) * params.PageSize
	queryArgs := append(args, params.PageSize, offset)
	rows, err := s.db.QueryContext(ctx,
		`SELECT sc.id, sc.merchant_id, sc.member_id, sc.code, sc.name, sc.service_item_id,
		        COALESCE(si.name, ''), sc.total_uses, sc.used_count, sc.remaining_uses, sc.status,
		        sc.valid_from, sc.valid_until, COALESCE(m.name, ''), sc.created_at
		 FROM service_cards sc
		 LEFT JOIN service_items si ON si.id = sc.service_item_id
		 LEFT JOIN members m ON m.id = sc.member_id
		 WHERE `+whereClause+
			` ORDER BY sc.created_at DESC LIMIT $`+strconv.Itoa(argIdx)+` OFFSET $`+strconv.Itoa(argIdx+1),
		queryArgs...,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query cards", err)
	}
	defer rows.Close()

	var cards []ServiceCard
	for rows.Next() {
		var c ServiceCard
		var vfTime, vuTime sql.NullTime
		if err := rows.Scan(&c.ID, &c.MerchantID, &c.MemberID, &c.Code, &c.Name,
			&c.ServiceItemID, &c.ServiceName, &c.TotalUses, &c.UsedCount, &c.RemainingUses,
			&c.Status, &vfTime, &vuTime, &c.MemberName, &c.CreatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan card", err)
		}
		if vfTime.Valid {
			s := vfTime.Time.Format(time.RFC3339)
			c.ValidFrom = &s
		}
		if vuTime.Valid {
			s := vuTime.Time.Format(time.RFC3339)
			c.ValidUntil = &s
		}
		cards = append(cards, c)
	}
	if cards == nil {
		cards = []ServiceCard{}
	}

	return &CardListResult{
		Cards:    cards,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, rows.Err()
}

// IntPtr helper.
func IntPtr(v int) *int {
	return &v
}

// Int64Ptr helper.
func Int64Ptr(v int64) *int64 {
	return &v
}

// --- Import needed for unused fmt import ---
var _ = fmt.Sprintf
