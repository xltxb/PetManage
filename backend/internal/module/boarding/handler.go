package boarding

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/pagination"
	"pawprint/backend/internal/pkg/response"
)

// Handler processes boarding HTTP requests.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// CheckIn handles POST /api/v1/boarding-orders/check-in.
func (h *Handler) CheckIn(c *gin.Context) {
	var req CheckInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("参数校验失败"))
		return
	}
	storeID, _ := c.Get("current_store_id")
	req.StoreID = storeID.(int64)

	order, err := h.svc.CheckIn(req)
	if err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.SuccessCreated(c, order)
}

// CheckOut handles POST /api/v1/boarding-orders/:id/check-out.
func (h *Handler) CheckOut(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, apperr.BadRequest("无效的订单ID"))
		return
	}
	storeID, _ := c.Get("current_store_id")

	resp, err := h.svc.CheckOut(id, storeID.(int64))
	if err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Cancel(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, apperr.BadRequest("无效的订单ID"))
		return
	}

	var req CancelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("参数校验失败"))
		return
	}

	storeID, _ := c.Get("current_store_id")
	if err := h.svc.Cancel(id, storeID.(int64), req.OperatorID, req.Reason); err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, nil)
}

// List handles GET /api/v1/boarding-orders.
func (h *Handler) List(c *gin.Context) {
	storeID, _ := c.Get("current_store_id")
	status := c.Query("status")
	page, pageSize := pagination.Parse(c)

	list, total, err := h.svc.ListOrders(storeID.(int64), status, page, pageSize)
	if err != nil {
		response.Error(c, apperr.Internal(err))
		return
	}
	response.List(c, list, total, page, pageSize)
}

// Get handles GET /api/v1/boarding-orders/:id.
func (h *Handler) Get(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	storeID, _ := c.Get("current_store_id")

	order, err := h.svc.GetOrder(id, storeID.(int64))
	if err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, order)
}

// PostCareLog handles POST /api/v1/boarding-orders/:id/care-logs.
func (h *Handler) PostCareLog(c *gin.Context) {
	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, apperr.BadRequest("无效的订单ID"))
		return
	}

	var req CareLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("参数校验失败"))
		return
	}

	storeID, _ := c.Get("current_store_id")

	if err := h.svc.LogCare(orderID, storeID.(int64), req.Task, req.Status, req.Note, req.OperatorID); err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.SuccessCreated(c, nil)
}

// GetCareLogs handles GET /api/v1/boarding-orders/:id/care-logs.
func (h *Handler) GetCareLogs(c *gin.Context) {
	orderID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	logs, err := h.svc.GetCareLogs(orderID)
	if err != nil {
		response.Error(c, apperr.Internal(err))
		return
	}
	response.Success(c, logs)
}
