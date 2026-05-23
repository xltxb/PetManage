package coupon

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Template types.
const (
	TypeFullReduction = "full_reduction"
	TypeDiscount      = "discount"
	TypeCashVoucher   = "cash_voucher"
	TypeNewMember     = "new_member"
	TypeBirthday      = "birthday"
)

// Issue methods.
const (
	IssueManual       = "manual"
	IssueAutoNewMember = "auto_new_member"
	IssueAutoBirthday  = "auto_birthday"
)

// CouponTemplate represents a coupon rule definition.
type CouponTemplate struct {
	ID                   int64   `json:"id"`
	MerchantID           int64   `json:"merchant_id"`
	Name                 string  `json:"name"`
	Description          string  `json:"description"`
	Type                 string  `json:"type"`
	ValueCents           int     `json:"value_cents"`
	MinOrderCents        int     `json:"min_order_cents"`
	MaxDiscountCents     int     `json:"max_discount_cents"`
	ValidityDays         int     `json:"validity_days"`
	MaxClaimsPerMember   int     `json:"max_claims_per_member"`
	ApplicableCategories string  `json:"applicable_categories"`
	IssueMethod          string  `json:"issue_method"`
	TotalIssued          int     `json:"total_issued"`
	TotalUsed            int     `json:"total_used"`
	Status               string  `json:"status"`
	CreatedAt            string  `json:"created_at"`
	UpdatedAt            string  `json:"updated_at"`
}

// CouponCode represents an individual issued coupon.
type CouponCode struct {
	ID          int64   `json:"id"`
	TemplateID  int64   `json:"template_id"`
	MerchantID  int64   `json:"merchant_id"`
	MemberID    *int64  `json:"member_id,omitempty"`
	Code        string  `json:"code"`
	Status      string  `json:"status"`
	UsedAt      *string `json:"used_at,omitempty"`
	UsedOrderID *int64  `json:"used_order_id,omitempty"`
	ExpiresAt   string  `json:"expires_at"`
	ClaimedAt   string  `json:"claimed_at"`
}

// TemplateStats holds coupon usage statistics.
type TemplateStats struct {
	TemplateID   int64   `json:"template_id"`
	TemplateName string  `json:"template_name"`
	Type         string  `json:"type"`
	TotalIssued  int     `json:"total_issued"`
	TotalUsed    int     `json:"total_used"`
	UsageRate    float64 `json:"usage_rate"`
}

// CreateTemplateRequest is the input for creating a coupon template.
type CreateTemplateRequest struct {
	Name                 string `json:"name"`
	Description          string `json:"description"`
	Type                 string `json:"type"`
	ValueCents           int    `json:"value_cents"`
	MinOrderCents        int    `json:"min_order_cents"`
	MaxDiscountCents     int    `json:"max_discount_cents"`
	ValidityDays         int    `json:"validity_days"`
	MaxClaimsPerMember   int    `json:"max_claims_per_member"`
	ApplicableCategories []int64 `json:"applicable_categories"`
	IssueMethod          string `json:"issue_method"`
}

// IssueRequest specifies how many coupons to issue and to whom.
type IssueRequest struct {
	MemberIDs []int64 `json:"member_ids"`
	Count     int     `json:"count"` // per member, default 1
}

// TemplateListParams holds filters for listing templates.
type TemplateListParams struct {
	Type   string
	Status string
	Page   int
	PageSize int
}

// TemplateListResult wraps template list with pagination.
type TemplateListResult struct {
	Templates []CouponTemplate `json:"templates"`
	Total     int              `json:"total"`
	Page      int              `json:"page"`
	PageSize  int              `json:"page_size"`
}

// CodeListParams holds filters for listing coupon codes.
type CodeListParams struct {
	TemplateID int64
	MemberID   int64
	Status     string
	Page       int
	PageSize   int
}

