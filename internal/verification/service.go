package verification

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// --- Types ---

// CouponResult represents the result of verifying a coupon code.
type CouponResult struct {
	ID             int64  `json:"id"`
	Code           string `json:"code"`
	DiscountType   string `json:"discount_type"`
	ValueCents     int    `json:"value_cents"`
	MinOrderCents  int    `json:"min_order_cents"`
	Status         string `json:"status"`
	UsedAt         *string `json:"used_at,omitempty"`
	UsedByMemberID *int64  `json:"used_by_member_id,omitempty"`
	UsedOrderID    *int64  `json:"used_order_id,omitempty"`
}

// ServiceCardResult represents the result of verifying a service card.
type ServiceCardResult struct {
	ID             int64  `json:"id"`
	Code           string `json:"code"`
	Name           string `json:"name"`
	ServiceItemID  *int64 `json:"service_item_id,omitempty"`
	ServiceName    string `json:"service_name,omitempty"`
	TotalUses      int    `json:"total_uses"`
	UsedCount      int    `json:"used_count"`
	RemainingUses  int    `json:"remaining_uses"`
	Status         string `json:"status"`
	ValidUntil     *string `json:"valid_until,omitempty"`
	MemberID       *int64 `json:"member_id,omitempty"`
	MemberName     string `json:"member_name,omitempty"`
}

// ThirdPartyVoucherResult represents the result of verifying a third-party voucher.
type ThirdPartyVoucherResult struct {
	ID              int64  `json:"id"`
	Code            string `json:"code"`
	Source          string `json:"source"`
	Name            string `json:"name"`
	ServiceItemID   *int64 `json:"service_item_id,omitempty"`
	ServiceName     string `json:"service_name,omitempty"`
	AmountCents     int64  `json:"amount_cents"`
	Status          string `json:"status"`
	VerifiedAt      *string `json:"verified_at,omitempty"`
	VerifiedOrderID *int64  `json:"verified_order_id,omitempty"`
}

// VerificationRecord represents a single verification log entry.
type VerificationRecord struct {
	ID               int64  `json:"id"`
	MerchantID       int64  `json:"merchant_id"`
	VerificationType string `json:"verification_type"`
	Code             string `json:"code"`
	ReferenceID      int64  `json:"reference_id"`
	Result           string `json:"result"`
	Detail           string `json:"detail"`
	OrderID          *int64 `json:"order_id,omitempty"`
	VerifiedBy       int64  `json:"verified_by"`
	VerifiedAt       string `json:"verified_at"`
	CreatedAt        string `json:"created_at"`
}

// VerifyRequest is the request body for code verification.
type VerifyRequest struct {
	Code    string `json:"code"`
	OrderID *int64 `json:"order_id,omitempty"`
}

// ListParams holds optional filters for listing verification records.
type ListParams struct {
	VerificationType string
	Code             string
	Page             int
	PageSize         int
}

