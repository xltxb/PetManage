package auth

import "github.com/gin-gonic/gin"

// RegisterRoutes registers auth endpoints under /api/v1/auth.
func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	auth := r.Group("/auth")
	{
		auth.POST("/login", h.Login)
		auth.POST("/refresh", h.Refresh)
		auth.POST("/switch-store", h.SwitchStore)
		auth.POST("/logout", h.Logout)
	}
}
