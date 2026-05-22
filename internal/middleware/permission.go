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
