package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xltxb/PetManage/internal/revenue"
	"github.com/xltxb/PetManage/pkg/apperrors"
	"github.com/xltxb/PetManage/internal/middleware"
)

func makeRevenueSummaryHandler(svc *revenue.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		q := r.URL.Query()
		startDate := q.Get("start_date")
		endDate := q.Get("end_date")
		groupBy := q.Get("group_by")

		if startDate == "" || endDate == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("start_date and end_date are required"))
			return
		}
		if groupBy == "" {
			groupBy = "day"
		}

		result, err := svc.GetSummary(r.Context(), *claims.MerchantID, startDate, endDate, groupBy)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get revenue summary", err))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeRevenueTransactionsHandler(svc *revenue.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		q := r.URL.Query()

		page, _ := strconv.Atoi(q.Get("page"))
		pageSize, _ := strconv.Atoi(q.Get("page_size"))

		params := revenue.ListTransactionsParams{
			StartDate:     q.Get("start_date"),
			EndDate:       q.Get("end_date"),
			Type:          q.Get("type"),
			PaymentMethod: q.Get("payment_method"),
			Page:          page,
			PageSize:      pageSize,
		}

		result, err := svc.ListTransactions(r.Context(), *claims.MerchantID, params)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list transactions", err))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}
