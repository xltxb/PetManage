package member

import (
	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/middleware"
	"pawprint/backend/internal/module/auth"
)

// RegisterRoutes registers member endpoints under the given router group.
func RegisterRoutes(r *gin.RouterGroup, h *Handler, authSvc *auth.Service, idem gin.HandlerFunc) {
	customers := r.Group("/customers")
	{
		customers.GET("", middleware.RequirePermission(authSvc, "member:view"), h.List)
		customers.GET("/:id", middleware.RequirePermission(authSvc, "member:view"), h.Get)
		customers.POST("/:id/wallet", middleware.RequirePermission(authSvc, "member:wallet"), idem, h.Recharge)
		customers.PUT("/:id/wallet", middleware.RequirePermission(authSvc, "member:wallet"), idem, h.AdjustWallet)
	}
}
