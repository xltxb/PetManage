package inventory

import "github.com/gin-gonic/gin"

// RegisterRoutes registers inventory endpoints under the given router group.
func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	inv := r.Group("/inventory")
	{
		inv.POST("/sale-out", h.SaleOut)
		inv.POST("/purchase-in", h.PurchaseIn)
		inv.POST("/adjust", h.Adjust)
		inv.GET("/alerts", h.Alerts)
	}
}
