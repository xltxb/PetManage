package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/internal/statement"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

func makeProfitLossHandler(svc *statement.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		year, month, err := parseYearMonth(r)
		if err != nil {
			apperrors.WriteError(w, r, err)
			return
		}

		result, svcErr := svc.GetProfitLoss(r.Context(), *claims.MerchantID, year, month)
		if svcErr != nil {
			if appErr, ok := svcErr.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get profit & loss", svcErr))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeRevenueDetailHandler(svc *statement.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		year, month, err := parseYearMonth(r)
		if err != nil {
			apperrors.WriteError(w, r, err)
			return
		}

		result, svcErr := svc.GetRevenueDetail(r.Context(), *claims.MerchantID, year, month)
		if svcErr != nil {
			if appErr, ok := svcErr.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get revenue detail", svcErr))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeProductSalesHandler(svc *statement.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		year, month, err := parseYearMonth(r)
		if err != nil {
			apperrors.WriteError(w, r, err)
			return
		}

		result, svcErr := svc.GetProductSales(r.Context(), *claims.MerchantID, year, month)
		if svcErr != nil {
			if appErr, ok := svcErr.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get product sales", svcErr))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeServicePerformanceHandler(svc *statement.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		year, month, err := parseYearMonth(r)
		if err != nil {
			apperrors.WriteError(w, r, err)
			return
		}

		result, svcErr := svc.GetServicePerformance(r.Context(), *claims.MerchantID, year, month)
		if svcErr != nil {
			if appErr, ok := svcErr.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
			} else {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get service performance", svcErr))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// --- Statement export handlers ---

func makeExportHandler(svc *statement.Service, exportFn func(ctx context.Context, merchantID int64, startTime, endTime string) ([]byte, string, error), contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		startTime := r.URL.Query().Get("start_time")
		endTime := r.URL.Query().Get("end_time")

		data, filename, err := exportFn(r.Context(), *claims.MerchantID, startTime, endTime)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to export", err))
			return
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		w.Write(data)
	}
}

func makeProfitLossExcelHandler(svc *statement.Service) http.HandlerFunc {
	return makeExportHandler(svc, svc.ExportProfitLossExcel,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
}

func makeProfitLossPDFHandler(svc *statement.Service) http.HandlerFunc {
	return makeExportHandler(svc, svc.ExportProfitLossPDF, "application/pdf")
}

func makeRevenueDetailExcelHandler(svc *statement.Service) http.HandlerFunc {
	return makeExportHandler(svc, svc.ExportRevenueDetailExcel,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
}

func makeRevenueDetailPDFHandler(svc *statement.Service) http.HandlerFunc {
	return makeExportHandler(svc, svc.ExportRevenueDetailPDF, "application/pdf")
}

func makeProductSalesExcelHandler(svc *statement.Service) http.HandlerFunc {
	return makeExportHandler(svc, svc.ExportProductSalesExcel,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
}

func makeProductSalesPDFHandler(svc *statement.Service) http.HandlerFunc {
	return makeExportHandler(svc, svc.ExportProductSalesPDF, "application/pdf")
}

func makeServicePerformanceExcelHandler(svc *statement.Service) http.HandlerFunc {
	return makeExportHandler(svc, svc.ExportServicePerformanceExcel,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
}

func makeServicePerformancePDFHandler(svc *statement.Service) http.HandlerFunc {
	return makeExportHandler(svc, svc.ExportServicePerformancePDF, "application/pdf")
}

func parseYearMonth(r *http.Request) (int, int, *apperrors.AppError) {
	q := r.URL.Query()
	yearStr := q.Get("year")
	if yearStr == "" {
		return 0, 0, apperrors.NewValidationError("year is required")
	}
	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 2000 {
		return 0, 0, apperrors.NewValidationError("invalid year")
	}

	month := 0
	monthStr := q.Get("month")
	if monthStr != "" {
		month, err = strconv.Atoi(monthStr)
		if err != nil || month < 0 || month > 12 {
			return 0, 0, apperrors.NewValidationError("invalid month (0-12)")
		}
	}

	return year, month, nil
}
