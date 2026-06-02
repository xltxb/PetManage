package analytics

import (
	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/middleware"
	"pawprint/backend/internal/module/auth"
)

// RegisterRoutes registers analytics endpoints under the given router group.
func RegisterRoutes(r *gin.RouterGroup, h *Handler, authSvc *auth.Service) {
	r.GET("/analytics/report", middleware.RequirePermission(authSvc, "analytics:view"), h.GetReport)
}
