package analytics

import "github.com/gin-gonic/gin"

// RegisterRoutes registers analytics endpoints under the given router group.
func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	r.GET("/analytics/report", h.GetReport)
}
