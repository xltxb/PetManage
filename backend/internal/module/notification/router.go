package notification

import (
	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/middleware"
	"pawprint/backend/internal/module/auth"
)

// RegisterRoutes registers notification endpoints under the given router group.
func RegisterRoutes(r *gin.RouterGroup, h *Handler, authSvc *auth.Service, idem gin.HandlerFunc) {
	notif := r.Group("/notifications")
	{
		notif.POST("/send", middleware.RequirePermission(authSvc, "setting:manage"), idem, h.Send)
	}
}
