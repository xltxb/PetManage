package setting

import "github.com/gin-gonic/gin"

// RegisterRoutes registers settings endpoints under the given router group.
func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	settings := r.Group("/settings")
	{
		settings.GET("", h.GetAll)
		settings.GET("/:key", h.Get)
		settings.PUT("/:key", h.Set)
	}
}
