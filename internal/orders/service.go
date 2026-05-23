package orders

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"github.com/xltxb/PetManage/internal/balance"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Service handles order management and refund logic.
type Service struct {
	db *sql.DB
}

// NewService creates a new order management service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// GetDB returns the underlying database connection.
func (s *Service) GetDB() *sql.DB {
	return s.db
}

// --- Types ---

// OrderListItem is a row in the order list.
type OrderListItem struct {
	ID         int64     `json:"id"`
	MerchantID int64     `json:"merchant_id"`
	MemberID   *int64    `json:"member_id"`
	MemberName string    `json:"member_name"`
	TotalCents int       `json:"total_cents"`
	PaidCents  int       `json:"paid_cents"`
	Status     string    `json:"status"`
	Notes      string    `json:"notes"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// OrderListFilter holds optional filter parameters for listing orders.
type OrderListFilter struct {
	Keyword  string // matches order id (partial) or member name/phone
	Status   string // completed, refunded, partially_refunded
	DateFrom string // YYYY-MM-DD
	DateTo   string // YYYY-MM-DD
	Page     int
	PageSize int
}

// OrderDetail is the full detail of a single order.
type OrderDetail struct {
	ID         int64             `json:"id"`
	MerchantID int64             `json:"merchant_id"`
	MemberID   *int64            `json:"member_id"`
	MemberName string            `json:"member_name"`
	TotalCents int               `json:"total_cents"`
	PaidCents  int               `json:"paid_cents"`
	Status     string            `json:"status"`
	Notes      string            `json:"notes"`
	Items      []OrderItemInfo   `json:"items"`
	Payments   []PaymentInfo     `json:"payments"`
	Refunds    []RefundInfo      `json:"refunds"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// OrderItemInfo is a single line item in an order.
type OrderItemInfo struct {
	ID           int64             `json:"id"`
	ProductID    *int64            `json:"product_id"`
	ProductName  string            `json:"product_name"`
	PriceCents   int               `json:"price_cents"`
	Quantity     int               `json:"quantity"`
	ProductSkuID *int64            `json:"product_sku_id"`
	SkuSpecInfo  map[string]string `json:"sku_spec_info,omitempty"`
	ServiceItemID *int64           `json:"service_item_id"`
}

// PaymentInfo is a payment record in an order.
type PaymentInfo struct {
	ID         int64     `json:"id"`
	OrderID    int64     `json:"order_id"`
	Method     string    `json:"method"`
	AmountCents int      `json:"amount_cents"`
	CreatedAt  time.Time `json:"created_at"`
}

// RefundInfo is a refund record.
type RefundInfo struct {
	ID           int64            `json:"id"`
	OrderID      int64            `json:"order_id"`
	RefundType   string           `json:"refund_type"`
	Reason       string           `json:"reason"`
	AmountCents  int              `json:"amount_cents"`
	Status       string           `json:"status"`
	Items        []RefundItemInfo `json:"items,omitempty"`
	RequestedBy  int64            `json:"requested_by"`
	ApprovedBy   *int64           `json:"approved_by"`
	CreatedAt    time.Time        `json:"created_at"`
}

// RefundItemInfo is a line item within a refund.
type RefundItemInfo struct {
	ID          int64 `json:"id"`
	OrderItemID int64 `json:"order_item_id"`
	Quantity    int   `json:"quantity"`
	AmountCents int   `json:"amount_cents"`
}

// RefundRequest is the input for creating a refund.
type RefundRequest struct {
	RefundType string            `json:"refund_type"` // "full" or "partial"
	Reason     string            `json:"reason"`
	Items      []RefundItemInput `json:"items,omitempty"` // required for partial
}

// RefundItemInput specifies which order item and how many to refund.
type RefundItemInput struct {
	OrderItemID int64 `json:"order_item_id"`
	Quantity    int   `json:"quantity"`
}

// RefundResponse is returned after a refund is processed.
type RefundResponse struct {
	RefundID    int64  `json:"refund_id"`
	OrderID     int64  `json:"order_id"`
	AmountCents int    `json:"amount_cents"`
	Status      string `json:"status"`
	NeedsApproval bool `json:"needs_approval,omitempty"`
}

// --- Order Listing ---

// ListOrders returns orders for a merchant with optional filters.
func (s *Service) ListOrders(ctx context.Context, merchantID int64, filter OrderListFilter) ([]OrderListItem, int, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > 100 {
		filter.PageSize = 20
	}

	where := "WHERE o.merchant_id = $1"
	args := []interface{}{merchantID}
	argIdx := 2

	if filter.Keyword != "" {
		// Try match as order ID or member name/phone.
		if id, err := strconv.ParseInt(filter.Keyword, 10, 64); err == nil {
			where += " AND o.id = $" + strconv.Itoa(argIdx)
			args = append(args, id)
			argIdx++
		} else {
			where += " AND (m.name ILIKE $" + strconv.Itoa(argIdx) + " OR m.phone ILIKE $" + strconv.Itoa(argIdx) + ")"
			args = append(args, "%"+filter.Keyword+"%")
			argIdx++
		}
	}

	if filter.Status != "" {
		where += " AND o.status = $" + strconv.Itoa(argIdx)
		args = append(args, filter.Status)
		argIdx++
	}

	if filter.DateFrom != "" {
		where += " AND o.created_at >= $" + strconv.Itoa(argIdx)
		args = append(args, filter.DateFrom+" 00:00:00")
		argIdx++
	}

	if filter.DateTo != "" {
		where += " AND o.created_at <= $" + strconv.Itoa(argIdx)
		args = append(args, filter.DateTo+" 23:59:59")
		argIdx++
	}

	// Count total.
	var total int
	countQuery := `SELECT COUNT(*) FROM orders o
		LEFT JOIN members m ON m.id = o.member_id AND m.deleted_at IS NULL
		` + where
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, apperrors.NewInternalError("failed to count orders", err)
	}

	// Query page.
	offset := (filter.Page - 1) * filter.PageSize
	query := `SELECT o.id, o.merchant_id, o.member_id,
		COALESCE(m.name, ''), o.total_cents, o.paid_cents, o.status,
		COALESCE(o.notes, ''), o.created_at, o.updated_at
		FROM orders o
		LEFT JOIN members m ON m.id = o.member_id AND m.deleted_at IS NULL
		` + where + `
		ORDER BY o.created_at DESC
		LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, filter.PageSize, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, apperrors.NewInternalError("failed to query orders", err)
	}
	defer rows.Close()

	var items []OrderListItem
	for rows.Next() {
		var it OrderListItem
		if err := rows.Scan(&it.ID, &it.MerchantID, &it.MemberID,
			&it.MemberName, &it.TotalCents, &it.PaidCents, &it.Status,
			&it.Notes, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, 0, apperrors.NewInternalError("failed to scan order", err)
		}
		items = append(items, it)
	}
	if items == nil {
		items = []OrderListItem{}
	}

	return items, total, nil
}

// --- Order Detail ---

// GetOrderDetail returns full detail for a single order.
func (s *Service) GetOrderDetail(ctx context.Context, merchantID, orderID int64) (*OrderDetail, error) {
	var d OrderDetail
	err := s.db.QueryRowContext(ctx,
		`SELECT o.id, o.merchant_id, o.member_id, COALESCE(m.name, ''),
		 o.total_cents, o.paid_cents, o.status, COALESCE(o.notes, ''),
		 o.created_at, o.updated_at
		 FROM orders o
		 LEFT JOIN members m ON m.id = o.member_id AND m.deleted_at IS NULL
		 WHERE o.id = $1 AND o.merchant_id = $2`,
		orderID, merchantID,
	).Scan(&d.ID, &d.MerchantID, &d.MemberID, &d.MemberName,
		&d.TotalCents, &d.PaidCents, &d.Status, &d.Notes,
		&d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("order not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query order", err)
	}

	// Load items.
	itemRows, err := s.db.QueryContext(ctx,
		`SELECT id, product_id, product_name, price_cents, quantity,
		 product_sku_id, sku_spec_info, service_item_id
		 FROM order_items WHERE order_id = $1 ORDER BY id`, orderID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query order items", err)
	}
	defer itemRows.Close()

	for itemRows.Next() {
		var oi OrderItemInfo
		var skuSpecJSON []byte
		if err := itemRows.Scan(&oi.ID, &oi.ProductID, &oi.ProductName,
			&oi.PriceCents, &oi.Quantity, &oi.ProductSkuID, &skuSpecJSON,
			&oi.ServiceItemID); err != nil {
			return nil, apperrors.NewInternalError("failed to scan order item", err)
		}
		if skuSpecJSON != nil {
			json.Unmarshal(skuSpecJSON, &oi.SkuSpecInfo)
		}
		d.Items = append(d.Items, oi)
	}
	if d.Items == nil {
		d.Items = []OrderItemInfo{}
	}

	// Load payments.
	payRows, err := s.db.QueryContext(ctx,
		`SELECT id, order_id, method, amount_cents, created_at
		 FROM payments WHERE order_id = $1 ORDER BY id`, orderID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query payments", err)
	}
	defer payRows.Close()

	for payRows.Next() {
		var pi PaymentInfo
		if err := payRows.Scan(&pi.ID, &pi.OrderID, &pi.Method, &pi.AmountCents, &pi.CreatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan payment", err)
		}
		d.Payments = append(d.Payments, pi)
	}
	if d.Payments == nil {
		d.Payments = []PaymentInfo{}
	}

	// Load refunds.
	refRows, err := s.db.QueryContext(ctx,
		`SELECT id, order_id, refund_type, reason, amount_cents, status,
		 requested_by, approved_by, created_at
		 FROM refunds WHERE order_id = $1 ORDER BY id`, orderID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query refunds", err)
	}
	defer refRows.Close()

	for refRows.Next() {
		var ri RefundInfo
		if err := refRows.Scan(&ri.ID, &ri.OrderID, &ri.RefundType, &ri.Reason,
			&ri.AmountCents, &ri.Status, &ri.RequestedBy, &ri.ApprovedBy, &ri.CreatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan refund", err)
		}
		d.Refunds = append(d.Refunds, ri)
	}
	if d.Refunds == nil {
		d.Refunds = []RefundInfo{}
	}

	return &d, nil
}

// --- Refund ---

const largeRefundThreshold = 50001 // ¥500.01 → needs approval (50000 cents = ¥500 is the boundary, >500 needs approval)

// RefundOrder processes a full or partial refund.
func (s *Service) RefundOrder(ctx context.Context, merchantID, orderID, userID int64, req RefundRequest) (*RefundResponse, error) {
	if req.RefundType != "full" && req.RefundType != "partial" {
		return nil, apperrors.NewValidationError("refund_type must be 'full' or 'partial'")
	}
	if req.RefundType == "partial" && len(req.Items) == 0 {
		return nil, apperrors.NewValidationError("partial refund requires at least one item")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	// Lock order and verify.
	var memberID sql.NullInt64
	var currentStatus string
	var totalCents int
	err = tx.QueryRowContext(ctx,
		`SELECT member_id, total_cents, status FROM orders
		 WHERE id = $1 AND merchant_id = $2
		 FOR UPDATE`,
		orderID, merchantID,
	).Scan(&memberID, &totalCents, &currentStatus)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("order not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query order", err)
	}

	if currentStatus != "completed" && currentStatus != "partially_refunded" {
		return nil, apperrors.NewValidationError("order status '" + currentStatus + "' cannot be refunded")
	}

	// Collect items to refund.
	type refundTarget struct {
		orderItemID  int64
		productID    *int64
		productSkuID *int64
		quantity     int
		priceCents   int
	}

	var targets []refundTarget
	var refundAmountCents int

	if req.RefundType == "full" {
		rows, err := tx.QueryContext(ctx,
			`SELECT id, product_id, product_sku_id, quantity, price_cents
			 FROM order_items WHERE order_id = $1`, orderID)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to query order items", err)
		}

		for rows.Next() {
			var id, qty, price int64
			var pid, skuID sql.NullInt64
			if err := rows.Scan(&id, &pid, &skuID, &qty, &price); err != nil {
				rows.Close()
				return nil, apperrors.NewInternalError("failed to scan order item", err)
			}
			target := refundTarget{
				orderItemID:  id,
				quantity:     int(qty),
				priceCents:   int(price),
			}
			if pid.Valid {
				target.productID = &pid.Int64
			}
			if skuID.Valid {
				target.productSkuID = &skuID.Int64
			}
			refundAmountCents += int(price) * int(qty)
			targets = append(targets, target)
		}
		rows.Close()

		if len(targets) == 0 {
			return nil, apperrors.NewValidationError("order has no items")
		}
	} else {
		// Partial refund: validate each requested item.
		for _, item := range req.Items {
			if item.Quantity <= 0 {
				return nil, apperrors.NewValidationError("refund quantity must be positive")
			}

			var pid, skuID sql.NullInt64
			var qty, price int64
			err := tx.QueryRowContext(ctx,
				`SELECT product_id, product_sku_id, quantity, price_cents
				 FROM order_items WHERE id = $1 AND order_id = $2
				 FOR UPDATE`,
				item.OrderItemID, orderID,
			).Scan(&pid, &skuID, &qty, &price)
			if err == sql.ErrNoRows {
				return nil, apperrors.NewNotFoundError("order item not found: " + strconv.FormatInt(item.OrderItemID, 10))
			}
			if err != nil {
				return nil, apperrors.NewInternalError("failed to query order item", err)
			}

			// Check how many already refunded for this item.
			var alreadyRefunded int64
			tx.QueryRowContext(ctx,
				`SELECT COALESCE(SUM(ri.quantity), 0)
				 FROM refund_items ri
				 JOIN refunds r ON r.id = ri.refund_id
				 WHERE ri.order_item_id = $1 AND r.order_id = $2 AND r.status != 'rejected'`,
				item.OrderItemID, orderID,
			).Scan(&alreadyRefunded)

			availableQty := int(qty) - int(alreadyRefunded)
			if item.Quantity > availableQty {
				return nil, apperrors.NewValidationError("refund quantity " + strconv.Itoa(item.Quantity) + " exceeds available " + strconv.Itoa(availableQty) + " for item " + strconv.FormatInt(item.OrderItemID, 10))
			}

			target := refundTarget{
				orderItemID: item.OrderItemID,
				quantity:    item.Quantity,
				priceCents:  int(price),
			}
			if pid.Valid {
				target.productID = &pid.Int64
			}
			if skuID.Valid {
				target.productSkuID = &skuID.Int64
			}
			refundAmountCents += int(price) * item.Quantity
			targets = append(targets, target)
		}
	}

	if refundAmountCents <= 0 {
		return nil, apperrors.NewValidationError("refund amount must be positive")
	}

	// Determine refund status based on amount.
	refundStatus := "completed"
	needsApproval := false
	if refundAmountCents >= largeRefundThreshold {
		refundStatus = "pending_approval"
		needsApproval = true
	}

	// Restore inventory for each target.
	for _, t := range targets {
		if t.productSkuID != nil && *t.productSkuID > 0 {
			_, err = tx.ExecContext(ctx,
				`UPDATE product_skus SET stock = stock + $1, updated_at = NOW() WHERE id = $2`,
				t.quantity, *t.productSkuID)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to restore SKU stock", err)
			}
			_, err = tx.ExecContext(ctx,
				`INSERT INTO stock_flows (merchant_id, product_id, product_sku_id, order_id, type, quantity_change)
				 VALUES ($1, $2, $3, $4, 'inbound', $5)`,
				merchantID, t.productID, *t.productSkuID, orderID, t.quantity)
		} else if t.productID != nil && *t.productID > 0 {
			_, err = tx.ExecContext(ctx,
				`UPDATE products SET stock = stock + $1, updated_at = NOW() WHERE id = $2`,
				t.quantity, *t.productID)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to restore product stock", err)
			}
			_, err = tx.ExecContext(ctx,
				`INSERT INTO stock_flows (merchant_id, product_id, order_id, type, quantity_change)
				 VALUES ($1, $2, $3, 'inbound', $4)`,
				merchantID, *t.productID, orderID, t.quantity)
		}
		if err != nil {
			return nil, apperrors.NewInternalError("failed to record stock flow", err)
		}
	}

	// Return payments if the refund is for the full remaining amount or we're in completed status.
	if !needsApproval {
		if err := s.returnPayments(ctx, tx, orderID, memberID, merchantID, refundAmountCents, userID); err != nil {
			return nil, err
		}
	}

	// Create refund record.
	var refundID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO refunds (order_id, merchant_id, refund_type, reason, amount_cents, status, requested_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		orderID, merchantID, req.RefundType, req.Reason, refundAmountCents, refundStatus, userID,
	).Scan(&refundID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create refund record", err)
	}

	// Create refund items.
	for _, t := range targets {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO refund_items (refund_id, order_item_id, quantity, amount_cents)
			 VALUES ($1, $2, $3, $4)`,
			refundID, t.orderItemID, t.quantity, t.priceCents*t.quantity)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to create refund item", err)
		}
	}

	// Update order status (only if no approval is needed).
	if !needsApproval {
	newStatus := "refunded"
	if req.RefundType == "partial" {
		newStatus = "partially_refunded"
	}
	// Check if all items are now refunded (full refund via partial multiple times can lead to fully refunded).
	if req.RefundType != "full" {
		var remainingCents int64
		tx.QueryRowContext(ctx,
			`SELECT total_cents - COALESCE(
			 (SELECT SUM(ri.amount_cents) FROM refund_items ri
			  JOIN refunds r ON r.id = ri.refund_id
			  WHERE r.order_id = $1 AND r.status != 'rejected'), 0
			) FROM orders WHERE id = $1`, orderID).Scan(&remainingCents)
		if remainingCents <= 0 {
			newStatus = "refunded"
		}
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2`,
		newStatus, orderID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update order status", err)
	}
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit refund", err)
	}

	return &RefundResponse{
		RefundID:      refundID,
		OrderID:       orderID,
		AmountCents:   refundAmountCents,
		Status:        refundStatus,
		NeedsApproval: needsApproval,
	}, nil
}

// returnPayments handles returning money/points/coupons for a refund.
func (s *Service) returnPayments(ctx context.Context, tx *sql.Tx, orderID int64, memberID sql.NullInt64, merchantID int64, refundAmount int, operatorID int64) error {
	mid := int64(0)
	if memberID.Valid {
		mid = memberID.Int64
	}

	// Query payments for this order.
	rows, err := tx.QueryContext(ctx,
		`SELECT id, method, amount_cents FROM payments WHERE order_id = $1 ORDER BY id`,
		orderID)
	if err != nil {
		return apperrors.NewInternalError("failed to query payments", err)
	}
	defer rows.Close()

	type pay struct {
		id         int64
		method     string
		amountCents int
	}
	var payments []pay
	for rows.Next() {
		var p pay
		if err := rows.Scan(&p.id, &p.method, &p.amountCents); err != nil {
			return apperrors.NewInternalError("failed to scan payment", err)
		}
		payments = append(payments, p)
	}

	if len(payments) == 0 {
		return nil
	}

	// For full refund, we return all payments proportionally.
	// For partial refund, we return proportionally up to the refund amount.
	remainingRefund := refundAmount

	type paymentReturn struct {
		method string
		amount int
	}
	var returns []paymentReturn

	// Calculate total paid.
	totalPaid := 0
	for _, p := range payments {
		totalPaid += p.amountCents
	}

	// Refund proportionally for each payment method.
	for _, p := range payments {
		if remainingRefund <= 0 {
			break
		}
		proportion := refundAmount * p.amountCents / totalPaid
		if proportion <= 0 {
			proportion = p.amountCents
		}
		if proportion > remainingRefund {
			proportion = remainingRefund
		}
		returns = append(returns, paymentReturn{method: p.method, amount: proportion})
		remainingRefund -= proportion
	}

	// Actually return funds.
	for _, r := range returns {
		if r.amount <= 0 {
			continue
		}
		switch r.method {
		case "balance":
			if mid > 0 {
				_, err := balance.RefundBalance(ctx, tx, mid, merchantID, int64(r.amount), operatorID)
				if err != nil {
					return err
				}
			}
		case "points":
			if mid > 0 {
				_, err := tx.ExecContext(ctx,
					`UPDATE members SET points = points + $1, updated_at = NOW() WHERE id = $2`,
					r.amount, mid)
				if err != nil {
					return apperrors.NewInternalError("failed to return points", err)
				}
			}
		case "coupon":
			// Restore coupon to active status.
			_, _ = tx.ExecContext(ctx,
				`UPDATE coupons SET status = 'active', used_at = NULL,
				 used_by_member_id = NULL, used_order_id = NULL, updated_at = NOW()
				 WHERE used_order_id = $1 AND status = 'used'`,
				orderID)
		}
		// cash, wechat, alipay: no system return (handled externally)
	}

	return nil
}
