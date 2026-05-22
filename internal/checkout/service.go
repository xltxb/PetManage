package checkout

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// CheckoutItem represents a single item in the checkout (product or service).
type CheckoutItem struct {
	ProductID     *int64 `json:"product_id,omitempty"`
	SkuID         *int64 `json:"sku_id,omitempty"`
	ServiceItemID *int64 `json:"service_item_id,omitempty"`
	Quantity      int    `json:"quantity"`
}

// CheckoutPayment represents a payment in the checkout.
type CheckoutPayment struct {
	Method        string `json:"method"`
	AmountCents   int    `json:"amount_cents"`
	ReceivedCents int    `json:"received_cents,omitempty"` // for cash: actual amount received
	CouponCode    string `json:"coupon_code,omitempty"`    // for coupon: code to redeem
}

// CheckoutRequest is the request body for creating an order.
type CheckoutRequest struct {
	MemberID   *int64            `json:"member_id"`
	Items      []CheckoutItem    `json:"items"`
	Payments   []CheckoutPayment `json:"payments"`
	OrderNotes string            `json:"order_notes"`
}

// CheckoutResponse is the response after a successful checkout.
type CheckoutResponse struct {
	OrderID     int64             `json:"order_id"`
	MerchantID  int64             `json:"merchant_id"`
	TotalCents  int               `json:"total_cents"`
	PaidCents   int               `json:"paid_cents"`
	Status      string            `json:"status"`
	Items       []OrderItemDetail `json:"items"`
	Payments    []PaymentDetail   `json:"payments"`
	ChangeCents int               `json:"change_cents"`
	CreatedAt   time.Time         `json:"created_at"`
	OrderNotes  string            `json:"order_notes,omitempty"`
}

// OrderItemDetail is an item in the order response.
type OrderItemDetail struct {
	ProductID     *int64            `json:"product_id,omitempty"`
	ProductName   string            `json:"product_name"`
	SkuID         *int64            `json:"sku_id,omitempty"`
	SkuSpecInfo   map[string]string `json:"sku_spec_info,omitempty"`
	ServiceItemID *int64            `json:"service_item_id,omitempty"`
	ServiceName   string            `json:"service_name,omitempty"`
	PriceCents    int               `json:"price_cents"`
	Quantity      int               `json:"quantity"`
}

// PaymentDetail is a payment in the order response.
type PaymentDetail struct {
	Method        string `json:"method"`
	AmountCents   int    `json:"amount_cents"`
	ReceivedCents int    `json:"received_cents,omitempty"`
	CouponCode    string `json:"coupon_code,omitempty"`
	CouponLabel   string `json:"coupon_label,omitempty"`
}

// --- Cart calculation types ---

// CartItemInput represents a single item in the cart for price calculation.
type CartItemInput struct {
	ProductID     *int64 `json:"product_id,omitempty"`
	SkuID         *int64 `json:"sku_id,omitempty"`
	ServiceItemID *int64 `json:"service_item_id,omitempty"`
	Quantity      int    `json:"quantity"`
}

// CartItemResult represents a calculated cart item with pricing.
type CartItemResult struct {
	ProductID      *int64            `json:"product_id,omitempty"`
	SkuID          *int64            `json:"sku_id,omitempty"`
	SkuSpecInfo    map[string]string `json:"sku_spec_info,omitempty"`
	ServiceItemID  *int64            `json:"service_item_id,omitempty"`
	Name           string            `json:"name"`
	Barcode        string            `json:"barcode,omitempty"`
	UnitPriceCents int               `json:"unit_price_cents"`
	DiscountCents  int               `json:"discount_cents"`
	Quantity       int               `json:"quantity"`
	LineTotalCents int               `json:"line_total_cents"`
}

// CartCalculateRequest is the request body for cart calculation.
type CartCalculateRequest struct {
	MemberID *int64          `json:"member_id"`
	Items    []CartItemInput `json:"items"`
}

// CartCalculateResponse is the response for cart calculation.
type CartCalculateResponse struct {
	Items               []CartItemResult `json:"items"`
	OriginalCents       int              `json:"original_cents"`
	DiscountCents       int              `json:"discount_cents"`
	PayableCents        int              `json:"payable_cents"`
	MemberBalanceCents  int              `json:"member_balance_cents"`
	MemberPoints        int              `json:"member_points"`
	MaxPointsDeductCents int             `json:"max_points_deduct_cents"`
}

