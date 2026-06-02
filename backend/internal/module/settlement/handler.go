package settlement

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/pagination"
	"pawprint/backend/internal/pkg/response"
)

// Handler processes settlement HTTP requests.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Create handles POST /api/v1/settlements.
func (h *Handler) Create(c *gin.Context) {
	var req CreateSettlementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("参数校验失败"))
		return
	}
	storeID, _ := c.Get("current_store_id")
	req.StoreID = storeID.(int64)

	s, err := h.svc.Create(req)
	if err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.SuccessCreated(c, s)
}

// Pay handles POST /api/v1/settlements/:id/pay.
func (h *Handler) Pay(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req PayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("参数校验失败"))
		return
	}

	if err := h.svc.Pay(id, req.Amount, req.Method, req.OperatorID); err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, nil)
}

// Refund handles POST /api/v1/settlements/:id/refund.
func (h *Handler) Refund(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req RefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("参数校验失败"))
		return
	}

	if err := h.svc.Refund(id, req.OperatorID, req.Reason); err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, nil)
}

// Void handles POST /api/v1/settlements/:id/void.
func (h *Handler) Void(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	if err := h.svc.Void(id); err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, nil)
}

// List handles GET /api/v1/settlements.
func (h *Handler) List(c *gin.Context) {
	storeID, _ := c.Get("current_store_id")
	status := c.Query("status")
	page, pageSize := pagination.Parse(c)

	list, total, err := h.svc.repo.ListByStore(storeID.(int64), status, page, pageSize)
	if err != nil {
		response.Error(c, apperr.Internal(err))
		return
	}
	response.List(c, list, total, page, pageSize)
}
