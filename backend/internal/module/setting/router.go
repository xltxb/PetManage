package setting

import (
	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/middleware"
	"pawprint/backend/internal/module/auth"
)

// RegisterRoutes registers settings endpoints under the given router group.
func RegisterRoutes(r *gin.RouterGroup, h *Handler, authSvc *auth.Service, idem gin.HandlerFunc) {
	settings := r.Group("/settings")
	{
		settings.GET("", middleware.RequirePermission(authSvc, "setting:manage"), h.GetAll)
		settings.GET("/:key", middleware.RequirePermission(authSvc, "setting:manage"), h.Get)
		settings.PUT("/:key", middleware.RequirePermission(authSvc, "setting:manage"), idem, h.Set)
	}
}