// PointsToCentsRate defines how many points equal 1 cent (100 points = 100 cents = ¥1).
// PointsToCentsRate: 100 points = 100 cents (¥1), meaning 1 point = 1 cent.
const PointsToCentsRate = 1

// MaxPointsRatio is the maximum percentage of the order that can be paid with points (50%).
const MaxPointsRatio = 0.5

// MemberInfo holds basic member identification info.
type MemberInfo struct {
	MemberID     int64  `json:"member_id"`
	CardNo       string `json:"card_no"`
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	Status       string `json:"status"`
	BalanceCents int    `json:"balance_cents"`
	Points       int    `json:"points"`
}

// CouponInfo holds validated coupon information.
type CouponInfo struct {
	ID             int64  `json:"id"`
	Code           string `json:"code"`
	DiscountType   string `json:"discount_type"`
	ValueCents     int    `json:"value_cents"`
	MinOrderCents  int    `json:"min_order_cents"`
	Status         string `json:"status"`
}

// Service provides checkout operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new checkout Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// LookupMember finds a member by QR token or phone.
func (s *Service) LookupMember(ctx context.Context, merchantID int64, phone, qrToken string) (*MemberInfo, error) {
	if phone == "" && qrToken == "" {
		return nil, apperrors.NewValidationError("phone or qr_token is required")
	}

	if qrToken != "" {
		return nil, apperrors.NewValidationError("qr_token lookup not implemented directly; use /merchant/members/qrcode/scan endpoint")
	}

	var m MemberInfo
	var phoneStr sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, card_no, name, phone, status, COALESCE(balance_cents, 0), COALESCE(points, 0)
		 FROM members
		 WHERE merchant_id = $1 AND phone = $2 AND status = 'active' AND deleted_at IS NULL
		 LIMIT 1`,
		merchantID, phone,
	).Scan(&m.MemberID, &m.CardNo, &m.Name, &phoneStr, &m.Status, &m.BalanceCents, &m.Points)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("member not found with phone: " + phone)
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to lookup member", err)
	}
	if phoneStr.Valid {
		m.Phone = phoneStr.String
	}
	return &m, nil
}

// CartCalculate computes the cart pricing with potential member discounts.
func (s *Service) CartCalculate(ctx context.Context, merchantID int64, req CartCalculateRequest) (*CartCalculateResponse, error) {
	if len(req.Items) == 0 {
		return nil, apperrors.NewValidationError("at least one item is required")
	}

	var results []CartItemResult
	originalCents := 0
	discountCents := 0

	for _, item := range req.Items {
		if item.Quantity <= 0 {
			return nil, apperrors.NewValidationError("quantity must be positive")
		}

		var result CartItemResult

		if item.ServiceItemID != nil && *item.ServiceItemID > 0 {
			var name string
			var priceCents, memberPriceCents int
			err := s.db.QueryRowContext(ctx,
				`SELECT name, price_cents, member_price_cents FROM service_items
				 WHERE id = $1 AND merchant_id = $2 AND status = 'active' AND deleted_at IS NULL`,
				*item.ServiceItemID, merchantID,
			).Scan(&name, &priceCents, &memberPriceCents)
			if err == sql.ErrNoRows {
				return nil, apperrors.NewNotFoundError("service item not found or inactive")
			}
			if err != nil {
				return nil, apperrors.NewInternalError("failed to query service item", err)
			}

			effectivePrice := priceCents
			itemDiscount := 0
			if req.MemberID != nil && memberPriceCents > 0 && memberPriceCents < priceCents {
				itemDiscount = (priceCents - memberPriceCents) * item.Quantity
				effectivePrice = memberPriceCents
			}

			result = CartItemResult{
				ServiceItemID:  item.ServiceItemID,
				Name:           name,
				UnitPriceCents: priceCents,
				DiscountCents:  itemDiscount,
				Quantity:       item.Quantity,
				LineTotalCents: effectivePrice * item.Quantity,
			}
		} else if item.SkuID != nil && *item.SkuID > 0 {
			var name string
			var priceCents int
			var specJSON []byte
			var skuSpecInfo map[string]string
			err := s.db.QueryRowContext(ctx,
				`SELECT p.name, ps.price_cents, ps.spec_info
				 FROM product_skus ps
				 JOIN products p ON p.id = ps.product_id
				 WHERE ps.id = $1 AND p.merchant_id = $2 AND ps.status = 'active' AND ps.deleted_at IS NULL
				   AND p.status = 'active' AND p.deleted_at IS NULL`,
				*item.SkuID, merchantID,
			).Scan(&name, &priceCents, &specJSON)
			if err == sql.ErrNoRows {
				return nil, apperrors.NewNotFoundError("SKU not found or inactive")
			}
			if err != nil {
				return nil, apperrors.NewInternalError("failed to query SKU", err)
			}
			if specJSON != nil {
				json.Unmarshal(specJSON, &skuSpecInfo)
			}

			result = CartItemResult{
				SkuID:          item.SkuID,
				SkuSpecInfo:    skuSpecInfo,
				Name:           name,
				UnitPriceCents: priceCents,
				DiscountCents:  0,
				Quantity:       item.Quantity,
				LineTotalCents: priceCents * item.Quantity,
			}
		} else if item.ProductID != nil && *item.ProductID > 0 {
			var name, barcode string
			var priceCents int
			err := s.db.QueryRowContext(ctx,
				`SELECT name, barcode, price_cents FROM products
				 WHERE id = $1 AND merchant_id = $2 AND status = 'active' AND deleted_at IS NULL`,
				*item.ProductID, merchantID,
			).Scan(&name, &barcode, &priceCents)
			if err == sql.ErrNoRows {
				return nil, apperrors.NewNotFoundError("product not found or inactive")
			}
			if err != nil {
				return nil, apperrors.NewInternalError("failed to query product", err)
			}

			result = CartItemResult{
				ProductID:      item.ProductID,
				Name:           name,
				Barcode:        barcode,
				UnitPriceCents: priceCents,
				DiscountCents:  0,
				Quantity:       item.Quantity,
				LineTotalCents: priceCents * item.Quantity,
			}
		} else {
			return nil, apperrors.NewValidationError("each item must have product_id, sku_id, or service_item_id")
		}

		results = append(results, result)
		originalCents += result.UnitPriceCents * result.Quantity
		discountCents += result.DiscountCents
	}

	resp := &CartCalculateResponse{
		Items:         results,
		OriginalCents: originalCents,
		DiscountCents: discountCents,
		PayableCents:  originalCents - discountCents,
	}

	// If member is identified, load balance and points.
	if req.MemberID != nil && *req.MemberID > 0 {
		var balance, points int
		err := s.db.QueryRowContext(ctx,
			`SELECT COALESCE(balance_cents, 0), COALESCE(points, 0) FROM members WHERE id = $1 AND deleted_at IS NULL`,
			*req.MemberID,
		).Scan(&balance, &points)
		if err == nil {
			resp.MemberBalanceCents = balance
			resp.MemberPoints = points
			// Max points deduction: minimum of (total points / rate) and (50% of payable amount)
			maxFromPoints := points / PointsToCentsRate
			maxFromRatio := int(float64(resp.PayableCents) * MaxPointsRatio)
			if maxFromPoints > maxFromRatio {
				resp.MaxPointsDeductCents = maxFromRatio
			} else {
				resp.MaxPointsDeductCents = maxFromPoints
			}
		}
	}

	return resp, nil
}

// VerifyCoupon validates a coupon code and returns its details.
func (s *Service) VerifyCoupon(ctx context.Context, merchantID int64, code string) (*CouponInfo, error) {
	if code == "" {
		return nil, apperrors.NewValidationError("coupon code is required")
	}

	var c CouponInfo
	err := s.db.QueryRowContext(ctx,
		`SELECT id, code, type, value_cents, min_order_cents, status
		 FROM coupons
		 WHERE code = $1 AND merchant_id = $2 AND deleted_at IS NULL
		   AND status = 'active'
		 LIMIT 1`,
		code, merchantID,
	).Scan(&c.ID, &c.Code, &c.DiscountType, &c.ValueCents, &c.MinOrderCents, &c.Status)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("coupon not found or invalid: " + code)
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to verify coupon", err)
	}
	return &c, nil
}

// Checkout creates an order, deducts inventory, records payments and stock flows.
// Supports combined payments: cash (with change), wechat, alipay, balance, points, coupon.
func (s *Service) Checkout(ctx context.Context, merchantID int64, req CheckoutRequest) (*CheckoutResponse, error) {
	if len(req.Items) == 0 {
		return nil, apperrors.NewValidationError("at least one item is required")
	}
	if len(req.Payments) == 0 {
		return nil, apperrors.NewValidationError("at least one payment is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	// Calculate total and validate products/SKUs/service items.
	var totalCents int
	var itemDetails []OrderItemDetail
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			return nil, apperrors.NewValidationError("quantity must be positive")
		}

		var priceCents int
		var detail OrderItemDetail

		if item.ServiceItemID != nil && *item.ServiceItemID > 0 {
			var name string
			var regPrice, memberPrice int
			err := tx.QueryRowContext(ctx,
				`SELECT name, price_cents, member_price_cents FROM service_items
				 WHERE id = $1 AND merchant_id = $2 AND status = 'active' AND deleted_at IS NULL
				 FOR UPDATE`,
				*item.ServiceItemID, merchantID,
			).Scan(&name, &regPrice, &memberPrice)
			if err == sql.ErrNoRows {
				return nil, apperrors.NewNotFoundError("service item not found or inactive: " + strconv.FormatInt(*item.ServiceItemID, 10))
			}
			if err != nil {
				return nil, apperrors.NewInternalError("failed to query service item", err)
			}

			priceCents = regPrice
			if req.MemberID != nil && memberPrice > 0 && memberPrice < regPrice {
				priceCents = memberPrice
			}

			detail = OrderItemDetail{
				ServiceItemID: item.ServiceItemID,
				ProductName:   name,
				ServiceName:   name,
				PriceCents:    priceCents,
				Quantity:      item.Quantity,
			}
		} else if item.SkuID != nil && *item.SkuID > 0 {
			var name string
			var stock int
			var specJSON []byte
			var skuSpecInfo map[string]string
			err := tx.QueryRowContext(ctx,
				`SELECT p.name, ps.price_cents, ps.stock, ps.spec_info
				 FROM product_skus ps
				 JOIN products p ON p.id = ps.product_id
				 WHERE ps.id = $1 AND ps.status = 'active' AND ps.deleted_at IS NULL
				   AND p.merchant_id = $2 AND p.status = 'active' AND p.deleted_at IS NULL
				 FOR UPDATE OF ps`,
				*item.SkuID, merchantID,
			).Scan(&name, &priceCents, &stock, &specJSON)
			if err == sql.ErrNoRows {
				return nil, apperrors.NewNotFoundError("SKU not found or inactive: " + strconv.FormatInt(*item.SkuID, 10))
			}
			if err != nil {
				return nil, apperrors.NewInternalError("failed to query SKU", err)
			}
			if specJSON != nil {
				json.Unmarshal(specJSON, &skuSpecInfo)
			}
			if stock < item.Quantity {
				return nil, apperrors.NewValidationError("insufficient stock for: " + name)
			}

			detail = OrderItemDetail{
				ProductID:   item.ProductID,
				ProductName: name,
				SkuID:       item.SkuID,
				SkuSpecInfo: skuSpecInfo,
				PriceCents:  priceCents,
				Quantity:    item.Quantity,
			}
		} else if item.ProductID != nil && *item.ProductID > 0 {
			var name string
			var stock int
			err := tx.QueryRowContext(ctx,
				`SELECT name, price_cents, stock FROM products
				 WHERE id = $1 AND merchant_id = $2 AND status = 'active' AND deleted_at IS NULL
				 FOR UPDATE`, *item.ProductID, merchantID,
			).Scan(&name, &priceCents, &stock)
			if err == sql.ErrNoRows {
				return nil, apperrors.NewNotFoundError("product not found or inactive: " + strconv.FormatInt(*item.ProductID, 10))
			}
			if err != nil {
				return nil, apperrors.NewInternalError("failed to query product", err)
			}
			if stock < item.Quantity {
				return nil, apperrors.NewValidationError("insufficient stock for: " + name)
			}

			detail = OrderItemDetail{
				ProductID:   item.ProductID,
				ProductName: name,
				PriceCents:  priceCents,
				Quantity:    item.Quantity,
			}
		} else {
			return nil, apperrors.NewValidationError("each item must have product_id, sku_id, or service_item_id")
		}

		totalCents += detail.PriceCents * detail.Quantity
		itemDetails = append(itemDetails, detail)
	}

	// Validate payments total and handle special payment methods.
	var paidCents int
	var changeCents int
	var paymentDetails []PaymentDetail
	couponDeducted := 0

	for i, p := range req.Payments {
		if p.AmountCents < 0 {
			return nil, apperrors.NewValidationError("payment amount must not be negative")
		}

		var detail PaymentDetail

		switch p.Method {
		case "cash":
			if p.AmountCents <= 0 && p.ReceivedCents <= 0 {
				return nil, apperrors.NewValidationError("cash payment amount must be positive")
			}
			// If received_cents is provided and > amount_cents, calculate change.
			if p.ReceivedCents > 0 && p.ReceivedCents > p.AmountCents {
				changeCents += p.ReceivedCents - p.AmountCents
				detail = PaymentDetail{
					Method:        "cash",
					AmountCents:   p.AmountCents,
					ReceivedCents: p.ReceivedCents,
				}
			} else if p.ReceivedCents > 0 {
				detail = PaymentDetail{
					Method:        "cash",
					AmountCents:   p.AmountCents,
					ReceivedCents: p.ReceivedCents,
				}
			} else {
				detail = PaymentDetail{
					Method:      "cash",
					AmountCents: p.AmountCents,
				}
			}
			paidCents += p.AmountCents

		case "wechat", "alipay":
			if p.AmountCents <= 0 {
				return nil, apperrors.NewValidationError(p.Method + " payment amount must be positive")
			}
			detail = PaymentDetail{
				Method:      p.Method,
				AmountCents: p.AmountCents,
			}
			paidCents += p.AmountCents

		case "balance":
			if p.AmountCents <= 0 {
				return nil, apperrors.NewValidationError("balance payment amount must be positive")
			}
			if req.MemberID == nil || *req.MemberID <= 0 {
				return nil, apperrors.NewValidationError("member is required for balance payment")
			}
			// Check member balance.
			var balance int
			err := tx.QueryRowContext(ctx,
				`SELECT COALESCE(balance_cents, 0) FROM members WHERE id = $1 FOR UPDATE`,
				*req.MemberID,
			).Scan(&balance)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to check member balance", err)
			}
			if balance < p.AmountCents {
				return nil, apperrors.NewValidationError("insufficient balance: have " + strconv.Itoa(balance) + ", need " + strconv.Itoa(p.AmountCents))
			}
			// Deduct balance.
			_, err = tx.ExecContext(ctx,
				`UPDATE members SET balance_cents = balance_cents - $1, updated_at = NOW() WHERE id = $2`,
				p.AmountCents, *req.MemberID,
			)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to deduct balance", err)
			}
			detail = PaymentDetail{
				Method:      "balance",
				AmountCents: p.AmountCents,
			}
			paidCents += p.AmountCents

		case "points":
			if p.AmountCents <= 0 {
				return nil, apperrors.NewValidationError("points payment amount must be positive")
			}
			if req.MemberID == nil || *req.MemberID <= 0 {
				return nil, apperrors.NewValidationError("member is required for points payment")
			}
			// 100 points = 100 cents (¥1), so points needed equals amount_cents
			pointsNeeded := p.AmountCents
			// Check member points.
			var points int
			err := tx.QueryRowContext(ctx,
				`SELECT COALESCE(points, 0) FROM members WHERE id = $1 FOR UPDATE`,
				*req.MemberID,
			).Scan(&points)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to check member points", err)
			}
			if points < pointsNeeded {
				return nil, apperrors.NewValidationError("insufficient points: have " + strconv.Itoa(points) + ", need " + strconv.Itoa(pointsNeeded))
			}
			// Deduct points.
			_, err = tx.ExecContext(ctx,
				`UPDATE members SET points = points - $1, updated_at = NOW() WHERE id = $2`,
				pointsNeeded, *req.MemberID,
			)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to deduct points", err)
			}
			detail = PaymentDetail{
				Method:      "points",
				AmountCents: p.AmountCents,
			}
			paidCents += p.AmountCents

		case "coupon":
			if p.CouponCode == "" {
				return nil, apperrors.NewValidationError("coupon code is required for coupon payment")
			}
			// Validate coupon.
			var couponID int64
			var couponValue, minOrder int
			var cType, cStatus string
			err := tx.QueryRowContext(ctx,
				`SELECT id, value_cents, min_order_cents, type, status
				 FROM coupons
				 WHERE code = $1 AND merchant_id = $2 AND deleted_at IS NULL
				   AND status = 'active'
				 FOR UPDATE`,
				p.CouponCode, merchantID,
			).Scan(&couponID, &couponValue, &minOrder, &cType, &cStatus)
			if err == sql.ErrNoRows {
				return nil, apperrors.NewNotFoundError("coupon not found or invalid: " + p.CouponCode)
			}
			if err != nil {
				return nil, apperrors.NewInternalError("failed to validate coupon", err)
			}
			if totalCents < minOrder {
				return nil, apperrors.NewValidationError("order total " + strconv.Itoa(totalCents) + " does not meet coupon minimum " + strconv.Itoa(minOrder))
			}
			// Calculate actual deduction (fixed or percent).
			deduction := couponValue
			if cType == "percent" {
				deduction = totalCents * couponValue / 100
			}
			if deduction > totalCents {
				deduction = totalCents
			}
			couponDeducted += deduction

			// Mark coupon as used.
			_, err = tx.ExecContext(ctx,
				`UPDATE coupons SET status = 'used', used_at = NOW(), used_by_member_id = $1, updated_at = NOW()
				 WHERE id = $2`,
				req.MemberID, couponID,
			)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to mark coupon as used", err)
			}
			detail = PaymentDetail{
				Method:      "coupon",
				AmountCents: deduction,
				CouponCode:  p.CouponCode,
				CouponLabel: p.CouponCode + " (-¥" + strconv.Itoa(deduction) + ")",
			}
			// Coupon reduces the total amount needed; it's not "paid" in monetary terms.
			paidCents += 0 // Coupon doesn't add to paidCents; it reduces totalCents equivalent.

		default:
			return nil, apperrors.NewValidationError("invalid payment method: " + p.Method + ". Valid methods: cash, wechat, alipay, balance, points, coupon")
		}

		paymentDetails = append(paymentDetails, detail)

		// Prevent duplicate coupon usage.
		if p.Method == "coupon" {
			for j := 0; j < i; j++ {
				if req.Payments[j].Method == "coupon" {
					return nil, apperrors.NewValidationError("only one coupon can be used per order")
				}
			}
		}
	}

	// Apply coupon deduction: it reduces the effective total.
	effectiveTotal := totalCents - couponDeducted
	if effectiveTotal < 0 {
		effectiveTotal = 0
	}

	// Verify payments cover the order total (after coupon deduction).
	if paidCents < effectiveTotal {
		return nil, apperrors.NewValidationError("payment amount insufficient: paid " + strconv.Itoa(paidCents) + ", needed " + strconv.Itoa(effectiveTotal))
	}

	// Check for overpayment (cash change is handled per-payment above).
	// Non-cash overpayment is an error.
	nonCashOverpay := paidCents - effectiveTotal
	if nonCashOverpay > 0 {
		return nil, apperrors.NewValidationError("overpayment detected: paid " + strconv.Itoa(paidCents) + " for " + strconv.Itoa(effectiveTotal))
	}

	// Create order.
	var orderID int64
	var orderCreatedAt time.Time
	err = tx.QueryRowContext(ctx,
		`INSERT INTO orders (merchant_id, member_id, total_cents, paid_cents, status, notes)
		 VALUES ($1, $2, $3, $4, 'completed', $5)
		 RETURNING id, created_at`,
		merchantID, req.MemberID, totalCents, paidCents, nullIfEmpty(req.OrderNotes),
	).Scan(&orderID, &orderCreatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create order", err)
	}

	// Link coupon to order if used.
	if couponDeducted > 0 {
		_, _ = tx.ExecContext(ctx,
			`UPDATE coupons SET used_order_id = $1 WHERE code = $2 AND merchant_id = $3`,
			orderID, req.Payments[0].CouponCode, merchantID,
		)
		// Find the coupon payment and set used_order_id.
		for _, p := range req.Payments {
			if p.Method == "coupon" {
				_, _ = tx.ExecContext(ctx,
					`UPDATE coupons SET used_order_id = $1 WHERE code = $2 AND merchant_id = $3 AND used_order_id IS NULL`,
					orderID, p.CouponCode, merchantID,
				)
				break
			}
		}
	}

	// Create order items, deduct inventory, record stock flows.
	for i, item := range req.Items {
		detail := itemDetails[i]

		if item.ServiceItemID != nil && *item.ServiceItemID > 0 {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO order_items (order_id, product_id, product_name, price_cents, quantity, service_item_id)
				 VALUES ($1, $2, $3, $4, $5, $6)`,
				orderID, nil, detail.ProductName, detail.PriceCents, item.Quantity, *item.ServiceItemID,
			)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to create order item", err)
			}
			continue
		}

		var skuSpecJSON []byte
		if detail.SkuSpecInfo != nil {
			skuSpecJSON, _ = json.Marshal(detail.SkuSpecInfo)
		}

		if item.SkuID != nil && *item.SkuID > 0 {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO order_items (order_id, product_id, product_name, price_cents, quantity, product_sku_id, sku_spec_info)
				 SELECT $1, $2, name, $3, $4, $5, $6 FROM products WHERE id = $2`,
				orderID, item.ProductID, detail.PriceCents, item.Quantity, *item.SkuID, skuSpecJSON,
			)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to create order item", err)
			}

			_, err = tx.ExecContext(ctx,
				`UPDATE product_skus SET stock = stock - $1, updated_at = NOW()
				 WHERE id = $2`,
				item.Quantity, *item.SkuID,
			)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to deduct SKU inventory", err)
			}

			_, err = tx.ExecContext(ctx,
				`INSERT INTO stock_flows (merchant_id, product_id, product_sku_id, order_id, type, quantity_change)
				 VALUES ($1, $2, $3, $4, 'sale', $5)`,
				merchantID, *item.ProductID, *item.SkuID, orderID, -item.Quantity,
			)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to record stock flow", err)
			}
		} else if item.ProductID != nil && *item.ProductID > 0 {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO order_items (order_id, product_id, product_name, price_cents, quantity)
				 SELECT $1, $2, name, $3, $4 FROM products WHERE id = $2`,
				orderID, *item.ProductID, detail.PriceCents, item.Quantity,
			)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to create order item", err)
			}

			_, err = tx.ExecContext(ctx,
				`UPDATE products SET stock = stock - $1, updated_at = NOW()
				 WHERE id = $2 AND merchant_id = $3`,
				item.Quantity, *item.ProductID, merchantID,
			)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to deduct inventory", err)
			}

			_, err = tx.ExecContext(ctx,
				`INSERT INTO stock_flows (merchant_id, product_id, order_id, type, quantity_change)
				 VALUES ($1, $2, $3, 'sale', $4)`,
				merchantID, *item.ProductID, orderID, -item.Quantity,
			)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to record stock flow", err)
			}
		}
	}

	// Create payments.
	for _, p := range paymentDetails {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO payments (order_id, method, amount_cents)
			 VALUES ($1, $2, $3)`,
			orderID, p.Method, p.AmountCents,
		)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to create payment", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit transaction", err)
	}

	return &CheckoutResponse{
		OrderID:     orderID,
		MerchantID:  merchantID,
		TotalCents:  totalCents,
		PaidCents:   paidCents,
		Status:      "completed",
		Items:       itemDetails,
		Payments:    paymentDetails,
		ChangeCents: changeCents,
		CreatedAt:   orderCreatedAt,
		OrderNotes:  req.OrderNotes,
	}, nil
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
