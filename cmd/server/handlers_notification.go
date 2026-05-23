package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/internal/notification"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// makeNotificationListHandler lists notifications with optional filters.
func makeNotificationListHandler(svc *notification.Service) http.HandlerFunc {
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

		params := notification.ListParams{
			Category: q.Get("category"),
			Status:   q.Get("status"),
			UserType: q.Get("user_type"),
			Page:     page,
			PageSize: pageSize,
		}

		result, err := svc.List(r.Context(), *claims.MerchantID, params)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list notifications", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// makeNotificationMarkReadHandler marks a notification as read.
func makeNotificationMarkReadHandler(svc *notification.Service) http.HandlerFunc {
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

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid notification id"))
			return
		}

		if err := svc.MarkRead(r.Context(), *claims.MerchantID, id); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to mark notification as read", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
		})
	}
}

// makeNotificationSendUpcomingHandler triggers sending upcoming appointment reminders.
func makeNotificationSendUpcomingHandler(svc *notification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		count, err := svc.SendUpcomingReminders(r.Context())
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to send upcoming reminders", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sent_count": count,
		})
	}
}
