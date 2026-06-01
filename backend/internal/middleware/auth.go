package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/module/auth"
	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/response"
)

// AuthRequired validates the JWT access token and injects claims into context.
func AuthRequired(authSvc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			response.Error(c, apperr.Unauthorized("缺少认证令牌"))
			c.Abort()
			return
		}

		claims, err := authSvc.ParseAccessToken(token)
		if err != nil {
			if ae, ok := err.(*apperr.AppError); ok {
				response.Error(c, ae)
			} else {
				response.Error(c, apperr.Unauthorized())
			}
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("store_id", claims.StoreID)
		c.Set("role", claims.Role)
		c.Next()
	}
}

func extractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}
