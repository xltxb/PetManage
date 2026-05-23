package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/internal/payable"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

func makePayableListHandler(svc *payable.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		pageSize, _ := strconv.Atoi(q.Get("page_size"))
		supplierID, _ := strconv.ParseInt(q.Get("supplier_id"), 10, 64)

		params := payable.ListPayablesParams{
			SupplierID: supplierID,
			Status:     q.Get("status"),
			Page:       page,
			PageSize:   pageSize,
		}

		result, err := svc.ListPayables(r.Context(), *claims.MerchantID, params)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list payables", err))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makePayableSupplierSummaryHandler(svc *payable.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		result, err := svc.ListBySupplier(r.Context(), *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list supplier summaries", err))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makePayableGetHandler(svc *payable.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid payable id"))
			return
		}

		result, err := svc.GetByID(r.Context(), id, *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get payable", err))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makePayableRegisterPaymentHandler(svc *payable.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid payable id"))
			return
		}

		var req payable.RegisterPaymentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		result, err := svc.RegisterPayment(r.Context(), *claims.MerchantID, claims.UserID, id, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to register payment", err))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(result)
	}
}

func makePayableStatementHandler(svc *payable.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		q := r.URL.Query()
		supplierIDStr := q.Get("supplier_id")
		startDate := q.Get("start_date")
		endDate := q.Get("end_date")

		if supplierIDStr == "" || startDate == "" || endDate == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("supplier_id, start_date, and end_date are required"))
			return
		}

		supplierID, err := strconv.ParseInt(supplierIDStr, 10, 64)
		if err != nil || supplierID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid supplier_id"))
			return
		}

		result, err := svc.GetStatement(r.Context(), *claims.MerchantID, supplierID, startDate, endDate)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get statement", err))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makePayableStatementExportHandler(svc *payable.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		q := r.URL.Query()
		supplierIDStr := q.Get("supplier_id")
		startDate := q.Get("start_date")
		endDate := q.Get("end_date")

		if supplierIDStr == "" || startDate == "" || endDate == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("supplier_id, start_date, and end_date are required"))
			return
		}

		supplierID, err := strconv.ParseInt(supplierIDStr, 10, 64)
		if err != nil || supplierID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid supplier_id"))
			return
		}

		statement, err := svc.GetStatement(r.Context(), *claims.MerchantID, supplierID, startDate, endDate)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get statement", err))
			}
			return
		}

		pdfData, err := svc.GenerateStatementPDF(statement)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to generate PDF", err))
			return
		}

		filename := "statement_" + statement.SupplierName + "_" + startDate + "_" + endDate + ".pdf"
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
		w.WriteHeader(http.StatusOK)
		w.Write(pdfData)
	}
}