// CodeListResult wraps code list with pagination.
type CodeListResult struct {
	Codes    []CouponCode `json:"codes"`
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

// Service provides coupon management operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new coupon Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// CreateTemplate creates a new coupon template.
func (s *Service) CreateTemplate(ctx context.Context, merchantID int64, req CreateTemplateRequest) (*CouponTemplate, error) {
	if req.Name == "" {
		return nil, apperrors.NewValidationError("name is required")
	}
	if req.Type == "" {
		return nil, apperrors.NewValidationError("type is required")
	}
	validTypes := map[string]bool{TypeFullReduction: true, TypeDiscount: true, TypeCashVoucher: true, TypeNewMember: true, TypeBirthday: true}
	if !validTypes[req.Type] {
		return nil, apperrors.NewValidationError("invalid coupon type: " + req.Type)
	}
	if req.ValueCents <= 0 {
		return nil, apperrors.NewValidationError("value_cents must be positive")
	}
	if req.ValidityDays <= 0 {
		req.ValidityDays = 30
	}
	if req.MaxClaimsPerMember <= 0 {
		req.MaxClaimsPerMember = 1
	}
	validMethods := map[string]bool{IssueManual: true, IssueAutoNewMember: true, IssueAutoBirthday: true}
	if !validMethods[req.IssueMethod] {
		return nil, apperrors.NewValidationError("invalid issue_method: " + req.IssueMethod)
	}

	catsJSON := "[]"
	if len(req.ApplicableCategories) > 0 {
		b, _ := json.Marshal(req.ApplicableCategories)
		catsJSON = string(b)
	}

	var t CouponTemplate
	var createdAt, updatedAt time.Time
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO coupon_templates (merchant_id, name, description, type, value_cents,
		 min_order_cents, max_discount_cents, validity_days, max_claims_per_member,
		 applicable_categories, issue_method, total_issued, total_used, status)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,0,0,'active')
		 RETURNING id, merchant_id, name, description, type, value_cents, min_order_cents,
		 max_discount_cents, validity_days, max_claims_per_member, applicable_categories,
		 issue_method, total_issued, total_used, status, created_at, updated_at`,
		merchantID, req.Name, req.Description, req.Type, req.ValueCents,
		req.MinOrderCents, req.MaxDiscountCents, req.ValidityDays, req.MaxClaimsPerMember,
		catsJSON, req.IssueMethod,
	).Scan(&t.ID, &t.MerchantID, &t.Name, &t.Description, &t.Type, &t.ValueCents,
		&t.MinOrderCents, &t.MaxDiscountCents, &t.ValidityDays, &t.MaxClaimsPerMember,
		&t.ApplicableCategories, &t.IssueMethod, &t.TotalIssued, &t.TotalUsed, &t.Status,
		&createdAt, &updatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create coupon template", err)
	}
	t.CreatedAt = createdAt.Format(time.RFC3339)
	t.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &t, nil
}

// GetTemplate returns a single template by ID.
func (s *Service) GetTemplate(ctx context.Context, merchantID, templateID int64) (*CouponTemplate, error) {
	var t CouponTemplate
	var createdAt, updatedAt time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT id, merchant_id, name, description, type, value_cents, min_order_cents,
		 max_discount_cents, validity_days, max_claims_per_member, applicable_categories,
		 issue_method, total_issued, total_used, status, created_at, updated_at
		 FROM coupon_templates
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		templateID, merchantID,
	).Scan(&t.ID, &t.MerchantID, &t.Name, &t.Description, &t.Type, &t.ValueCents,
		&t.MinOrderCents, &t.MaxDiscountCents, &t.ValidityDays, &t.MaxClaimsPerMember,
		&t.ApplicableCategories, &t.IssueMethod, &t.TotalIssued, &t.TotalUsed, &t.Status,
		&createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("coupon template not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get template", err)
	}
	t.CreatedAt = createdAt.Format(time.RFC3339)
	t.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &t, nil
}

// ListTemplates returns templates with optional filtering and pagination.
func (s *Service) ListTemplates(ctx context.Context, merchantID int64, params TemplateListParams) (*TemplateListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}

	args := []interface{}{merchantID}
	argIdx := 2
	conditions := []string{"merchant_id = $1", "deleted_at IS NULL"}

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

	whereClause := strings.Join(conditions, " AND ")

	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM coupon_templates WHERE `+whereClause, args...,
	).Scan(&total); err != nil {
		return nil, apperrors.NewInternalError("failed to count templates", err)
	}

	offset := (params.Page - 1) * params.PageSize
	queryArgs := append(args, params.PageSize, offset)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, merchant_id, name, description, type, value_cents, min_order_cents,
		 max_discount_cents, validity_days, max_claims_per_member, applicable_categories,
		 issue_method, total_issued, total_used, status, created_at, updated_at
		 FROM coupon_templates
		 WHERE `+whereClause+
			` ORDER BY created_at DESC LIMIT $`+strconv.Itoa(argIdx)+` OFFSET $`+strconv.Itoa(argIdx+1),
		queryArgs...,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list templates", err)
	}
	defer rows.Close()

	var templates []CouponTemplate
	for rows.Next() {
		var t CouponTemplate
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&t.ID, &t.MerchantID, &t.Name, &t.Description, &t.Type, &t.ValueCents,
			&t.MinOrderCents, &t.MaxDiscountCents, &t.ValidityDays, &t.MaxClaimsPerMember,
			&t.ApplicableCategories, &t.IssueMethod, &t.TotalIssued, &t.TotalUsed, &t.Status,
			&createdAt, &updatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan template", err)
		}
		t.CreatedAt = createdAt.Format(time.RFC3339)
		t.UpdatedAt = updatedAt.Format(time.RFC3339)
		templates = append(templates, t)
	}
	if templates == nil {
		templates = []CouponTemplate{}
	}

	return &TemplateListResult{
		Templates: templates,
		Total:     total,
		Page:      params.Page,
		PageSize:  params.PageSize,
	}, rows.Err()
}

// UpdateTemplate updates an existing coupon template.
func (s *Service) UpdateTemplate(ctx context.Context, merchantID, templateID int64, req CreateTemplateRequest) (*CouponTemplate, error) {
	if req.Name == "" {
		return nil, apperrors.NewValidationError("name is required")
	}

	catsJSON := "[]"
	if len(req.ApplicableCategories) > 0 {
		b, _ := json.Marshal(req.ApplicableCategories)
		catsJSON = string(b)
	}

	var t CouponTemplate
	var createdAt, updatedAt time.Time
	err := s.db.QueryRowContext(ctx,
		`UPDATE coupon_templates SET
		 name=$1, description=$2, type=$3, value_cents=$4, min_order_cents=$5,
		 max_discount_cents=$6, validity_days=$7, max_claims_per_member=$8,
		 applicable_categories=$9, issue_method=$10, updated_at=NOW()
		 WHERE id=$11 AND merchant_id=$12 AND deleted_at IS NULL
		 RETURNING id, merchant_id, name, description, type, value_cents, min_order_cents,
		 max_discount_cents, validity_days, max_claims_per_member, applicable_categories,
		 issue_method, total_issued, total_used, status, created_at, updated_at`,
		req.Name, req.Description, req.Type, req.ValueCents, req.MinOrderCents,
		req.MaxDiscountCents, req.ValidityDays, req.MaxClaimsPerMember,
		catsJSON, req.IssueMethod,
		templateID, merchantID,
	).Scan(&t.ID, &t.MerchantID, &t.Name, &t.Description, &t.Type, &t.ValueCents,
		&t.MinOrderCents, &t.MaxDiscountCents, &t.ValidityDays, &t.MaxClaimsPerMember,
		&t.ApplicableCategories, &t.IssueMethod, &t.TotalIssued, &t.TotalUsed, &t.Status,
		&createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("coupon template not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update template", err)
	}
	t.CreatedAt = createdAt.Format(time.RFC3339)
	t.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &t, nil
}

// ToggleTemplate enables or disables a coupon template.
func (s *Service) ToggleTemplate(ctx context.Context, merchantID, templateID int64) (*CouponTemplate, error) {
	var t CouponTemplate
	var createdAt, updatedAt time.Time
	err := s.db.QueryRowContext(ctx,
		`UPDATE coupon_templates SET
		 status = CASE WHEN status = 'active' THEN 'disabled' ELSE 'active' END,
		 updated_at = NOW()
		 WHERE id=$1 AND merchant_id=$2 AND deleted_at IS NULL
		 RETURNING id, merchant_id, name, description, type, value_cents, min_order_cents,
		 max_discount_cents, validity_days, max_claims_per_member, applicable_categories,
		 issue_method, total_issued, total_used, status, created_at, updated_at`,
		templateID, merchantID,
	).Scan(&t.ID, &t.MerchantID, &t.Name, &t.Description, &t.Type, &t.ValueCents,
		&t.MinOrderCents, &t.MaxDiscountCents, &t.ValidityDays, &t.MaxClaimsPerMember,
		&t.ApplicableCategories, &t.IssueMethod, &t.TotalIssued, &t.TotalUsed, &t.Status,
		&createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("coupon template not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to toggle template", err)
	}
	t.CreatedAt = createdAt.Format(time.RFC3339)
	t.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &t, nil
}

// generateCode creates a unique coupon code.
func generateCode(prefix string) string {
	b := make([]byte, 4)
	rand.Read(b)
	return strings.ToUpper(prefix + hex.EncodeToString(b))
}

// IssueCoupons issues coupon codes from a template to specified members.
func (s *Service) IssueCoupons(ctx context.Context, merchantID, templateID int64, req IssueRequest) ([]CouponCode, error) {
	tmpl, err := s.GetTemplate(ctx, merchantID, templateID)
	if err != nil {
		return nil, err
	}
	if tmpl.Status != "active" {
		return nil, apperrors.NewValidationError("coupon template is not active")
	}

	if len(req.MemberIDs) == 0 {
		return nil, apperrors.NewValidationError("member_ids is required")
	}
	count := req.Count
	if count <= 0 {
		count = 1
	}

	// Prefix by type for human readability.
	prefixMap := map[string]string{
		TypeFullReduction: "MJ",
		TypeDiscount:      "ZK",
		TypeCashVoucher:   "DJ",
		TypeNewMember:     "XM",
		TypeBirthday:      "SR",
	}
	prefix := prefixMap[tmpl.Type]
	if prefix == "" {
		prefix = "CP"
	}

	expiresAt := time.Now().AddDate(0, 0, tmpl.ValidityDays)

	var codes []CouponCode

	for _, memberID := range req.MemberIDs {
		// Check per-member claim limit.
		var existing int
		err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM coupon_codes
			 WHERE template_id=$1 AND member_id=$2 AND deleted_at IS NULL`,
			templateID, memberID,
		).Scan(&existing)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to check claim limit", err)
		}
		if existing+count > tmpl.MaxClaimsPerMember {
			return nil, apperrors.NewValidationError(
				fmt.Sprintf("member %d has reached the per-member claim limit (%d)", memberID, tmpl.MaxClaimsPerMember),
			)
		}

		for i := 0; i < count; i++ {
			codeStr := generateCode(prefix)

			var ctype string
			switch tmpl.Type {
			case TypeDiscount:
				ctype = "percent"
			default:
				ctype = "fixed"
			}

			// Insert into legacy coupons table for checkout/POS compatibility.
			_, err = s.db.ExecContext(ctx,
				`INSERT INTO coupons (merchant_id, code, type, value_cents, min_order_cents, status)
				 VALUES ($1, $2, $3, $4, $5, 'active')`,
				merchantID, codeStr, ctype, tmpl.ValueCents, tmpl.MinOrderCents,
			)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to create coupon code", err)
			}

			// Insert into coupon_codes management table.
			var c CouponCode
			var claimedAt time.Time
			err = s.db.QueryRowContext(ctx,
				`INSERT INTO coupon_codes (template_id, merchant_id, member_id, code, status, expires_at, claimed_at)
				 VALUES ($1,$2,$3,$4,'active',$5,NOW())
				 RETURNING id, template_id, merchant_id, member_id, code, status, expires_at, claimed_at`,
				templateID, merchantID, memberID, codeStr, expiresAt,
			).Scan(&c.ID, &c.TemplateID, &c.MerchantID, &c.MemberID, &c.Code, &c.Status, &c.ExpiresAt, &claimedAt)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to create coupon code record", err)
			}
			c.ExpiresAt = expiresAt.Format(time.RFC3339)
			c.ClaimedAt = claimedAt.Format(time.RFC3339)
			codes = append(codes, c)
		}
	}

	// Update template counters.
	_, _ = s.db.ExecContext(ctx,
		`UPDATE coupon_templates SET total_issued = total_issued + $1, updated_at = NOW()
		 WHERE id = $2`, len(codes), templateID,
	)

	return codes, nil
}

