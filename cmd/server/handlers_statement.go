package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/internal/statement"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

func makeProfitLossHandler(svc *statement.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		year, month, err := parseYearMonth(r)
		if err != nil {
			apperrors.WriteError(w, r, err)
			return
		}

		result, svcErr := svc.GetProfitLoss(r.Context(), *claims.MerchantID, year, month)
		if svcErr != nil {
			if appErr, ok := svcErr.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get profit & loss", svcErr))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeRevenueDetailHandler(svc *statement.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		year, month, err := parseYearMonth(r)
		if err != nil {
			apperrors.WriteError(w, r, err)
			return
		}

		result, svcErr := svc.GetRevenueDetail(r.Context(), *claims.MerchantID, year, month)
		if svcErr != nil {
			if appErr, ok := svcErr.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get revenue detail", svcErr))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeProductSalesHandler(svc *statement.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		year, month, err := parseYearMonth(r)
		if err != nil {
			apperrors.WriteError(w, r, err)
			return
		}

		result, svcErr := svc.GetProductSales(r.Context(), *claims.MerchantID, year, month)
		if svcErr != nil {
			if appErr, ok := svcErr.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get product sales", svcErr))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeServicePerformanceHandler(svc *statement.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		year, month, err := parseYearMonth(r)
		if err != nil {
			apperrors.WriteError(w, r, err)
			return
		}

		result, svcErr := svc.GetServicePerformance(r.Context(), *claims.MerchantID, year, month)
		if svcErr != nil {
			if appErr, ok := svcErr.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get service performance", svcErr))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func parseYearMonth(r *http.Request) (int, int, *apperrors.AppError) {
	q := r.URL.Query()
	yearStr := q.Get("year")
	if yearStr == "" {
		return 0, 0, apperrors.NewValidationError("year is required")
	}
	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 2000 {
		return 0, 0, apperrors.NewValidationError("invalid year")
	}

	month := 0
	monthStr := q.Get("month")
	if monthStr != "" {
		month, err = strconv.Atoi(monthStr)
		if err != nil || month < 0 || month > 12 {
			return 0, 0, apperrors.NewValidationError("invalid month (0-12)")
		}
	}

	return year, month, nil
}
