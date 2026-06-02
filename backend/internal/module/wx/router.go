package wx

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.RouterGroup, h *Handler, idem gin.HandlerFunc) {
	wx := r.Group("/wx")
	{
		wx.POST("/auth/login", h.Login)
		wx.GET("/service-offerings", h.ServiceOfferings)
		wx.POST("/appointments", idem, h.CreateAppointment)
		wx.POST("/appointments/:id/cancel", h.CancelAppointment)
	}
}
