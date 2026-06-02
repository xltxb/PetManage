package member

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/pagination"
	"pawprint/backend/internal/pkg/response"
)

// Handler processes member HTTP requests.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// List handles GET /api/v1/customers.
func (h *Handler) List(c *gin.Context) {
	storeID, _ := c.Get("current_store_id")
	keyword := c.Query("keyword")
	page, pageSize := pagination.Parse(c)

	list, total, err := h.svc.ListCustomers(storeID.(int64), keyword, page, pageSize)
	if err != nil {
		response.Error(c, apperr.Internal(err))
		return
	}
	response.List(c, list, total, page, pageSize)
}

// Get handles GET /api/v1/customers/:id.
func (h *Handler) Get(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	customer, err := h.svc.GetCustomer(id)
	if err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, customer)
}

// Recharge handles POST /api/v1/customers/:id/wallet.
func (h *Handler) Recharge(c *gin.Context) {
	customerID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req WalletRechargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("参数校验失败"))
		return
	}

	storeID, _ := c.Get("current_store_id")
	if req.StoreID == 0 {
		req.StoreID = storeID.(int64)
	}

	if err := h.svc.Recharge(customerID, req.Amount, req.StoreID, req.OperatorID, req.Remark); err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, nil)
}

// AdjustWallet handles PUT /api/v1/customers/:id/wallet.
func (h *Handler) AdjustWallet(c *gin.Context) {
	customerID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req WalletAdjustRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("参数校验失败"))
		return
	}

	storeID, _ := c.Get("current_store_id")

	if err := h.svc.WalletAdjust(customerID, req.Amount, storeID.(int64), req.OperatorID, req.Remark); err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, nil)
}
