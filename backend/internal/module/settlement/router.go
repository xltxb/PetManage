package settlement

import (
	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/middleware"
	"pawprint/backend/internal/module/auth"
)

// RegisterRoutes registers settlement endpoints under the given router group.
func RegisterRoutes(r *gin.RouterGroup, h *Handler, authSvc *auth.Service, idem gin.HandlerFunc) {
	settlements := r.Group("/settlements")
	{
		settlements.GET("", middleware.RequirePermission(authSvc, "settlement:create"), h.List)
		settlements.POST("", middleware.RequirePermission(authSvc, "settlement:create"), idem, h.Create)
		settlements.POST("/:id/pay", middleware.RequirePermission(authSvc, "settlement:pay"), idem, h.Pay)
		settlements.POST("/:id/refund", middleware.RequirePermission(authSvc, "settlement:pay"), idem, h.Refund)
		settlements.POST("/:id/void", middleware.RequirePermission(authSvc, "settlement:create"), idem, h.Void)
	}
}