// ListResult wraps the records list with pagination.
type ListResult struct {
	Records  []VerificationRecord `json:"records"`
	Total    int                  `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

// Service provides verification operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new verification Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// VerifyCoupon validates and redeems a coupon code. Already-used coupons are rejected.
func (s *Service) VerifyCoupon(ctx context.Context, merchantID, userID int64, code string, orderID *int64) (*CouponResult, error) {
	if code == "" {
		return nil, apperrors.NewValidationError("coupon code is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	var c CouponResult
	var usedAt sql.NullTime
	err = tx.QueryRowContext(ctx,
		`SELECT id, code, type, value_cents, min_order_cents, status,
		        used_at, used_by_member_id, used_order_id
		 FROM coupons
		 WHERE code = $1 AND merchant_id = $2 AND deleted_at IS NULL
		 LIMIT 1`,
		code, merchantID,
	).Scan(&c.ID, &c.Code, &c.DiscountType, &c.ValueCents, &c.MinOrderCents, &c.Status,
		&usedAt, &c.UsedByMemberID, &c.UsedOrderID)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("coupon not found: " + code)
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query coupon", err)
	}

	if c.Status == "used" {
		s.recordVerification(ctx, tx, merchantID, "coupon", code, c.ID, "failed",
			"coupon already used at "+usedAt.Time.Format(time.RFC3339), orderID, userID)
		tx.Commit()
		return nil, apperrors.NewValidationError("coupon already used at " + usedAt.Time.Format("2006-01-02 15:04"))
	}
	if c.Status == "expired" {
		s.recordVerification(ctx, tx, merchantID, "coupon", code, c.ID, "failed",
			"coupon expired", orderID, userID)
		tx.Commit()
		return nil, apperrors.NewValidationError("coupon has expired")
	}
	if c.Status == "disabled" {
		s.recordVerification(ctx, tx, merchantID, "coupon", code, c.ID, "failed",
			"coupon disabled", orderID, userID)
		tx.Commit()
		return nil, apperrors.NewValidationError("coupon is disabled")
	}

	// Mark coupon as used.
	_, err = tx.ExecContext(ctx,
		`UPDATE coupons SET status = 'used', used_at = NOW(), used_by_member_id = NULL,
		 used_order_id = $1, updated_at = NOW()
		 WHERE id = $2`,
		orderID, c.ID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to mark coupon as used", err)
	}

	if usedAt.Valid {
		c.UsedAt = strPtr(usedAt.Time.Format(time.RFC3339))
	}

	s.recordVerification(ctx, tx, merchantID, "coupon", code, c.ID, "success",
		"coupon verified: ¥"+strconv.Itoa(c.ValueCents/100)+" ("+c.DiscountType+")", orderID, userID)

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit", err)
	}

	c.Status = "used"
	return &c, nil
}

// VerifyThirdPartyVoucher validates and redeems a third-party voucher code.
func (s *Service) VerifyThirdPartyVoucher(ctx context.Context, merchantID, userID int64, code string, orderID *int64) (*ThirdPartyVoucherResult, error) {
	if code == "" {
		return nil, apperrors.NewValidationError("voucher code is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	var v ThirdPartyVoucherResult
	var verifiedAt sql.NullTime
	var siName sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT tpv.id, tpv.code, tpv.source, tpv.name, tpv.service_item_id,
		        COALESCE(si.name, ''), tpv.amount_cents, tpv.status, tpv.verified_at, tpv.verified_order_id
		 FROM third_party_vouchers tpv
		 LEFT JOIN service_items si ON si.id = tpv.service_item_id
		 WHERE tpv.code = $1 AND tpv.merchant_id = $2
		 LIMIT 1`,
		code, merchantID,
	).Scan(&v.ID, &v.Code, &v.Source, &v.Name, &v.ServiceItemID,
		&siName, &v.AmountCents, &v.Status, &verifiedAt, &v.VerifiedOrderID)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("voucher not found: " + code)
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query voucher", err)
	}
	if siName.Valid {
		v.ServiceName = siName.String
	}

	if v.Status == "verified" {
		vTime := ""
		if verifiedAt.Valid {
			vTime = verifiedAt.Time.Format("2006-01-02 15:04")
		}
		s.recordVerification(ctx, tx, merchantID, "third_party_voucher", code, v.ID, "failed",
			"voucher already verified at "+vTime, orderID, userID)
		tx.Commit()
		return nil, apperrors.NewValidationError("voucher already verified at " + vTime)
	}
	if v.Status == "expired" {
		s.recordVerification(ctx, tx, merchantID, "third_party_voucher", code, v.ID, "failed",
			"voucher expired", orderID, userID)
		tx.Commit()
		return nil, apperrors.NewValidationError("voucher has expired")
	}

	// Mark voucher as verified.
	_, err = tx.ExecContext(ctx,
		`UPDATE third_party_vouchers SET status = 'verified', verified_at = NOW(),
		 verified_by = $1, verified_order_id = $2, updated_at = NOW()
		 WHERE id = $3`,
		userID, orderID, v.ID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to verify voucher", err)
	}

	detail := "third-party voucher verified: " + v.Source + " ¥" + strconv.FormatInt(v.AmountCents/100, 10)
	s.recordVerification(ctx, tx, merchantID, "third_party_voucher", code, v.ID, "success",
		detail, orderID, userID)

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit", err)
	}

	v.Status = "verified"
	now := time.Now().Format(time.RFC3339)
	v.VerifiedAt = &now
	return &v, nil
}

