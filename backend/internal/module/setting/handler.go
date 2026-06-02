package setting

import (
	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/response"
)

// Handler processes settings HTTP requests.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetAll handles GET /api/v1/settings.
func (h *Handler) GetAll(c *gin.Context) {
	storeID, _ := c.Get("current_store_id")
	settings, err := h.svc.GetAll(storeID.(int64))
	if err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, settings)
}

// Get handles GET /api/v1/settings/:key.
func (h *Handler) Get(c *gin.Context) {
	key := c.Param("key")
	storeID, _ := c.Get("current_store_id")
	val, err := h.svc.Get(storeID.(int64), key)
	if err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, val)
}

// Set handles PUT /api/v1/settings/:key.
func (h *Handler) Set(c *gin.Context) {
	key := c.Param("key")
	var body struct {
		Value     interface{} `json:"value" binding:"required"`
		UpdatedBy int64       `json:"updated_by"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, apperr.BadRequest("参数校验失败"))
		return
	}
	storeID, _ := c.Get("current_store_id")

	if err := h.svc.Set(storeID.(int64), key, body.Value, body.UpdatedBy); err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, nil)
}
