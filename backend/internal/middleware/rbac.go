package middleware

import (
	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/module/auth"
	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/response"
)

// RequirePermission checks that the authenticated user has the required permission.
// Usage: r.POST("/appointments", RequirePermission(authSvc, "appointment:create"), handler.Create)
func RequirePermission(authSvc *auth.Service, permissionCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// super_admin bypasses permission checks
		role, _ := c.Get("role")
		if role == "super_admin" {
			c.Next()
			return
		}

		userID, exists := c.Get("user_id")
		if !exists {
			response.Error(c, apperr.Unauthorized())
			c.Abort()
			return
		}

		storeID, _ := c.Get("store_id")

		perms, err := authSvc.GetPermissions(userID.(int64), storeID.(int64))
		if err != nil {
			response.Error(c, apperr.Internal(err))
			c.Abort()
			return
		}

		for _, p := range perms {
			if p == permissionCode {
				c.Next()
				return
			}
		}

		response.Error(c, apperr.Forbidden("无此操作权限: "+permissionCode))
		c.Abort()
	}
}
