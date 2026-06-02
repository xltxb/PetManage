package appointment

import (
	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/middleware"
	"pawprint/backend/internal/module/auth"
)

// RegisterRoutes registers appointment endpoints under the given router group.
func RegisterRoutes(r *gin.RouterGroup, h *Handler, authSvc *auth.Service, idem gin.HandlerFunc) {
	appointments := r.Group("/appointments")
	{
		appointments.GET("", middleware.RequirePermission(authSvc, "appointment:view"), h.List)
		appointments.POST("", middleware.RequirePermission(authSvc, "appointment:create"), idem, h.Create)
		appointments.GET("/:id", middleware.RequirePermission(authSvc, "appointment:view"), h.Get)
		appointments.POST("/:id/transitions", middleware.RequirePermission(authSvc, "appointment:transition"), idem, h.Transition)
		appointments.POST("/:id/cancel", middleware.RequirePermission(authSvc, "appointment:transition"), idem, h.Cancel)
		appointments.GET("/available-slots", middleware.RequirePermission(authSvc, "appointment:view"), h.AvailableSlots)
	}
}
