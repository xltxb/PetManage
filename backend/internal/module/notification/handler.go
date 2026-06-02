package notification

import (
	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/response"
)

// Handler processes notification HTTP requests.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Send handles POST /api/v1/notifications/send.
func (h *Handler) Send(c *gin.Context) {
	var req struct {
		TemplateCode string            `json:"template_code" binding:"required"`
		Channel      string            `json:"channel" binding:"required"`
		CustomerID   int64             `json:"customer_id"`
		Payload      map[string]string `json:"payload"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("参数校验失败"))
		return
	}

	storeID, _ := c.Get("current_store_id")

	sendReq := SendRequest{
		StoreID:      storeID.(int64),
		CustomerID:   req.CustomerID,
		TemplateCode: req.TemplateCode,
		Channel:      req.Channel,
		Payload:      req.Payload,
	}

	if err := h.svc.Send(sendReq); err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, nil)
}