// VerifyServiceCard validates a service card code, deducts one use, and returns updated info.
func (s *Service) VerifyServiceCard(ctx context.Context, merchantID, userID int64, code string, orderID *int64) (*ServiceCardResult, error) {
	if code == "" {
		return nil, apperrors.NewValidationError("service card code is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	var sc ServiceCardResult
	var validUntil sql.NullTime
	var siName sql.NullString
	var memberName sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT sc.id, sc.code, sc.name, sc.service_item_id, COALESCE(si.name, ''),
		        sc.total_uses, sc.used_count, sc.remaining_uses, sc.status,
		        sc.valid_until, sc.member_id, COALESCE(m.name, '')
		 FROM service_cards sc
		 LEFT JOIN service_items si ON si.id = sc.service_item_id
		 LEFT JOIN members m ON m.id = sc.member_id
		 WHERE sc.code = $1 AND sc.merchant_id = $2
		 LIMIT 1
		 FOR UPDATE OF sc`,
		code, merchantID,
	).Scan(&sc.ID, &sc.Code, &sc.Name, &sc.ServiceItemID, &siName,
		&sc.TotalUses, &sc.UsedCount, &sc.RemainingUses, &sc.Status,
		&validUntil, &sc.MemberID, &memberName)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("service card not found: " + code)
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query service card", err)
	}
	if siName.Valid {
		sc.ServiceName = siName.String
	}
	if memberName.Valid {
		sc.MemberName = memberName.String
	}
	if validUntil.Valid {
		sc.ValidUntil = strPtr(validUntil.Time.Format(time.RFC3339))
	}

	if sc.Status == "used" {
		s.recordVerification(ctx, tx, merchantID, "service_card", code, sc.ID, "failed",
			"service card already fully used", orderID, userID)
		tx.Commit()
		return nil, apperrors.NewValidationError("service card already fully used")
	}
	if sc.Status == "expired" {
		s.recordVerification(ctx, tx, merchantID, "service_card", code, sc.ID, "failed",
			"service card expired", orderID, userID)
		tx.Commit()
		return nil, apperrors.NewValidationError("service card has expired")
	}
	if sc.RemainingUses <= 0 {
		s.recordVerification(ctx, tx, merchantID, "service_card", code, sc.ID, "failed",
			"service card has no remaining uses", orderID, userID)
		tx.Commit()
		return nil, apperrors.NewValidationError("service card has no remaining uses")
	}

	// Deduct one use.
	newUsed := sc.UsedCount + 1
	newRemaining := sc.RemainingUses - 1
	newStatus := "active"
	if newRemaining <= 0 {
		newStatus = "used"
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE service_cards SET used_count = $1, remaining_uses = $2,
		 status = $3, updated_at = NOW()
		 WHERE id = $4`,
		newUsed, newRemaining, newStatus, sc.ID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update service card", err)
	}

	sc.UsedCount = newUsed
	sc.RemainingUses = newRemaining
	sc.Status = newStatus

	detail := "service card used: " + strconv.Itoa(newUsed) + "/" + strconv.Itoa(sc.TotalUses) +
		" (" + strconv.Itoa(sc.RemainingUses) + " remaining)"
	s.recordVerification(ctx, tx, merchantID, "service_card", code, sc.ID, "success",
		detail, orderID, userID)

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit", err)
	}

	return &sc, nil
}

// ListRecords returns verification records with optional filtering and pagination.
func (s *Service) ListRecords(ctx context.Context, merchantID int64, params ListParams) (*ListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}

	var conditions []string
	var args []interface{}
	conditions = append(conditions, "merchant_id = $1")
	args = append(args, merchantID)
	argIdx := 2

	if params.VerificationType != "" {
		conditions = append(conditions, "verification_type = $"+strconv.Itoa(argIdx))
		args = append(args, params.VerificationType)
		argIdx++
	}
	if params.Code != "" {
		conditions = append(conditions, "code = $"+strconv.Itoa(argIdx))
		args = append(args, params.Code)
		argIdx++
	}

	whereClause := ""
	for i, c := range conditions {
		if i > 0 {
			whereClause += " AND "
		}
		whereClause += c
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM verification_records WHERE `+whereClause,
		args...,
	).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count records", err)
	}

	offset := (params.Page - 1) * params.PageSize
	queryArgs := append(args, params.PageSize, offset)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, merchant_id, verification_type, code, reference_id, result,
		        COALESCE(detail, ''), order_id, verified_by, verified_at, created_at
		 FROM verification_records
		 WHERE `+whereClause+
			` ORDER BY verified_at DESC LIMIT $`+strconv.Itoa(argIdx)+` OFFSET $`+strconv.Itoa(argIdx+1),
		queryArgs...,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query records", err)
	}
	defer rows.Close()

	var records []VerificationRecord
	for rows.Next() {
		var r VerificationRecord
		var verifiedAt, createdAt time.Time
		if err := rows.Scan(&r.ID, &r.MerchantID, &r.VerificationType, &r.Code, &r.ReferenceID,
			&r.Result, &r.Detail, &r.OrderID, &r.VerifiedBy, &verifiedAt, &createdAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan record", err)
		}
		r.VerifiedAt = verifiedAt.Format(time.RFC3339)
		r.CreatedAt = createdAt.Format(time.RFC3339)
		records = append(records, r)
	}
	if records == nil {
		records = []VerificationRecord{}
	}

	return &ListResult{
		Records:  records,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, rows.Err()
}

// recordVerification inserts a verification log entry within a transaction.
func (s *Service) recordVerification(ctx context.Context, tx *sql.Tx, merchantID int64,
	verificationType, code string, referenceID int64, result, detail string,
	orderID *int64, verifiedBy int64) {
	_, _ = tx.ExecContext(ctx,
		`INSERT INTO verification_records (merchant_id, verification_type, code, reference_id,
		 result, detail, order_id, verified_by, verified_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())`,
		merchantID, verificationType, code, referenceID, result, detail, orderID, verifiedBy,
	)
}

func strPtr(s string) *string {
	return &s
}
