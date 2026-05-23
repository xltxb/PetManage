package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xltxb/PetManage/internal/coupon"
	"github.com/xltxb/PetManage/internal/promotion"
	"github.com/xltxb/PetManage/internal/verification"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// =============================================================================
// F074: Marketing & Verification API handlers for Open Platform
// =============================================================================

// POST /api/open/v1/coupons/{id}/claim — member claims a coupon from a template.
func makeOpenCouponClaimHandler(cs *coupon.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		templateID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || templateID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid template id"))
			return
		}

		var req struct {
			MemberID int64 `json:"member_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body: "+err.Error()))
			return
		}
		if req.MemberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("member_id is required"))
			return
		}

		codes, err := cs.IssueCoupons(r.Context(), merchantID, templateID, coupon.IssueRequest{
			MemberIDs: []int64{req.MemberID},
			Count:     1,
		})
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to claim coupon", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    codes[0],
			"message": "coupon claimed successfully",
		})
	}
}

// POST /api/open/v1/coupons/verify — verify a coupon code and return its discount value.
func makeOpenCouponVerifyHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		var req struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body: "+err.Error()))
			return
		}
		if req.Code == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("code is required"))
			return
		}

		var id int64
		var ctype string
		var valueCents int
		var minOrderCents int
		var status string
		err := db.QueryRowContext(r.Context(),
			`SELECT id, type, value_cents, COALESCE(min_order_cents, 0), status
			 FROM coupons WHERE code = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
			req.Code, merchantID,
		).Scan(&id, &ctype, &valueCents, &minOrderCents, &status)
		if err == sql.ErrNoRows {
			apperrors.WriteError(w, r, apperrors.NewValidationError("coupon not found"))
			return
		}
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to query coupon", err))
			return
		}

		if status == "used" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("coupon already used"))
			return
		}
		if status == "expired" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("coupon is expired"))
			return
		}
		if status == "disabled" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("coupon is disabled"))
			return
		}
		if status != "active" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("coupon is not active"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":           true,
			"code":            req.Code,
			"type":            ctype,
			"value_cents":     valueCents,
			"min_order_cents": minOrderCents,
			"message":         "coupon is valid",
		})
	}
}

// GET /api/open/v1/activities — list currently active promotion activities.
func makeOpenActivitiesHandler(ps *promotion.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		activities, err := ps.GetActivePromotions(r.Context(), merchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to query activities", err))
			return
		}
		if activities == nil {
			activities = []promotion.ActivePromotion{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"activities": activities,
		})
	}
}

// POST /api/open/v1/groupon/verify — verify a third-party groupon voucher code.
func makeOpenGrouponVerifyHandler(vs *verification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		var req struct {
			Code   string `json:"code"`
			Source string `json:"source"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body: "+err.Error()))
			return
		}
		if req.Code == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("code is required"))
			return
		}

		// Ensure the voucher belongs to this merchant before verifying.
		voucher, err := vs.VerifyThirdPartyVoucher(r.Context(), merchantID, 0, req.Code, nil)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to verify groupon voucher", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"verified":     true,
			"code":         voucher.Code,
			"source":       voucher.Source,
			"name":         voucher.Name,
			"amount_cents": voucher.AmountCents,
			"message":      "voucher verified successfully",
		})
	}
}
