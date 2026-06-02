package member

import "github.com/gin-gonic/gin"

// RegisterRoutes registers member endpoints under the given router group.
func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	customers := r.Group("/customers")
	{
		customers.GET("", h.List)
		customers.GET("/:id", h.Get)
		customers.POST("/:id/wallet", h.Recharge)
		customers.PUT("/:id/wallet", h.AdjustWallet)
	}
}
