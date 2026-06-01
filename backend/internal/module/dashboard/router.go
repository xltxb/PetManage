package dashboard

import "github.com/gin-gonic/gin"

// RegisterRoutes registers dashboard endpoints under the given router group.
func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	r.GET("/dashboard/summary", h.GetSummary)
}
