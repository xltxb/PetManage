package notification

import "github.com/gin-gonic/gin"

// RegisterRoutes registers notification endpoints under the given router group.
func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	notif := r.Group("/notifications")
	{
		notif.POST("/send", h.Send)
	}
}
