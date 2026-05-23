package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/internal/review"
	apperrors "github.com/xltxb/PetManage/pkg/apperrors"
)

func makeReviewListHandler(svc *review.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		pageSize, _ := strconv.Atoi(q.Get("page_size"))
		ratingMin, _ := strconv.Atoi(q.Get("rating_min"))
		ratingMax, _ := strconv.Atoi(q.Get("rating_max"))
		employeeID, _ := strconv.ParseInt(q.Get("employee_id"), 10, 64)
		memberID, _ := strconv.ParseInt(q.Get("member_id"), 10, 64)

		var replied *bool
		if q.Get("replied") == "true" {
			t := true
			replied = &t
		} else if q.Get("replied") == "false" {
			f := false
			replied = &f
		}

		result, err := svc.ListReviews(r.Context(), *claims.MerchantID, review.ListParams{
			Page:       page,
			PageSize:   pageSize,
			RatingMin:  ratingMin,
			RatingMax:  ratingMax,
			Replied:    replied,
			EmployeeID: employeeID,
			MemberID:   memberID,
			DateFrom:   q.Get("date_from"),
			DateTo:     q.Get("date_to"),
		})
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list reviews", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeReviewGetHandler(svc *review.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid review id"))
			return
		}

		reviewType := r.PathValue("type")
		if reviewType != "service" && reviewType != "product" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("type must be 'service' or 'product'"))
			return
		}

		review, err := svc.GetReview(r.Context(), *claims.MerchantID, reviewType, id)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get review", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(review)
	}
}

func makeReviewReplyHandler(svc *review.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid review id"))
			return
		}

		reviewType := r.PathValue("type")
		if reviewType != "service" && reviewType != "product" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("type must be 'service' or 'product'"))
			return
		}

		var req review.ReplyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		review, err := svc.SubmitReply(r.Context(), *claims.MerchantID, reviewType, id, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to submit reply", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(review)
	}
}

func makeReviewStatsHandler(svc *review.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		period := r.URL.Query().Get("period")
		if period == "" {
			period = "month"
		}

		stats, err := svc.GetStats(r.Context(), *claims.MerchantID, period)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get review stats", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}

func makeReviewEmployeeStatsHandler(svc *review.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		stats, err := svc.GetEmployeeStats(r.Context(), *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get employee stats", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"employees": stats,
		})
	}
}
