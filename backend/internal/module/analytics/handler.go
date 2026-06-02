package analytics

import (
	"time"

	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/response"
)

// Handler processes analytics HTTP requests.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetReport handles GET /api/v1/analytics/report.
func (h *Handler) GetReport(c *gin.Context) {
	storeID, _ := c.Get("current_store_id")

	startStr := c.Query("start")
	endStr := c.Query("end")

	start, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		start = time.Now().AddDate(-1, 0, 0)
	}
	end, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		end = time.Now()
	}

	report, err := h.svc.GetReport(storeID.(int64), start, end)
	if err != nil {
		response.Error(c, apperr.Internal(err))
		return
	}
	response.Success(c, report)
}
