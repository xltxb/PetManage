package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xltxb/PetManage/internal/fixedexpense"
	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

func makeCreateFixedExpenseHandler(svc *fixedexpense.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		var req fixedexpense.CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		result, err := svc.Create(r.Context(), *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create fixed expense", err))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(result)
	}
}

func makeListFixedExpensesHandler(svc *fixedexpense.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		q := r.URL.Query()
		params := fixedexpense.ListParams{
			Category: q.Get("category"),
		}
		if p := q.Get("page"); p != "" {
			params.Page, _ = strconv.Atoi(p)
		}
		if ps := q.Get("page_size"); ps != "" {
			params.PageSize, _ = strconv.Atoi(ps)
		}

		result, err := svc.List(r.Context(), *claims.MerchantID, params)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list fixed expenses", err))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeUpdateFixedExpenseHandler(svc *fixedexpense.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		idStr := r.PathValue("id")
		expenseID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid expense id"))
			return
		}

		var req fixedexpense.UpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		result, svcErr := svc.Update(r.Context(), expenseID, *claims.MerchantID, req)
		if svcErr != nil {
			if appErr, ok := svcErr.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update fixed expense", svcErr))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeDeleteFixedExpenseHandler(svc *fixedexpense.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		idStr := r.PathValue("id")
		expenseID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid expense id"))
			return
		}

		if err := svc.Delete(r.Context(), expenseID, *claims.MerchantID); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to delete fixed expense", err))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	}
}
