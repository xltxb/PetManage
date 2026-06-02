package appointment

import "github.com/gin-gonic/gin"

// RegisterRoutes registers appointment endpoints under the given router group.
func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	appointments := r.Group("/appointments")
	{
		appointments.GET("", h.List)
		appointments.POST("", h.Create)
		appointments.GET("/:id", h.Get)
		appointments.POST("/:id/transitions", h.Transition)
		appointments.POST("/:id/cancel", h.Cancel)
		appointments.GET("/available-slots", h.AvailableSlots)
	}
}
