package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/xltxb/PetManage/internal/checkout"
	"github.com/xltxb/PetManage/internal/orders"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// =============================================================================
// F073: Order & Payment API handlers for Open Platform
// =============================================================================

// OpenCreateOrderRequest is the request body for creating an order via open API.
type OpenCreateOrderRequest struct {
	MemberID *int64                    `json:"member_id"`
	Items    []checkout.CartItemInput  `json:"items"`
	Notes    string                    `json:"notes,omitempty"`
}

// OpenCreateOrderResponse is the response for order creation.
type OpenCreateOrderResponse struct {
	OrderID    int64                       `json:"order_id"`
	OrderNo    string                      `json:"order_no"`
	MerchantID int64                       `json:"merchant_id"`
	MemberID   *int64                      `json:"member_id"`
	TotalCents int                         `json:"total_cents"`
	Status     string                      `json:"status"`
	Items      []checkout.CartItemResult   `json:"items"`
	CreatedAt  time.Time                   `json:"created_at"`
	Notes      string                      `json:"notes,omitempty"`
}

// PayCallbackRequest is the request body for payment callback notification.
type PayCallbackRequest struct {
	Method       string `json:"method"`
	AmountCents  int    `json:"amount_cents"`
	ExternalRef  string `json:"external_ref,omitempty"`
}

