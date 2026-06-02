package appointment

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/pagination"
	"pawprint/backend/internal/pkg/response"
)

// Handler processes appointment HTTP requests.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Create handles POST /api/v1/appointments.
func (h *Handler) Create(c *gin.Context) {
	var req CreateAppointmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("参数校验失败: "+err.Error()))
		return
	}

	storeID, _ := c.Get("current_store_id")
	req.StoreID = storeID.(int64)

	a, err := h.svc.Create(req)
	if err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.SuccessCreated(c, a)
}

// Get handles GET /api/v1/appointments/:id.
func (h *Handler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, apperr.BadRequest("无效的预约ID"))
		return
	}

	storeID, _ := c.Get("current_store_id")

	a, err := h.svc.GetByID(id, storeID.(int64))
	if err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, a)
}

// List handles GET /api/v1/appointments.
func (h *Handler) List(c *gin.Context) {
	storeID, _ := c.Get("current_store_id")
	page, pageSize := pagination.Parse(c)

	var req ListRequest
	_ = c.ShouldBindQuery(&req) // ignore binding errors for optional params

	list, total, err := h.svc.List(storeID.(int64), req.Status, req.DateFrom, req.DateTo, page, pageSize)
	if err != nil {
		response.Error(c, apperr.Internal(err))
		return
	}
	response.List(c, list, total, page, pageSize)
}

// Transition handles POST /api/v1/appointments/:id/transitions.
func (h *Handler) Transition(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, apperr.BadRequest("无效的预约ID"))
		return
	}

	var req TransitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("请提供有效的操作类型"))
		return
	}

	storeID, _ := c.Get("current_store_id")

	if err := h.svc.Transition(id, storeID.(int64), req.Action); err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, nil)
}

// Cancel handles POST /api/v1/appointments/:id/cancel.
func (h *Handler) Cancel(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, apperr.BadRequest("无效的预约ID"))
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)

	storeID, _ := c.Get("current_store_id")

	if err := h.svc.Transition(id, storeID.(int64), ActionCancel); err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, nil)
}

// AvailableSlots handles GET /api/v1/appointments/available-slots.
// Returns available time slots for a station on a given day.
func (h *Handler) AvailableSlots(c *gin.Context) {
	stationID, _ := strconv.ParseInt(c.Query("station_id"), 10, 64)
	dateStr := c.Query("date") // YYYY-MM-DD

	if stationID == 0 || dateStr == "" {
		response.Error(c, apperr.BadRequest("请提供 station_id 和 date"))
		return
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		response.Error(c, apperr.BadRequest("日期格式无效，应为 YYYY-MM-DD"))
		return
	}

	storeID, _ := c.Get("current_store_id")

	slots, err := h.svc.GetAvailableSlots(storeID.(int64), stationID, date)
	if err != nil {
		response.Error(c, apperr.Internal(err))
		return
	}
	response.Success(c, slots)
}
