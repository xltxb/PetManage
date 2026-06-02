package boarding

import "github.com/gin-gonic/gin"

// RegisterRoutes registers boarding endpoints under the given router group.
func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	boarding := r.Group("/boarding-orders")
	{
		boarding.GET("", h.List)
		boarding.POST("/check-in", h.CheckIn)
		boarding.GET("/:id", h.Get)
		boarding.POST("/:id/check-out", h.CheckOut)
		boarding.GET("/:id/care-logs", h.GetCareLogs)
		boarding.POST("/:id/care-logs", h.PostCareLog)
	}
}
