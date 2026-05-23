package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xltxb/PetManage/internal/attendance"
	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// --- Attendance check-in/check-out handlers ---

func makeAttendanceCheckInHandler(svc *attendance.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		employeeID, err := svc.GetEmployeeIDByUser(r.Context(), *claims.MerchantID, claims.UserID, nil)
		if err != nil || employeeID == 0 {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("only employees can check in"))
			return
		}

		record, err := svc.CheckIn(r.Context(), *claims.MerchantID, employeeID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("check-in failed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(record)
	}
}

func makeAttendanceCheckOutHandler(svc *attendance.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		employeeID, err := svc.GetEmployeeIDByUser(r.Context(), *claims.MerchantID, claims.UserID, nil)
		if err != nil || employeeID == 0 {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("only employees can check out"))
			return
		}

		record, err := svc.CheckOut(r.Context(), *claims.MerchantID, employeeID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("check-out failed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(record)
	}
}

func makeAttendanceTodayHandler(svc *attendance.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		employeeID, err := svc.GetEmployeeIDByUser(r.Context(), *claims.MerchantID, claims.UserID, nil)
		if err != nil || employeeID == 0 {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("only employees have attendance records"))
			return
		}

		record, err := svc.GetTodayStatus(r.Context(), *claims.MerchantID, employeeID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get today's status", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if record == nil {
			// Not checked in yet — return empty object instead of null.
			json.NewEncoder(w).Encode(map[string]interface{}{
				"checked_in": false,
			})
			return
		}
		json.NewEncoder(w).Encode(record)
	}
}

// --- Leave request handlers ---

func makeLeaveApplyHandler(svc *attendance.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		employeeID, err := svc.GetEmployeeIDByUser(r.Context(), *claims.MerchantID, claims.UserID, nil)
		if err != nil || employeeID == 0 {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("only employees can apply for leave"))
			return
		}

		var req attendance.CreateLeaveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		lr, err := svc.ApplyLeave(r.Context(), *claims.MerchantID, employeeID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to apply leave", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(lr)
	}
}

func makeLeaveListHandler(svc *attendance.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		params := attendance.LeaveListParams{}
		q := r.URL.Query()

		if v := q.Get("employee_id"); v != "" {
			params.EmployeeID, _ = strconv.ParseInt(v, 10, 64)
		}
		params.Status = q.Get("status")
		if v := q.Get("page"); v != "" {
			params.Page, _ = strconv.Atoi(v)
		}
		if v := q.Get("page_size"); v != "" {
			params.PageSize, _ = strconv.Atoi(v)
		}

		// If this is an employee viewing their own leaves, restrict to their employee_id.
		employeeID, _ := svc.GetEmployeeIDByUser(r.Context(), *claims.MerchantID, claims.UserID, nil)
		if employeeID > 0 && params.EmployeeID == 0 {
			params.EmployeeID = employeeID
		}

		result, err := svc.ListLeaves(r.Context(), *claims.MerchantID, params)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list leave requests", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeLeaveReviewHandler(svc *attendance.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		leaveID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid leave id"))
			return
		}

		var req attendance.ReviewLeaveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		// Reviewer's employee_id (optional, for store owner this stays 0).
		var reviewerEmpID int64
		empID, _ := svc.GetEmployeeIDByUser(r.Context(), *claims.MerchantID, claims.UserID, nil)
		if empID > 0 {
			reviewerEmpID = empID
		}

		lr, err := svc.ReviewLeave(r.Context(), *claims.MerchantID, leaveID, reviewerEmpID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to review leave", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(lr)
	}
}

// --- Overtime handlers ---

func makeOvertimeApplyHandler(svc *attendance.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		employeeID, err := svc.GetEmployeeIDByUser(r.Context(), *claims.MerchantID, claims.UserID, nil)
		if err != nil || employeeID == 0 {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("only employees can register overtime"))
			return
		}

		var req attendance.CreateOvertimeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		or_, err := svc.ApplyOvertime(r.Context(), *claims.MerchantID, employeeID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to register overtime", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(or_)
	}
}

func makeOvertimeListHandler(svc *attendance.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		params := attendance.OvertimeListParams{}
		q := r.URL.Query()

		if v := q.Get("employee_id"); v != "" {
			params.EmployeeID, _ = strconv.ParseInt(v, 10, 64)
		}
		params.Status = q.Get("status")
		if v := q.Get("page"); v != "" {
			params.Page, _ = strconv.Atoi(v)
		}
		if v := q.Get("page_size"); v != "" {
			params.PageSize, _ = strconv.Atoi(v)
		}

		// Restrict to employee's own records if they're an employee.
		employeeID, _ := svc.GetEmployeeIDByUser(r.Context(), *claims.MerchantID, claims.UserID, nil)
		if employeeID > 0 && params.EmployeeID == 0 {
			params.EmployeeID = employeeID
		}

		result, err := svc.ListOvertime(r.Context(), *claims.MerchantID, params)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list overtime records", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeOvertimeReviewHandler(svc *attendance.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		overtimeID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid overtime id"))
			return
		}

		var req attendance.ReviewOvertimeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		var reviewerEmpID int64
		empID, _ := svc.GetEmployeeIDByUser(r.Context(), *claims.MerchantID, claims.UserID, nil)
		if empID > 0 {
			reviewerEmpID = empID
		}

		or_, err := svc.ReviewOvertime(r.Context(), *claims.MerchantID, overtimeID, reviewerEmpID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to review overtime", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(or_)
	}
}

// --- Stats handler ---

func makeAttendanceStatsHandler(svc *attendance.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		params := attendance.StatsParams{}
		q := r.URL.Query()

		if v := q.Get("employee_id"); v != "" {
			params.EmployeeID, _ = strconv.ParseInt(v, 10, 64)
		}
		params.StartDate = q.Get("start_date")
		params.EndDate = q.Get("end_date")
		params.Type = q.Get("type")

		stats, err := svc.GetStats(r.Context(), *claims.MerchantID, params)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get attendance stats", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}
