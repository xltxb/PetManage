package settlement

import "github.com/gin-gonic/gin"

// RegisterRoutes registers settlement endpoints under the given router group.
func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	settlements := r.Group("/settlements")
	{
		settlements.GET("", h.List)
		settlements.POST("", h.Create)
		settlements.POST("/:id/pay", h.Pay)
		settlements.POST("/:id/refund", h.Refund)
		settlements.POST("/:id/void", h.Void)
	}
}
