package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xltxb/PetManage/internal/coupon"
	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// POST /api/v1/merchant/coupons/templates
func makeCouponTemplateCreateHandler(svc *coupon.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		var req coupon.CreateTemplateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		tmpl, err := svc.CreateTemplate(r.Context(), *claims.MerchantID, req)
		if err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(tmpl)
	}
}

// GET /api/v1/merchant/coupons/templates
func makeCouponTemplateListHandler(svc *coupon.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

		params := coupon.TemplateListParams{
			Type:     r.URL.Query().Get("type"),
			Status:   r.URL.Query().Get("status"),
			Page:     page,
			PageSize: pageSize,
		}

		res, err := svc.ListTemplates(r.Context(), *claims.MerchantID, params)
		if err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
	}
}

// GET /api/v1/merchant/coupons/templates/{id}
func makeCouponTemplateGetHandler(svc *coupon.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid template id"))
			return
		}

		tmpl, err := svc.GetTemplate(r.Context(), *claims.MerchantID, id)
		if err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tmpl)
	}
}

// PUT /api/v1/merchant/coupons/templates/{id}
func makeCouponTemplateUpdateHandler(svc *coupon.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid template id"))
			return
		}

		var req coupon.CreateTemplateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		tmpl, err := svc.UpdateTemplate(r.Context(), *claims.MerchantID, id, req)
		if err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tmpl)
	}
}

// POST /api/v1/merchant/coupons/templates/{id}/toggle
func makeCouponTemplateToggleHandler(svc *coupon.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid template id"))
			return
		}

		tmpl, err := svc.ToggleTemplate(r.Context(), *claims.MerchantID, id)
		if err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tmpl)
	}
}

// POST /api/v1/merchant/coupons/templates/{id}/issue
func makeCouponIssueHandler(svc *coupon.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid template id"))
			return
		}

		var req coupon.IssueRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		codes, err := svc.IssueCoupons(r.Context(), *claims.MerchantID, id, req)
		if err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"codes": codes,
			"total": len(codes),
		})
	}
}

// GET /api/v1/merchant/coupons/codes
func makeCouponCodeListHandler(svc *coupon.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		templateID, _ := strconv.ParseInt(r.URL.Query().Get("template_id"), 10, 64)
		memberID, _ := strconv.ParseInt(r.URL.Query().Get("member_id"), 10, 64)

		params := coupon.CodeListParams{
			TemplateID: templateID,
			MemberID:   memberID,
			Status:     r.URL.Query().Get("status"),
			Page:       page,
			PageSize:   pageSize,
		}

		res, err := svc.ListCodes(r.Context(), *claims.MerchantID, params)
		if err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
	}
}

// GET /api/v1/merchant/coupons/stats
func makeCouponStatsHandler(svc *coupon.Service) http.HandlerFunc {
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
		json.NewEncoder(w).Encode(map[string]interface{}{
			"stats": stats,
		})
	}
}