// POST /api/open/v1/orders — create a new order, returns order number and payment parameters.
func makeOpenOrderCreateHandler(cs *checkout.Service, os *orders.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		var req OpenCreateOrderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body: "+err.Error()))
			return
		}

		if len(req.Items) == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("at least one item is required"))
			return
		}

		// Calculate cart pricing.
		cartReq := checkout.CartCalculateRequest{
			MemberID: req.MemberID,
			Items:    req.Items,
		}
		cart, err := cs.CartCalculate(r.Context(), merchantID, cartReq)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to calculate order", err))
			return
		}

		// Validate quantities.
		for _, item := range req.Items {
			if item.Quantity <= 0 {
				apperrors.WriteError(w, r, apperrors.NewValidationError("quantity must be positive"))
				return
			}
		}

		// Generate unique order number: OP + YYYYMMDD + 8 hex chars.
		nonce := make([]byte, 4)
		rand.Read(nonce)
		orderNo := fmt.Sprintf("OP%s%s", time.Now().Format("20060102"), hex.EncodeToString(nonce))

		tx, err := os.GetDB().BeginTx(r.Context(), nil)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to begin transaction", err))
			return
		}
		defer tx.Rollback()

		var orderID int64
		var createdAt time.Time
		err = tx.QueryRowContext(r.Context(),
			`INSERT INTO orders (merchant_id, member_id, total_cents, paid_cents, status, notes, order_no)
			 VALUES ($1, $2, $3, 0, 'pending', $4, $5)
			 RETURNING id, created_at`,
			merchantID, req.MemberID, cart.PayableCents, nullIfEmptyStr(req.Notes), orderNo,
		).Scan(&orderID, &createdAt)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create order: "+err.Error(), err))
			return
		}

		// Create order items.
		for _, item := range cart.Items {
			var productID, skuID, serviceItemID interface{}
			productID = nil
			skuID = nil
			serviceItemID = nil

			if item.ProductID != nil && *item.ProductID > 0 {
				productID = *item.ProductID
			}
			if item.SkuID != nil && *item.SkuID > 0 {
				skuID = *item.SkuID
			}
			if item.ServiceItemID != nil && *item.ServiceItemID > 0 {
				serviceItemID = *item.ServiceItemID
			}

			var skuSpecJSON []byte
			if item.SkuSpecInfo != nil {
				skuSpecJSON, _ = json.Marshal(item.SkuSpecInfo)
			}

			_, err = tx.ExecContext(r.Context(),
				`INSERT INTO order_items (order_id, product_id, product_name, price_cents, quantity, product_sku_id, sku_spec_info, service_item_id)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				orderID, productID, item.Name, item.UnitPriceCents, item.Quantity, skuID, skuSpecJSON, serviceItemID,
			)
			if err != nil {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create order item", err))
				return
			}
		}

		if err := tx.Commit(); err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to commit order", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(OpenCreateOrderResponse{
			OrderID:    orderID,
			OrderNo:    orderNo,
			MerchantID: merchantID,
			MemberID:   req.MemberID,
			TotalCents: cart.PayableCents,
			Status:     "pending",
			Items:      cart.Items,
			CreatedAt:  createdAt,
			Notes:      req.Notes,
		})
	}
}

// GET /api/open/v1/orders/{id} — query order detail and payment status.
func makeOpenOrderDetailHandler(os *orders.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		orderID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || orderID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid order id"))
			return
		}

		detail, err := os.GetOrderDetail(r.Context(), merchantID, orderID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get order detail", err))
			return
		}

		// Also fetch order_no from the orders table.
		var orderNo sql.NullString
		os.GetDB().QueryRowContext(r.Context(),
			`SELECT order_no FROM orders WHERE id = $1`, orderID,
		).Scan(&orderNo)

		resp := map[string]interface{}{
			"id":          detail.ID,
			"merchant_id": detail.MerchantID,
			"member_id":   detail.MemberID,
			"member_name": detail.MemberName,
			"total_cents": detail.TotalCents,
			"paid_cents":  detail.PaidCents,
			"status":      detail.Status,
			"notes":       detail.Notes,
			"items":       detail.Items,
			"payments":    detail.Payments,
			"refunds":     detail.Refunds,
			"created_at":  detail.CreatedAt,
			"updated_at":  detail.UpdatedAt,
		}
		if orderNo.Valid {
			resp["order_no"] = orderNo.String
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// POST /api/open/v1/orders/{id}/pay-callback — payment callback notification, order status auto-updates to "paid".
func makeOpenPayCallbackHandler(os *orders.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		orderID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || orderID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid order id"))
			return
		}

		var req PayCallbackRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body: "+err.Error()))
			return
		}

		if req.Method == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("payment method is required"))
			return
		}
		if req.AmountCents <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("payment amount must be positive"))
			return
		}

		validMethods := map[string]bool{"cash": true, "wechat": true, "alipay": true, "balance": true, "points": true, "coupon": true}
		if !validMethods[strings.ToLower(req.Method)] {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid payment method: "+req.Method+". Valid methods: cash, wechat, alipay, balance, points, coupon"))
			return
		}

		tx, err := os.GetDB().BeginTx(r.Context(), nil)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to begin transaction", err))
			return
		}
		defer tx.Rollback()

		// Lock order and verify status.
		var currentStatus string
		var totalCents int
		err = tx.QueryRowContext(r.Context(),
			`SELECT status, total_cents FROM orders WHERE id = $1 AND merchant_id = $2 FOR UPDATE`,
			orderID, merchantID,
		).Scan(&currentStatus, &totalCents)
		if err == sql.ErrNoRows {
			apperrors.WriteError(w, r, apperrors.NewNotFoundError("order not found"))
			return
		}
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to query order", err))
			return
		}

		if currentStatus != "pending" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("order status '" + currentStatus + "' cannot receive payment callback (must be pending)"))
			return
		}

		// Record the payment.
		_, err = tx.ExecContext(r.Context(),
			`INSERT INTO payments (order_id, method, amount_cents)
			 VALUES ($1, $2, $3)`,
			orderID, strings.ToLower(req.Method), req.AmountCents,
		)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to record payment", err))
			return
		}

		// Update order to paid.
		_, err = tx.ExecContext(r.Context(),
			`UPDATE orders SET status = 'paid', paid_cents = $1, updated_at = NOW() WHERE id = $2`,
			req.AmountCents, orderID,
		)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update order status", err))
			return
		}

		if err := tx.Commit(); err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to commit payment callback", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"order_id":     orderID,
			"status":       "paid",
			"method":       strings.ToLower(req.Method),
			"amount_cents": req.AmountCents,
			"message":       "payment received, order updated to paid",
		})
	}
}

// GET /api/open/v1/orders — list orders for a member.
func makeOpenOrderListHandler(os *orders.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		q := r.URL.Query()
		memberIDStr := q.Get("member_id")
		if memberIDStr == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("member_id is required"))
			return
		}

		memberID, err := strconv.ParseInt(memberIDStr, 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member_id"))
			return
		}

		page := 1
		pageSize := 20
		if p := q.Get("page"); p != "" {
			if v, err := strconv.Atoi(p); err == nil && v > 0 {
				page = v
			}
		}
		if ps := q.Get("page_size"); ps != "" {
			if v, err := strconv.Atoi(ps); err == nil && v > 0 && v <= 100 {
				pageSize = v
			}
		}

		list, _, err := os.ListOrders(r.Context(), merchantID, orders.OrderListFilter{
			Page:     page,
			PageSize: pageSize,
		})
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list orders", err))
			return
		}

		// Filter by member_id since ListOrders doesn't have a member_id filter directly.
		var filtered []orders.OrderListItem
		for _, o := range list {
			if o.MemberID != nil && *o.MemberID == memberID {
				filtered = append(filtered, o)
			}
		}

		if filtered == nil {
			filtered = []orders.OrderListItem{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"orders":    filtered,
			"total":     len(filtered),
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// POST /api/open/v1/orders/{id}/refund — request a refund for an order.
func makeOpenOrderRefundHandler(os *orders.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		orderID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || orderID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid order id"))
			return
		}

		var req orders.RefundRequest
		if r.Body != nil && r.ContentLength > 0 {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body: "+err.Error()))
				return
			}
		}
		if req.RefundType == "" {
			req.RefundType = "full"
		}

		result, err := os.RefundOrder(r.Context(), merchantID, orderID, 0, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to process refund", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"refund_id":       result.RefundID,
			"order_id":        result.OrderID,
			"amount_cents":    result.AmountCents,
			"status":          result.Status,
			"needs_approval":  result.NeedsApproval,
			"message":         "refund request submitted",
		})
	}
}

func nullIfEmptyStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
