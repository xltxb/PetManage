package pet

import (
	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/middleware"
	"pawprint/backend/internal/module/auth"
)

// RegisterRoutes registers pet endpoints under the given router group.
func RegisterRoutes(r *gin.RouterGroup, h *Handler, authSvc *auth.Service, idem gin.HandlerFunc) {
	pets := r.Group("/pets")
	{
		pets.POST("", middleware.RequirePermission(authSvc, "pet:edit"), idem, h.Create)
		pets.GET("/:id", middleware.RequirePermission(authSvc, "pet:view"), h.Get)
		pets.GET("/:id/consumption", middleware.RequirePermission(authSvc, "pet:view"), h.Consumption)
		pets.POST("/:id/health", middleware.RequirePermission(authSvc, "pet:health"), idem, h.AddHealthRecord)
		pets.POST("/:id/weights", middleware.RequirePermission(authSvc, "pet:health"), idem, h.AddWeightRecord)
	}
	// Customer-scoped pets
	r.GET("/customers/:id/pets", middleware.RequirePermission(authSvc, "pet:view"), h.ListByCustomer)
}
