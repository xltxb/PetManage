package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xltxb/PetManage/internal/merchantrole"
	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// --- Merchant role management handlers ---

func makeMerchantRoleCreateHandler(svc *merchantrole.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		var req merchantrole.CreateRoleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		role, err := svc.CreateRole(r.Context(), *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create role", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(role)
	}
}

func makeMerchantRoleListHandler(svc *merchantrole.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		roles, err := svc.ListRoles(r.Context(), *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list roles", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"roles": roles,
		})
	}
}

func makeMerchantRoleGetHandler(svc *merchantrole.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid role id"))
			return
		}

		role, err := svc.GetRole(r.Context(), *claims.MerchantID, id)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get role", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(role)
	}
}

func makeMerchantRoleUpdateHandler(svc *merchantrole.Service, permChecker *middleware.PermissionChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid role id"))
			return
		}

		var req merchantrole.UpdateRoleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		role, err := svc.UpdateRole(r.Context(), *claims.MerchantID, id, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update role", err))
			return
		}

		// Invalidate permission cache so all employees get updated permissions.
		permChecker.InvalidateAll()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(role)
	}
}

func makeMerchantRoleDeleteHandler(svc *merchantrole.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid role id"))
			return
		}

		if err := svc.DeleteRole(r.Context(), *claims.MerchantID, id); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to delete role", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "role deleted"})
	}
}

func makeMerchantRolePermissionsHandler(svc *merchantrole.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"permissions": svc.GetAvailablePermissions(),
		})
	}
}

func makeEmployeeAssignRoleHandler(svc *merchantrole.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		employeeID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || employeeID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid employee id"))
			return
		}

		var req merchantrole.AssignRoleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		if err := svc.AssignRole(r.Context(), *claims.MerchantID, employeeID, req.RoleID); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to assign role", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "role assigned"})
	}
}

func makeEmployeeCreateAccountHandler(svc *merchantrole.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		employeeID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || employeeID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid employee id"))
			return
		}

		account, err := svc.CreateEmployeeAccount(r.Context(), *claims.MerchantID, employeeID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create account", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(account)
	}
}

func makeEmployeeDisableAccountHandler(svc *merchantrole.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		employeeID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || employeeID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid employee id"))
			return
		}

		if err := svc.DisableEmployeeAccount(r.Context(), *claims.MerchantID, employeeID); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to disable account", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "account disabled"})
	}
}
