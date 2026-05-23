package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/internal/orders"
	"github.com/xltxb/PetManage/internal/risk"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// --- Order list handler ---

func makeOrderListHandler(svc *orders.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		pageSize, _ := strconv.Atoi(q.Get("page_size"))

		list, total, err := svc.ListOrders(r.Context(), *claims.MerchantID, orders.OrderListFilter{
			Keyword:  q.Get("keyword"),
			Status:   q.Get("status"),
			DateFrom: q.Get("date_from"),
			DateTo:   q.Get("date_to"),
			Page:     page,
			PageSize: pageSize,
		})
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list orders", err))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"orders":    list,
			"total":     total,
			"page":      max(page, 1),
			"page_size": max(pageSize, 20),
		})
	}
}

// --- Order detail handler ---

func makeOrderDetailHandler(svc *orders.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		orderID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || orderID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid order id"))
			return
		}

		detail, err := svc.GetOrderDetail(r.Context(), *claims.MerchantID, orderID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get order detail", err))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(detail)
	}
}

// --- Refund handler ---

func makeRefundHandler(orderSvc *orders.Service, riskSvc *risk.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		orderID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || orderID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid order id"))
			return
		}

		var req orders.RefundRequest
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
				return
			}
		}
		if req.RefundType == "" {
			req.RefundType = "full"
		}

		result, err := orderSvc.RefundOrder(r.Context(), *claims.MerchantID, orderID, claims.UserID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to process refund", err))
			}
			return
		}

		// Check large refund risk (non-blocking, log only).
		alert, _ := riskSvc.CheckLargeRefund(r.Context(), orderID, *claims.MerchantID, result.AmountCents)

		resp := map[string]interface{}{
			"refund_id":      result.RefundID,
			"order_id":       result.OrderID,
			"amount_cents":   result.AmountCents,
			"status":         result.Status,
			"needs_approval": result.NeedsApproval,
		}
		if alert != nil {
			resp["risk_alert"] = map[string]interface{}{
				"id":          alert.ID,
				"alert_type":  alert.AlertType,
				"description": alert.Description,
				"status":      alert.Status,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
