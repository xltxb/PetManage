package pet

import "github.com/gin-gonic/gin"

// RegisterRoutes registers pet endpoints under the given router group.
func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	pets := r.Group("/pets")
	{
		pets.POST("", h.Create)
		pets.GET("/:id", h.Get)
		pets.POST("/:id/health", h.AddHealthRecord)
		pets.POST("/:id/weights", h.AddWeightRecord)
	}
	// Customer-scoped pets
	r.GET("/customers/:id/pets", h.ListByCustomer)
}
