package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/internal/schedule"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// makeScheduleListHandler lists schedules with optional date range and employee filters.
func makeScheduleListHandler(svc *schedule.Service) http.HandlerFunc {
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
		employeeID, _ := strconv.ParseInt(q.Get("employee_id"), 10, 64)

		result, err := svc.List(r.Context(), *claims.MerchantID, employeeID, q.Get("start_date"), q.Get("end_date"))
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list schedules", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"schedules": result,
		})
	}
}

// makeScheduleUpsertHandler creates or updates a single schedule entry.
func makeScheduleUpsertHandler(svc *schedule.Service) http.HandlerFunc {
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

		var req struct {
			EmployeeID   int64  `json:"employee_id"`
			ScheduleDate string `json:"schedule_date"`
			ShiftType    string `json:"shift_type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		sch, err := svc.Upsert(r.Context(), *claims.MerchantID, req.EmployeeID, req.ScheduleDate, req.ShiftType)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to upsert schedule", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sch)
	}
}

// makeScheduleBatchSetHandler sets schedules for an employee for multiple days.
func makeScheduleBatchSetHandler(svc *schedule.Service) http.HandlerFunc {
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

		var req schedule.BatchSetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		result, err := svc.BatchSet(r.Context(), *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to batch set schedules", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"schedules": result,
		})
	}
}

// makeScheduleCopyWeekHandler copies a week's schedules from one employee to another.
func makeScheduleCopyWeekHandler(svc *schedule.Service) http.HandlerFunc {
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

		var req schedule.CopyWeekRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		if err := svc.CopyWeek(r.Context(), *claims.MerchantID, req); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to copy schedules", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "schedules copied successfully"})
	}
}

// makeScheduleOnDutyHandler returns employees on duty for a given appointment time.
func makeScheduleOnDutyHandler(svc *schedule.Service) http.HandlerFunc {
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
		timeStr := q.Get("appointment_time")
		if timeStr == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("appointment_time query parameter is required"))
			return
		}

		apptTime, err := time.Parse(time.RFC3339, timeStr)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid appointment_time format, use RFC3339"))
			return
		}

		employees, err := svc.GetOnDutyEmployees(r.Context(), *claims.MerchantID, apptTime)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get on-duty employees", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"employees": employees,
		})
	}
}
