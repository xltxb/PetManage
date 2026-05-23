package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/internal/servicecard"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// POST /api/v1/merchant/service-cards/templates
func makeSCTemplateCreateHandler(svc *servicecard.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		var req servicecard.CreateTemplateRequest
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

// GET /api/v1/merchant/service-cards/templates
func makeSCTemplateListHandler(svc *servicecard.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

		params := servicecard.ListParams{
			Status:   r.URL.Query().Get("status"),
			Keyword:  r.URL.Query().Get("keyword"),
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

// GET /api/v1/merchant/service-cards/templates/{id}
func makeSCTemplateGetHandler(svc *servicecard.Service) http.HandlerFunc {
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

// PUT /api/v1/merchant/service-cards/templates/{id}
func makeSCTemplateUpdateHandler(svc *servicecard.Service) http.HandlerFunc {
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

		var req servicecard.UpdateTemplateRequest
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

// POST /api/v1/merchant/service-cards/templates/{id}/toggle
func makeSCTemplateToggleHandler(svc *servicecard.Service) http.HandlerFunc {
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

// POST /api/v1/merchant/service-cards/templates/{id}/purchase
func makeSCTemplatePurchaseHandler(svc *servicecard.Service) http.HandlerFunc {
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

		var req servicecard.PurchaseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		card, err := svc.Purchase(r.Context(), *claims.MerchantID, id, req)
		if err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(card)
	}
}

// GET /api/v1/merchant/service-cards/member/{memberId}
func makeSCMemberCardsHandler(svc *servicecard.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		memberID, err := strconv.ParseInt(r.PathValue("memberId"), 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		cards, err := svc.GetMemberCards(r.Context(), *claims.MerchantID, memberID)
		if err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"cards": cards,
			"total": len(cards),
		})
	}
}

// GET /api/v1/merchant/service-cards/{id}/usage-logs
func makeSCUsageLogsHandler(svc *servicecard.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid card id"))
			return
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

		res, err := svc.GetUsageLogs(r.Context(), *claims.MerchantID, id, page, pageSize)
		if err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
	}
}

// GET /api/v1/merchant/service-cards/expiring
func makeSCExpiringHandler(svc *servicecard.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		days, _ := strconv.Atoi(r.URL.Query().Get("days"))
		if days <= 0 {
			days = 30
		}

		res, err := svc.GetExpiringCards(r.Context(), *claims.MerchantID, days)
		if err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
	}
}

// GET /api/v1/merchant/service-cards
func makeSCAllCardsHandler(svc *servicecard.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

		params := servicecard.ListParams{
			Status:   r.URL.Query().Get("status"),
			Keyword:  r.URL.Query().Get("keyword"),
			Page:     page,
			PageSize: pageSize,
		}

		res, err := svc.GetAllCards(r.Context(), *claims.MerchantID, params)
		if err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
	}
}

// GET /api/v1/merchant/service-cards/code
func makeSCCardByCodeHandler(svc *servicecard.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("code parameter is required"))
			return
		}

		card, err := svc.GetCardByCode(r.Context(), *claims.MerchantID, code)
		if err != nil {
			writeAppError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(card)
	}
}
