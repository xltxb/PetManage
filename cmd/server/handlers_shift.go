package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/xltxb/PetManage/internal/auth"
	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/internal/shift"
)

func makeShiftCreateHandler(shiftSvc *shift.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"code": "UNAUTHORIZED", "message": "not authenticated"})
			return
		}

		merchantID := getMerchantID(claims)
		if merchantID == 0 {
			writeJSON(w, http.StatusForbidden, map[string]string{"code": "FORBIDDEN", "message": "merchant access required"})
			return
		}

		employeeID := getEmployeeID(claims)
		if employeeID == 0 {
			writeJSON(w, http.StatusForbidden, map[string]string{"code": "FORBIDDEN", "message": "employee access required, only staff can perform shift handover"})
			return
		}

		sr, err := shiftSvc.CreateShiftReport(r.Context(), merchantID, employeeID)
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, sr)
	}
}

func makeShiftGetHandler(shiftSvc *shift.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"code": "UNAUTHORIZED", "message": "not authenticated"})
			return
		}

		merchantID := getMerchantID(claims)
		if merchantID == 0 {
			writeJSON(w, http.StatusForbidden, map[string]string{"code": "FORBIDDEN", "message": "merchant access required"})
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "INVALID_PARAMS", "message": "invalid shift id"})
			return
		}

		sr, err := shiftSvc.GetShiftReport(r.Context(), merchantID, id)
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, sr)
	}
}

func makeShiftListHandler(shiftSvc *shift.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"code": "UNAUTHORIZED", "message": "not authenticated"})
			return
		}

		merchantID := getMerchantID(claims)
		if merchantID == 0 {
			writeJSON(w, http.StatusForbidden, map[string]string{"code": "FORBIDDEN", "message": "merchant access required"})
			return
		}

		q := r.URL.Query()
		params := shift.ListParams{
			StartDate: q.Get("start_date"),
			EndDate:   q.Get("end_date"),
			Status:    q.Get("status"),
		}
		if p := q.Get("page"); p != "" {
			params.Page, _ = strconv.Atoi(p)
		}
		if ps := q.Get("page_size"); ps != "" {
			params.PageSize, _ = strconv.Atoi(ps)
		}

		result, err := shiftSvc.ListShiftReports(r.Context(), merchantID, params)
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func makeShiftConfirmHandler(shiftSvc *shift.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"code": "UNAUTHORIZED", "message": "not authenticated"})
			return
		}

		merchantID := getMerchantID(claims)
		if merchantID == 0 {
			writeJSON(w, http.StatusForbidden, map[string]string{"code": "FORBIDDEN", "message": "merchant access required"})
			return
		}

		confirmerID := getEmployeeID(claims)
		if confirmerID == 0 {
			writeJSON(w, http.StatusForbidden, map[string]string{"code": "FORBIDDEN", "message": "employee access required, only staff can confirm shift reports"})
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "INVALID_PARAMS", "message": "invalid shift id"})
			return
		}

		sr, err := shiftSvc.ConfirmShiftReport(r.Context(), merchantID, id, confirmerID)
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, sr)
	}
}

func makeShiftTodayHandler(shiftSvc *shift.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"code": "UNAUTHORIZED", "message": "not authenticated"})
			return
		}

		merchantID := getMerchantID(claims)
		if merchantID == 0 {
			writeJSON(w, http.StatusForbidden, map[string]string{"code": "FORBIDDEN", "message": "merchant access required"})
			return
		}

		employeeID := getEmployeeID(claims)
		if employeeID == 0 {
			writeJSON(w, http.StatusForbidden, map[string]string{"code": "FORBIDDEN", "message": "employee access required"})
			return
		}

		sr, err := shiftSvc.GetTodayShiftStatus(r.Context(), merchantID, employeeID)
		if err != nil {
			writeAppError(w, err)
			return
		}

		hasShift := sr != nil && sr.ID > 0
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"has_shift": hasShift,
			"shift":     sr,
			"date":      time.Now().Format("2006-01-02"),
		})
	}
}

// getMerchantID safely extracts merchant ID from claims.
func getMerchantID(claims *auth.Claims) int64 {
	if claims.MerchantID != nil {
		return *claims.MerchantID
	}
	return 0
}

// getEmployeeID safely extracts employee ID from claims.
func getEmployeeID(claims *auth.Claims) int64 {
	if claims.EmployeeID != nil {
		return *claims.EmployeeID
	}
	return 0
}

// writeAppError writes an application error response.
func writeAppError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	// Check for standard error codes from apperrors package.
	codeToStatus := map[string]int{
		"INVALID_PARAMS": http.StatusBadRequest,
		"UNAUTHORIZED":   http.StatusUnauthorized,
		"FORBIDDEN":      http.StatusForbidden,
		"NOT_FOUND":      http.StatusNotFound,
		"CONFLICT":       http.StatusConflict,
		"INTERNAL_ERROR": http.StatusInternalServerError,
	}
	msg := err.Error()
	code := "INTERNAL_ERROR"
	status := http.StatusInternalServerError
	for k, s := range codeToStatus {
		if len(msg) >= len(k) && msg[:len(k)] == k {
			code = k
			status = s
			break
		}
	}
	writeJSON(w, status, map[string]string{"code": code, "message": msg})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
