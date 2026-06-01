package auth

import (
	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/response"
)

// Handler processes auth HTTP requests.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Login handles POST /api/v1/auth/login.
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("请输入用户名和密码"))
		return
	}

	resp, err := h.svc.Login(req.Username, req.Password)
	if err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, resp)
}

// Refresh handles POST /api/v1/auth/refresh.
func (h *Handler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("请提供 refresh_token"))
		return
	}

	resp, err := h.svc.RefreshToken(req.RefreshToken)
	if err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, resp)
}

// SwitchStore handles POST /api/v1/auth/switch-store.
func (h *Handler) SwitchStore(c *gin.Context) {
	var req SwitchStoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("请提供 store_id"))
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, apperr.Unauthorized())
		return
	}

	resp, err := h.svc.SwitchStore(userID.(int64), req.StoreID)
	if err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, resp)
}

// Logout handles POST /api/v1/auth/logout.
func (h *Handler) Logout(c *gin.Context) {
	response.Success(c, nil)
}
