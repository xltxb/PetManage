package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xltxb/PetManage/internal/membertag"
	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// --- Tag CRUD handlers ---

func makeTagCreateHandler(svc *membertag.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		var req membertag.CreateTagRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		tag, err := svc.CreateTag(r.Context(), *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create tag", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(tag)
	}
}

func makeTagListHandler(svc *membertag.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		tags, err := svc.ListTags(r.Context(), *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list tags", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tags":  tags,
			"total": len(tags),
		})
	}
}

func makeTagGetHandler(svc *membertag.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		tagID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || tagID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid tag id"))
			return
		}

		tag, err := svc.GetTag(r.Context(), *claims.MerchantID, tagID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get tag", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tag)
	}
}

func makeTagUpdateHandler(svc *membertag.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		tagID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || tagID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid tag id"))
			return
		}

		var req membertag.UpdateTagRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		tag, err := svc.UpdateTag(r.Context(), *claims.MerchantID, tagID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update tag", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tag)
	}
}

func makeTagDeleteHandler(svc *membertag.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		tagID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || tagID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid tag id"))
			return
		}

		if err := svc.DeleteTag(r.Context(), *claims.MerchantID, tagID); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to delete tag", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "tag deleted"})
	}
}

func makeTagToggleHandler(svc *membertag.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		tagID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || tagID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid tag id"))
			return
		}

		tag, err := svc.ToggleTag(r.Context(), *claims.MerchantID, tagID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to toggle tag", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tag)
	}
}

// --- Member Tag relation handlers ---

func makeMemberAddTagsHandler(svc *membertag.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		memberID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		var req membertag.AddMemberTagRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}
		if len(req.TagIDs) == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("tag_ids is required"))
			return
		}

		tags, err := svc.AddTags(r.Context(), *claims.MerchantID, memberID, req.TagIDs)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to add tags", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tags": tags,
		})
	}
}

func makeMemberRemoveTagHandler(svc *membertag.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		memberID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		tagID, err := strconv.ParseInt(r.PathValue("tagId"), 10, 64)
		if err != nil || tagID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid tag id"))
			return
		}

		if err := svc.RemoveTag(r.Context(), *claims.MerchantID, memberID, tagID); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to remove tag", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "tag removed"})
	}
}

func makeMemberTagsHandler(svc *membertag.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		memberID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		tags, err := svc.GetMemberTags(r.Context(), *claims.MerchantID, memberID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get member tags", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tags": tags,
		})
	}
}

// --- Auto-Tag Rule handlers ---

func makeTagRuleCreateHandler(svc *membertag.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		var req membertag.CreateRuleRequest
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
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create tag rule", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(rule)
	}
}

func makeTagRuleListHandler(svc *membertag.Service) http.HandlerFunc {
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
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list tag rules", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"rules": rules,
			"total": len(rules),
		})
	}
}

func makeTagRuleGetHandler(svc *membertag.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		ruleID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || ruleID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid rule id"))
			return
		}

		rule, err := svc.GetRule(r.Context(), *claims.MerchantID, ruleID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get tag rule", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rule)
	}
}

func makeTagRuleUpdateHandler(svc *membertag.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		ruleID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || ruleID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid rule id"))
			return
		}

		var req membertag.UpdateRuleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		rule, err := svc.UpdateRule(r.Context(), *claims.MerchantID, ruleID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update tag rule", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rule)
	}
}

func makeTagRuleDeleteHandler(svc *membertag.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		ruleID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || ruleID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid rule id"))
			return
		}

		if err := svc.DeleteRule(r.Context(), *claims.MerchantID, ruleID); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to delete tag rule", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "rule deleted"})
	}
}

func makeTagRuleToggleHandler(svc *membertag.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		ruleID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || ruleID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid rule id"))
			return
		}

		rule, err := svc.ToggleRule(r.Context(), *claims.MerchantID, ruleID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to toggle rule", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rule)
	}
}

func makeTagCheckApplyHandler(svc *membertag.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		memberID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		tags, err := svc.CheckAndApplyRules(r.Context(), *claims.MerchantID, memberID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to check and apply rules", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tags": tags,
		})
	}
}
