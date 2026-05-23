package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/internal/servicerecord"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

func makeServiceRecordGetHandler(svc *servicerecord.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid service record id"))
			return
		}

		record, err := svc.GetByID(r.Context(), *claims.MerchantID, id)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get service record", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(record)
	}
}

func makePetServiceRecordsHandler(svc *servicerecord.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		petIDStr := r.PathValue("petId")
		petID, err := strconv.ParseInt(petIDStr, 10, 64)
		if err != nil || petID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid pet id"))
			return
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

		result, err := svc.ListByPet(r.Context(), *claims.MerchantID, petID, servicerecord.ListParams{
			Page: page, PageSize: pageSize,
		})
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list service records", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeMemberServiceRecordsHandler(svc *servicerecord.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		memberIDStr := r.PathValue("memberId")
		memberID, err := strconv.ParseInt(memberIDStr, 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

		result, err := svc.ListByMember(r.Context(), *claims.MerchantID, memberID, servicerecord.ListParams{
			Page: page, PageSize: pageSize,
		})
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list service records", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeServiceRecordEvaluateHandler(svc *servicerecord.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		idStr := r.PathValue("id")
		recordID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || recordID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid service record id"))
			return
		}

		var req servicerecord.EvaluateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		eval, err := svc.SubmitEvaluation(r.Context(), *claims.MerchantID, recordID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to submit evaluation", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(eval)
	}
}
