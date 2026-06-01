package dashboard

import (
	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/response"
)

// Handler processes dashboard HTTP requests.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetSummary handles GET /api/v1/dashboard/summary.
func (h *Handler) GetSummary(c *gin.Context) {
	storeID, exists := c.Get("current_store_id")
	if !exists {
		// Fallback: try store_id from JWT
		storeID, exists = c.Get("store_id")
		if !exists {
			response.Error(c, apperr.BadRequest("缺少门店信息"))
			return
		}
	}

	summary, err := h.svc.GetSummary(storeID.(int64))
	if err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, summary)
}
