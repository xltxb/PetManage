package inventory

import (
	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/middleware"
	"pawprint/backend/internal/module/auth"
)

// RegisterRoutes registers inventory endpoints under the given router group.
func RegisterRoutes(r *gin.RouterGroup, h *Handler, authSvc *auth.Service, idem gin.HandlerFunc) {
	inv := r.Group("/inventory")
	{
		inv.POST("/sale-out", middleware.RequirePermission(authSvc, "inventory:sale"), idem, h.SaleOut)
		inv.POST("/purchase-in", middleware.RequirePermission(authSvc, "inventory:purchase"), idem, h.PurchaseIn)
		inv.POST("/adjust", middleware.RequirePermission(authSvc, "inventory:purchase"), idem, h.Adjust)
		inv.GET("/alerts", middleware.RequirePermission(authSvc, "inventory:view"), h.Alerts)
	}
}
