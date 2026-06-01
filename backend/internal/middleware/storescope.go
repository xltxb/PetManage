package middleware

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/module/auth"
	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/errcode"
	"pawprint/backend/internal/pkg/response"
)

// StoreScope enforces multi-store isolation.
// Extracts X-Store-Id header and validates user has access to that store.
// super_admin can pass X-Store-Id: * for cross-store queries.
func StoreScope(authSvc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		userID, _ := c.Get("user_id")
		jwtStoreID, _ := c.Get("store_id")

		storeIDStr := c.GetHeader("X-Store-Id")
		if storeIDStr == "" {
			response.Error(c, apperr.New(errcode.BadRequest, "缺少 X-Store-Id 请求头"))
			c.Abort()
			return
		}

		// super_admin wildcard
		if role == "super_admin" && storeIDStr == "*" {
			c.Set("current_store_id", int64(0)) // 0 means all stores in repo layer
			c.Next()
			return
		}

		storeID, err := strconv.ParseInt(storeIDStr, 10, 64)
		if err != nil {
			response.Error(c, apperr.New(errcode.BadRequest, "无效的 X-Store-Id"))
			c.Abort()
			return
		}

		// Non-super_admin: JWT store_id must match requested store
		if role != "super_admin" && jwtStoreID != storeID {
			response.Error(c, apperr.New(errcode.StoreForbidden, "跨门店访问被拒"))
			c.Abort()
			return
		}

		// Verify user has access to this store
		if _, err := authSvc.VerifyStoreAccess(userID.(int64), storeID); err != nil {
			if ae, ok := err.(*apperr.AppError); ok {
				response.Error(c, ae)
			} else {
				response.Error(c, apperr.New(errcode.StoreForbidden, "跨门店访问被拒"))
			}
			c.Abort()
			return
		}

		c.Set("current_store_id", storeID)
		c.Next()
	}
}
