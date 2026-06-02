package dashboard

import (
	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/middleware"
	"pawprint/backend/internal/module/auth"
)

// RegisterRoutes registers dashboard endpoints under the given router group.
func RegisterRoutes(r *gin.RouterGroup, h *Handler, authSvc *auth.Service) {
	r.GET("/dashboard/summary", middleware.RequirePermission(authSvc, "dashboard:view"), h.GetSummary)
}
