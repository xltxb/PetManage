package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xltxb/PetManage/internal/memberlevel"
	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// makeMemberLevelCreateHandler handles POST /api/v1/merchant/member-levels
func makeMemberLevelCreateHandler(svc *memberlevel.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		var req memberlevel.CreateLevelRuleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		rule, err := svc.CreateRule(r.Context(), *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create level rule", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rule)
	}
}

// makeMemberLevelListHandler handles GET /api/v1/merchant/member-levels
func makeMemberLevelListHandler(svc *memberlevel.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		rules, err := svc.ListRules(r.Context(), *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list level rules", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"rules": rules,
			"total": len(rules),
		})
	}
}

// makeMemberLevelGetHandler handles GET /api/v1/merchant/member-levels/{id}
func makeMemberLevelGetHandler(svc *memberlevel.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid level rule id"))
			return
		}

		rule, err := svc.GetRule(r.Context(), *claims.MerchantID, id)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get level rule", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rule)
	}
}

// makeMemberLevelUpdateHandler handles PUT /api/v1/merchant/member-levels/{id}
func makeMemberLevelUpdateHandler(svc *memberlevel.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid level rule id"))
			return
		}

		var req memberlevel.UpdateLevelRuleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		rule, err := svc.UpdateRule(r.Context(), *claims.MerchantID, id, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update level rule", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rule)
	}
}

// makeMemberLevelDeleteHandler handles DELETE /api/v1/merchant/member-levels/{id}
func makeMemberLevelDeleteHandler(svc *memberlevel.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid level rule id"))
			return
		}

		if err := svc.DeleteRule(r.Context(), *claims.MerchantID, id); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to delete level rule", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	}
}

// makeMemberLevelToggleHandler handles POST /api/v1/merchant/member-levels/{id}/toggle
func makeMemberLevelToggleHandler(svc *memberlevel.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid level rule id"))
			return
		}

		rule, err := svc.ToggleRuleStatus(r.Context(), *claims.MerchantID, id)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to toggle level rule", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rule)
	}
}

// makeMemberLevelLogsHandler handles GET /api/v1/merchant/members/{id}/level-logs
func makeMemberLevelLogsHandler(svc *memberlevel.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		memberID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		logs, err := svc.GetLevelLogs(r.Context(), *claims.MerchantID, memberID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get level logs", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"logs":  logs,
			"total": len(logs),
		})
	}
}

// makeMemberLevelCheckUpgradeHandler handles POST /api/v1/merchant/members/{id}/check-upgrade
func makeMemberLevelCheckUpgradeHandler(svc *memberlevel.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		memberID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		action := r.URL.Query().Get("action")
		if action == "" {
			action = "upgrade"
		}

		var result interface{}
		switch action {
		case "downgrade":
			log, err := svc.CheckAndDowngrade(r.Context(), *claims.MerchantID, memberID)
			if err != nil {
				if appErr, ok := err.(*apperrors.AppError); ok {
					apperrors.WriteError(w, r, appErr)
					return
				}
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to check downgrade", err))
				return
			}
			result = map[string]interface{}{
				"level_changed": log != nil,
				"log":           log,
			}
		default:
			log, err := svc.CheckAndUpgrade(r.Context(), *claims.MerchantID, memberID)
			if err != nil {
				if appErr, ok := err.(*apperrors.AppError); ok {
					apperrors.WriteError(w, r, appErr)
					return
				}
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to check upgrade", err))
				return
			}
			result = map[string]interface{}{
				"level_changed": log != nil,
				"log":           log,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// makeMemberLevelInfoHandler handles GET /api/v1/merchant/members/{id}/level
func makeMemberLevelInfoHandler(svc *memberlevel.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		memberID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		info, err := svc.GetMemberLevel(r.Context(), *claims.MerchantID, memberID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get member level", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}
}
