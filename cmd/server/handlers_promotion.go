package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/internal/promotion"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// POST /api/v1/merchant/promotions
func makePromotionCreateHandler(svc *promotion.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		var req promotion.CreateActivityRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		a, err := svc.CreateActivity(r.Context(), *claims.MerchantID, req)
		if err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(a)
	}
}

// GET /api/v1/merchant/promotions
func makePromotionListHandler(svc *promotion.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

		params := promotion.ListParams{
			Type:     r.URL.Query().Get("type"),
			Status:   r.URL.Query().Get("status"),
			Page:     page,
			PageSize: pageSize,
		}

		res, err := svc.ListActivities(r.Context(), *claims.MerchantID, params)
		if err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
	}
}

// GET /api/v1/merchant/promotions/{id}
func makePromotionGetHandler(svc *promotion.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid id"))
			return
		}

		a, err := svc.GetActivity(r.Context(), *claims.MerchantID, id)
		if err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(a)
	}
}

// PUT /api/v1/merchant/promotions/{id}
func makePromotionUpdateHandler(svc *promotion.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid id"))
			return
		}

		var req promotion.UpdateActivityRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		a, err := svc.UpdateActivity(r.Context(), *claims.MerchantID, id, req)
		if err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(a)
	}
}

// DELETE /api/v1/merchant/promotions/{id}
func makePromotionDeleteHandler(svc *promotion.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid id"))
			return
		}

		if err := svc.DeleteActivity(r.Context(), *claims.MerchantID, id); err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "deleted"})
	}
}

// POST /api/v1/merchant/promotions/{id}/toggle
func makePromotionToggleHandler(svc *promotion.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid id"))
			return
		}

		a, err := svc.ToggleActivity(r.Context(), *claims.MerchantID, id)
		if err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(a)
	}
}

// GET /api/v1/merchant/promotions/stats
func makePromotionStatsHandler(svc *promotion.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		stats, err := svc.GetStats(r.Context(), *claims.MerchantID)
		if err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}

// GET /api/v1/merchant/promotions/active
func makePromotionActiveHandler(svc *promotion.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		promos, err := svc.GetActivePromotions(r.Context(), *claims.MerchantID)
		if err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(promos)
	}
}
