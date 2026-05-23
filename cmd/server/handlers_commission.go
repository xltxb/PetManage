package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xltxb/PetManage/internal/commission"
	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// --- Commission rules handlers ---

func makeCommissionRulesGetHandler(svc *commission.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		rules, err := svc.GetRules(r.Context(), *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get commission rules", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rules)
	}
}

func makeCommissionRulesUpdateHandler(svc *commission.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		var req commission.UpdateRuleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		rules, err := svc.UpdateRules(r.Context(), *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update commission rules", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rules)
	}
}

// --- Assign technician handler ---

func makeCommissionAssignHandler(svc *commission.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		var req commission.AssignTechnicianRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		if req.OrderItemID <= 0 || req.EmployeeID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("order_item_id and employee_id are required"))
			return
		}

		record, err := svc.AssignTechnician(r.Context(), *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to assign technician", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(record)
	}
}

// --- Commission records list handler ---

func makeCommissionRecordsHandler(svc *commission.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		params := commission.ListRecordsParams{}
		q := r.URL.Query()

		if v := q.Get("employee_id"); v != "" {
			params.EmployeeID, _ = strconv.ParseInt(v, 10, 64)
		}
		if v := q.Get("order_id"); v != "" {
			params.OrderID, _ = strconv.ParseInt(v, 10, 64)
		}
		params.ItemType = q.Get("item_type")
		params.Status = q.Get("status")
		params.StartDate = q.Get("start_date")
		params.EndDate = q.Get("end_date")
		if v := q.Get("page"); v != "" {
			params.Page, _ = strconv.Atoi(v)
		}
		if v := q.Get("page_size"); v != "" {
			params.PageSize, _ = strconv.Atoi(v)
		}

		result, err := svc.ListRecords(r.Context(), *claims.MerchantID, params)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list commission records", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// --- Monthly commission summary handler ---

func makeCommissionSummaryHandler(svc *commission.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		year, _ := strconv.Atoi(r.URL.Query().Get("year"))
		month, _ := strconv.Atoi(r.URL.Query().Get("month"))

		if year <= 0 || month <= 0 || month > 12 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("year and month (1-12) query parameters are required"))
			return
		}

		summaries, err := svc.GetMonthlySummary(r.Context(), *claims.MerchantID, year, month)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get commission summary", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"year":      year,
			"month":     month,
			"summaries": summaries,
		})
	}
}

// --- Commission deduction handler (for refund) ---

func makeCommissionDeductHandler(svc *commission.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		var body struct {
			OrderItemID int64 `json:"order_item_id"`
			RefundID    int64 `json:"refund_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		if body.OrderItemID <= 0 || body.RefundID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("order_item_id and refund_id are required"))
			return
		}

		if err := svc.DeductCommission(r.Context(), *claims.MerchantID, body.OrderItemID, body.RefundID); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to deduct commission", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "commission deducted"})
	}
}
