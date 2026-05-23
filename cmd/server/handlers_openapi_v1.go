package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xltxb/PetManage/internal/dictionary"
	"github.com/xltxb/PetManage/internal/member"
	"github.com/xltxb/PetManage/internal/memberlevel"
	"github.com/xltxb/PetManage/internal/merchant"
	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/internal/openplatform"
	"github.com/xltxb/PetManage/internal/pet"
	"github.com/xltxb/PetManage/internal/product"
	"github.com/xltxb/PetManage/internal/servicemgmt"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// getOpenAPIMerchantID extracts the merchant_id from open platform auth claims.
// Returns 0 if the developer has no associated merchant.
func getOpenAPIMerchantID(r *http.Request) int64 {
	claims := middleware.OpenDevClaimsFromContext(r.Context())
	if claims == nil {
		return 0
	}
	return claims.MerchantID
}

// GET /api/open/v1/shop/info — returns store name, address, business hours, logo.
func makeOpenShopInfoHandler(ms *merchant.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		settings, err := ms.GetShopSettings(r.Context(), merchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get shop info", err))
			return
		}

		// Build logo full URL if logo is set.
		logoURL := settings.LogoURL
		if logoURL != "" && r.Host != "" {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			logoURL = scheme + "://" + r.Host + logoURL
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":           settings.Name,
			"address":        settings.Address,
			"contact_phone":  settings.ContactPhone,
			"contact_email":  settings.ContactEmail,
			"business_hours": settings.BusinessHours,
			"logo_url":       logoURL,
			"notice":         settings.Notice,
		})
	}
}

// GET /api/open/v1/products — product list with pagination and optional category filter.
func makeOpenProductListHandler(ps *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		q := r.URL.Query()

		// Category filter.
		var categoryID *int64
		if catStr := q.Get("category_id"); catStr != "" {
			if v, err := strconv.ParseInt(catStr, 10, 64); err == nil && v > 0 {
				categoryID = &v
			}
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

		params := product.ListParams{
			Status:   "active",
			Page:     page,
			PageSize: pageSize,
		}
		_ = categoryID // category_id filter not built into ListParams; reserved for future filtering

		result, err := ps.List(r.Context(), merchantID, params)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list products", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// GET /api/open/v1/products/{id} — product detail including SKUs.
func makeOpenProductDetailHandler(ps *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		productID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || productID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid product id"))
			return
		}

		detail, err := ps.GetByIDWithSKUs(r.Context(), productID, merchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get product", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(detail)
	}
}

// GET /api/open/v1/services — service item list with price and duration.
func makeOpenServiceListHandler(svc *servicemgmt.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		q := r.URL.Query()
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

		params := servicemgmt.ListItemsParams{
			Status:   "active",
			Page:     page,
			PageSize: pageSize,
		}
		if catStr := q.Get("category_id"); catStr != "" {
			if v, err := strconv.ParseInt(catStr, 10, 64); err == nil && v > 0 {
				params.CategoryID = &v
			}
		}

		result, err := svc.ListItems(r.Context(), merchantID, params)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list services", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// GET /api/open/v1/breeds — pet breed list, optionally filtered by type.
func makeOpenBreedsHandler(ds *dictionary.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			// Breeds are platform-level data, merchant association not strictly required.
		}

		petType := r.URL.Query().Get("type")

		result, err := ds.ListBreeds(r.Context(), petType, merchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list breeds", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// OpenAPIService holds references needed by the open API handlers.
type OpenAPIService struct {
	Merchant     *merchant.Service
	Product      *product.Service
	ServiceMgmt  *servicemgmt.Service
	Dict         *dictionary.Service
	Token        *openplatform.TokenService
	Member       *member.Service
	MemberLevel  *memberlevel.Service
	Pet          *pet.Service
}

// =============================================================================
// F071: Member & Pet API handlers
// =============================================================================

// POST /api/open/v1/members/register — register a new member.
func makeOpenMemberRegisterHandler(ms *member.Service, mls *memberlevel.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		var req member.CreateMemberRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body: "+err.Error()))
			return
		}

		m, err := ms.Create(r.Context(), merchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to register member", err))
			return
		}

		// Assign default level if configured.
		_ = mls.SetDefaultLevel(r.Context(), merchantID, m.ID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(m)
	}
}

// GET /api/open/v1/members/{id} — query member info including level, balance, points.
func makeOpenMemberGetHandler(ms *member.Service, mls *memberlevel.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		memberID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		m, err := ms.GetByID(r.Context(), memberID, merchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get member", err))
			return
		}

		levelInfo, _ := mls.GetMemberLevel(r.Context(), merchantID, memberID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"member": m,
			"level":  levelInfo,
		})
	}
}

// PUT /api/open/v1/members/{id} — update member information.
func makeOpenMemberUpdateHandler(ms *member.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		memberID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		var req member.UpdateMemberRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body: "+err.Error()))
			return
		}

		m, err := ms.Update(r.Context(), memberID, merchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update member", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m)
	}
}

// POST /api/open/v1/members/{id}/pets — add a pet profile to a member.
func makeOpenPetCreateHandler(ps *pet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		memberID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		var req pet.CreatePetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body: "+err.Error()))
			return
		}

		p, err := ps.Create(r.Context(), merchantID, memberID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create pet", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(p)
	}
}

// GET /api/open/v1/members/{id}/pets — list pets for a member.
func makeOpenPetListHandler(ps *pet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		memberID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		pets, err := ps.ListByMember(r.Context(), merchantID, memberID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list pets", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pets": pets,
		})
	}
}
