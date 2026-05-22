package middleware

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// PermissionChecker caches role permissions and validates access.
type PermissionChecker struct {
	db    *sql.DB
	mu    sync.RWMutex
	cache map[int64]permissionCacheEntry
}

type permissionCacheEntry struct {
	permissions []string
	expires     time.Time
}

// NewPermissionChecker creates a PermissionChecker with a 60-second cache TTL.
func NewPermissionChecker(db *sql.DB) *PermissionChecker {
	return &PermissionChecker{
		db:    db,
		cache: make(map[int64]permissionCacheEntry),
	}
}

func (pc *PermissionChecker) getPermissions(roleID int64) ([]string, error) {
	pc.mu.RLock()
	entry, ok := pc.cache[roleID]
	pc.mu.RUnlock()
	if ok && time.Now().Before(entry.expires) {
		return entry.permissions, nil
	}

	var permsStr string
	err := pc.db.QueryRow(
		`SELECT COALESCE(permissions::text, '[]') FROM platform_roles
		 WHERE id = $1 AND deleted_at IS NULL`, roleID,
	).Scan(&permsStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var perms []string
	json.Unmarshal([]byte(permsStr), &perms)

	pc.mu.Lock()
	pc.cache[roleID] = permissionCacheEntry{permissions: perms, expires: time.Now().Add(60 * time.Second)}
	pc.mu.Unlock()

	return perms, nil
}

// RequirePermission returns middleware that checks the current user has the given permission.
// The wildcard "*" grants all permissions.
func (pc *PermissionChecker) RequirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := UserClaimsFromContext(r.Context())
			if claims == nil {
				apperrors.WriteError(w, r, &apperrors.AppError{
					Code:    apperrors.CodeUnauthorized,
					Message: "authentication required",
				})
				return
			}

			if claims.RoleID == 0 {
				apperrors.WriteError(w, r, &apperrors.AppError{
					Code:    apperrors.CodeForbidden,
					Message: "no role assigned, access denied",
				})
				return
			}

			perms, err := pc.getPermissions(claims.RoleID)
			if err != nil {
				apperrors.WriteError(w, r, &apperrors.AppError{
					Code:    apperrors.CodeInternalError,
					Message: "failed to check permissions",
					Err:     err,
				})
				return
			}

			if !hasPermission(perms, permission) {
				apperrors.WriteError(w, r, &apperrors.AppError{
					Code:    apperrors.CodeForbidden,
					Message: "insufficient permissions: " + permission + " required",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireMerchantPermission returns middleware that checks merchant role permissions.
// For users with merchant_id: looks up the employee's merchant role for permissions.
// Merchant owners (no employee record) are granted full access.
// Platform users without merchant_id fall back to platform role permissions.
func (pc *PermissionChecker) RequireMerchantPermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := UserClaimsFromContext(r.Context())
			if claims == nil {
				apperrors.WriteError(w, r, &apperrors.AppError{
					Code:    apperrors.CodeUnauthorized,
					Message: "authentication required",
				})
				return
			}

			// Platform users (no merchant_id) — check platform roles.
			if claims.MerchantID == nil {
				if claims.RoleID == 0 {
					apperrors.WriteError(w, r, &apperrors.AppError{
						Code:    apperrors.CodeForbidden,
						Message: "no role assigned, access denied",
					})
					return
				}
				perms, err := pc.getPermissions(claims.RoleID)
				if err != nil {
					apperrors.WriteError(w, r, &apperrors.AppError{
						Code:    apperrors.CodeInternalError,
						Message: "failed to check permissions", Err: err,
					})
					return
				}
				if !hasPermission(perms, permission) {
					apperrors.WriteError(w, r, &apperrors.AppError{
						Code:    apperrors.CodeForbidden,
						Message: "insufficient permissions: " + permission + " required",
					})
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// Merchant users — check if they are an employee with a merchant role.
			var employeeID sql.NullInt64
			err := pc.db.QueryRow(
				`SELECT employee_id FROM platform_users WHERE id = $1 AND deleted_at IS NULL`,
				claims.UserID,
			).Scan(&employeeID)

			// No employee record — merchant owner/admin, grant full access.
			if err == sql.ErrNoRows || !employeeID.Valid || employeeID.Int64 == 0 {
				next.ServeHTTP(w, r)
				return
			}
			if err != nil {
				apperrors.WriteError(w, r, &apperrors.AppError{
					Code:    apperrors.CodeInternalError,
					Message: "failed to check permissions", Err: err,
				})
				return
			}

			// Employee found — check merchant role permissions.
			perms, err := pc.getMerchantPermissions(employeeID.Int64)
			if err != nil {
				apperrors.WriteError(w, r, &apperrors.AppError{
					Code:    apperrors.CodeInternalError,
					Message: "failed to check permissions", Err: err,
				})
				return
			}
			if !hasPermission(perms, permission) {
				apperrors.WriteError(w, r, &apperrors.AppError{
					Code:    apperrors.CodeForbidden,
					Message: "insufficient permissions: " + permission + " required",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (pc *PermissionChecker) getMerchantPermissions(employeeID int64) ([]string, error) {
	pc.mu.RLock()
	entry, ok := pc.cache[-employeeID]
	pc.mu.RUnlock()
	if ok && time.Now().Before(entry.expires) {
		return entry.permissions, nil
	}

	var permsStr sql.NullString
	err := pc.db.QueryRow(
		`SELECT mr.permissions::text
		 FROM employees e
		 JOIN merchant_roles mr ON e.merchant_role_id = mr.id AND mr.deleted_at IS NULL
		 WHERE e.id = $1 AND e.deleted_at IS NULL`, employeeID,
	).Scan(&permsStr)
	if err == sql.ErrNoRows || !permsStr.Valid {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var perms []string
	json.Unmarshal([]byte(permsStr.String), &perms)

	pc.mu.Lock()
	pc.cache[-employeeID] = permissionCacheEntry{permissions: perms, expires: time.Now().Add(60 * time.Second)}
	pc.mu.Unlock()

	return perms, nil
}

// InvalidateAll clears all cached permissions. Call after role permissions change.
func (pc *PermissionChecker) InvalidateAll() {
	pc.mu.Lock()
	pc.cache = make(map[int64]permissionCacheEntry)
	pc.mu.Unlock()
}

// RequirePlatformUser blocks merchant users from accessing platform endpoints.
// Only users with nil merchant_id (platform admins/operators) are allowed.
func RequirePlatformUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, &apperrors.AppError{
				Code:    apperrors.CodeUnauthorized,
				Message: "authentication required",
			})
			return
		}
		if claims.MerchantID != nil {
			apperrors.WriteError(w, r, &apperrors.AppError{
				Code:    apperrors.CodeForbidden,
				Message: "platform access only, merchant accounts are not permitted",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireMerchantUser blocks platform users from accessing merchant endpoints.
// Only users with a non-nil merchant_id (merchant admins/employees) are allowed.
func RequireMerchantUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, &apperrors.AppError{
				Code:    apperrors.CodeUnauthorized,
				Message: "authentication required",
			})
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, &apperrors.AppError{
				Code:    apperrors.CodeForbidden,
				Message: "merchant account required",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func hasPermission(perms []string, required string) bool {
	for _, p := range perms {
		if p == "*" {
			return true
		}
		if p == required {
			return true
		}
	}
	return false
}
