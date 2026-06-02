package boarding

import (
	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/middleware"
	"pawprint/backend/internal/module/auth"
)

// RegisterRoutes registers boarding endpoints under the given router group.
func RegisterRoutes(r *gin.RouterGroup, h *Handler, authSvc *auth.Service, idem gin.HandlerFunc) {
	boarding := r.Group("/boarding-orders")
	{
		boarding.GET("", middleware.RequirePermission(authSvc, "boarding:view"), h.List)
		boarding.POST("/check-in", middleware.RequirePermission(authSvc, "boarding:checkin"), idem, h.CheckIn)
		boarding.GET("/:id", middleware.RequirePermission(authSvc, "boarding:view"), h.Get)
		boarding.POST("/:id/check-out", middleware.RequirePermission(authSvc, "boarding:checkout"), idem, h.CheckOut)
		boarding.POST("/:id/cancel", middleware.RequirePermission(authSvc, "boarding:checkout"), idem, h.Cancel)
		boarding.GET("/:id/care-logs", middleware.RequirePermission(authSvc, "boarding:view"), h.GetCareLogs)
		boarding.POST("/:id/care-logs", middleware.RequirePermission(authSvc, "boarding:care"), idem, h.PostCareLog)
	}
}
