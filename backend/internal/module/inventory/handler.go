package inventory

import (
	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/response"
)

// Handler processes inventory HTTP requests.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// SaleOut handles POST /api/v1/inventory/sale-out.
func (h *Handler) SaleOut(c *gin.Context) {
	var req struct {
		ProductID  int64  `json:"product_id" binding:"required"`
		Quantity   int    `json:"quantity" binding:"required"`
		RefType    string `json:"ref_type"`
		RefID      int64  `json:"ref_id"`
		OperatorID int64  `json:"operator_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("参数校验失败"))
		return
	}
	storeID, _ := c.Get("current_store_id")

	if err := h.svc.SaleOut(storeID.(int64), req.ProductID, req.Quantity, req.OperatorID, req.RefType, req.RefID); err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, nil)
}

// PurchaseIn handles POST /api/v1/inventory/purchase-in.
func (h *Handler) PurchaseIn(c *gin.Context) {
	var req struct {
		ProductID  int64  `json:"product_id" binding:"required"`
		Quantity   int    `json:"quantity" binding:"required"`
		RefType    string `json:"ref_type"`
		RefID      int64  `json:"ref_id"`
		OperatorID int64  `json:"operator_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("参数校验失败"))
		return
	}
	storeID, _ := c.Get("current_store_id")

	if err := h.svc.PurchaseIn(storeID.(int64), req.ProductID, req.Quantity, req.OperatorID, req.RefType, req.RefID); err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, nil)
}

// Adjust handles POST /api/v1/inventory/adjust.
func (h *Handler) Adjust(c *gin.Context) {
	var req struct {
		ProductID  int64  `json:"product_id" binding:"required"`
		Delta      int    `json:"delta" binding:"required"`
		OperatorID int64  `json:"operator_id"`
		Remark     string `json:"remark" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("参数校验失败"))
		return
	}
	storeID, _ := c.Get("current_store_id")

	if err := h.svc.Adjust(storeID.(int64), req.ProductID, req.Delta, req.OperatorID, req.Remark); err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, nil)
}

// Alerts handles GET /api/v1/inventory/alerts.
func (h *Handler) Alerts(c *gin.Context) {
	storeID, _ := c.Get("current_store_id")
	alerts, err := h.svc.repo.CheckSafetyStock(storeID.(int64))
	if err != nil {
		response.Error(c, apperr.Internal(err))
		return
	}
	if alerts == nil { alerts = []InventoryAlert{} }
	response.Success(c, alerts)
}
