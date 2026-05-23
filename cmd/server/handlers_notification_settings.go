package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/internal/notification"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// makeNotificationSettingsGetHandler returns all notification settings for a merchant.
func makeNotificationSettingsGetHandler(svc *notification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		settings, err := svc.GetSettings(r.Context(), *claims.MerchantID)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get notification settings", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"settings": settings,
		})
	}
}

// makeNotificationSettingsUpdateHandler updates notification settings.
func makeNotificationSettingsUpdateHandler(svc *notification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		var req notification.UpdateSettingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		if err := svc.UpdateSettings(r.Context(), *claims.MerchantID, req); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update notification settings", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
		})
	}
}

// makeNotificationTemplatesGetHandler returns all notification templates for a merchant.
func makeNotificationTemplatesGetHandler(svc *notification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		templates, err := svc.GetTemplates(r.Context(), *claims.MerchantID)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get notification templates", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"templates": templates,
		})
	}
}

// makeNotificationTemplatesUpdateHandler updates notification templates.
func makeNotificationTemplatesUpdateHandler(svc *notification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		var req notification.UpdateTemplateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		if err := svc.UpdateTemplates(r.Context(), *claims.MerchantID, req); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update notification templates", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
		})
	}
}

// makeNotificationBirthdayHandler triggers birthday notifications.
func makeNotificationBirthdayHandler(svc *notification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		count, err := svc.SendBirthdayNotifications(r.Context())
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to send birthday notifications", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sent_count": count,
		})
	}
}

// makeNotificationInventoryAlertHandler triggers inventory alert notifications.
func makeNotificationInventoryAlertHandler(svc *notification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		count, err := svc.SendInventoryAlertNotifications(r.Context(), *claims.MerchantID)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to send inventory alert notifications", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sent_count": count,
		})
	}
}

// makeNotificationSendRecordsHandler queries notification send records with filters.
func makeNotificationSendRecordsHandler(svc *notification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		pageSize, _ := strconv.Atoi(q.Get("page_size"))

		params := notification.SendRecordParams{
			Category: q.Get("category"),
			Channel:  q.Get("channel"),
			Scenario: q.Get("scenario"),
			UserType: q.Get("user_type"),
			Page:     page,
			PageSize: pageSize,
		}

		result, err := svc.GetSendRecords(r.Context(), *claims.MerchantID, params)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list send records", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}