// ListCodes returns issued coupon codes with optional filtering.
func (s *Service) ListCodes(ctx context.Context, merchantID int64, params CodeListParams) (*CodeListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}

	args := []interface{}{merchantID}
	argIdx := 2
	conditions := []string{"cc.merchant_id = $1", "cc.deleted_at IS NULL"}

	if params.TemplateID > 0 {
		conditions = append(conditions, "cc.template_id = $"+strconv.Itoa(argIdx))
		args = append(args, params.TemplateID)
		argIdx++
	}
	if params.MemberID > 0 {
		conditions = append(conditions, "cc.member_id = $"+strconv.Itoa(argIdx))
		args = append(args, params.MemberID)
		argIdx++
	}
	if params.Status != "" {
		conditions = append(conditions, "cc.status = $"+strconv.Itoa(argIdx))
		args = append(args, params.Status)
		argIdx++
	}

	whereClause := strings.Join(conditions, " AND ")

	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM coupon_codes cc WHERE `+whereClause, args...,
	).Scan(&total); err != nil {
		return nil, apperrors.NewInternalError("failed to count codes", err)
	}

	offset := (params.Page - 1) * params.PageSize
	queryArgs := append(args, params.PageSize, offset)
	rows, err := s.db.QueryContext(ctx,
		`SELECT cc.id, cc.template_id, cc.merchant_id, cc.member_id, cc.code,
		 cc.status, cc.used_at, cc.used_order_id, cc.expires_at, cc.claimed_at
		 FROM coupon_codes cc
		 WHERE `+whereClause+
			` ORDER BY cc.claimed_at DESC LIMIT $`+strconv.Itoa(argIdx)+` OFFSET $`+strconv.Itoa(argIdx+1),
		queryArgs...,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list codes", err)
	}
	defer rows.Close()

	var codes []CouponCode
	for rows.Next() {
		var c CouponCode
		var usedAt sql.NullTime
		var memberID sql.NullInt64
		var usedOrderID sql.NullInt64
		var expiresAt, claimedAt time.Time
		if err := rows.Scan(&c.ID, &c.TemplateID, &c.MerchantID, &memberID, &c.Code,
			&c.Status, &usedAt, &usedOrderID, &expiresAt, &claimedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan code", err)
		}
		if memberID.Valid {
			c.MemberID = &memberID.Int64
		}
		if usedAt.Valid {
			s := usedAt.Time.Format(time.RFC3339)
			c.UsedAt = &s
		}
		if usedOrderID.Valid {
			c.UsedOrderID = &usedOrderID.Int64
		}
		c.ExpiresAt = expiresAt.Format(time.RFC3339)
		c.ClaimedAt = claimedAt.Format(time.RFC3339)
		codes = append(codes, c)
	}
	if codes == nil {
		codes = []CouponCode{}
	}

	return &CodeListResult{
		Codes:    codes,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, rows.Err()
}

// GetStats returns coupon usage statistics for all active templates.
func (s *Service) GetStats(ctx context.Context, merchantID int64) ([]TemplateStats, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, type, total_issued, total_used
		 FROM coupon_templates
		 WHERE merchant_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at DESC`,
		merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get stats", err)
	}
	defer rows.Close()

	var stats []TemplateStats
	for rows.Next() {
		var st TemplateStats
		if err := rows.Scan(&st.TemplateID, &st.TemplateName, &st.Type,
			&st.TotalIssued, &st.TotalUsed); err != nil {
			return nil, apperrors.NewInternalError("failed to scan stats", err)
		}
		if st.TotalIssued > 0 {
			st.UsageRate = float64(st.TotalUsed) / float64(st.TotalIssued) * 100
		}
		stats = append(stats, st)
	}
	if stats == nil {
		stats = []TemplateStats{}
	}
	return stats, rows.Err()
}

// MarkCodeUsed updates a coupon code as used (called during checkout/POS integration).
func (s *Service) MarkCodeUsed(ctx context.Context, code string, memberID, orderID int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE coupon_codes SET status = 'used', used_at = NOW(), used_order_id = $1, updated_at = NOW()
		 WHERE code = $2 AND status = 'active'`,
		orderID, code,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to mark code used", err)
	}
	// Increment template used count.
	_, _ = s.db.ExecContext(ctx,
		`UPDATE coupon_templates SET total_used = total_used + 1, updated_at = NOW()
		 WHERE id = (SELECT template_id FROM coupon_codes WHERE code = $1)`, code,
	)
	return nil
}
