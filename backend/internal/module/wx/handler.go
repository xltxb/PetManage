package wx

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/response"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("参数校验失败: "+err.Error()))
		return
	}

	resp, err := h.svc.MockLogin(req)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) ServiceOfferings(c *gin.Context) {
	storeID, err := strconv.ParseInt(c.Query("store_id"), 10, 64)
	if err != nil || storeID <= 0 {
		response.Error(c, apperr.BadRequest("请提供有效的 store_id"))
		return
	}

	offerings, err := h.svc.ListServiceOfferings(storeID)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, offerings)
}

func (h *Handler) CreateAppointment(c *gin.Context) {
	customerID, err := customerIDFromHeader(c)
	if err != nil {
		response.Error(c, apperr.Unauthorized("无效的小程序登录态"))
		return
	}

	var req CreateAppointmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("参数校验失败: "+err.Error()))
		return
	}

	appt, err := h.svc.CreateAppointment(customerID, req)
	if err != nil {
		writeError(c, err)
		return
	}
	response.SuccessCreated(c, appt)
}

func (h *Handler) CancelAppointment(c *gin.Context) {
	if _, err := customerIDFromHeader(c); err != nil {
		response.Error(c, apperr.Unauthorized("无效的小程序登录态"))
		return
	}
	storeID, err := strconv.ParseInt(c.Query("store_id"), 10, 64)
	if err != nil || storeID <= 0 {
		response.Error(c, apperr.BadRequest("请提供有效的 store_id"))
		return
	}
	appointmentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, apperr.BadRequest("无效的预约ID"))
		return
	}

	if err := h.svc.CancelAppointment(storeID, appointmentID, time.Now().UTC()); err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, nil)
}

func customerIDFromHeader(c *gin.Context) (int64, error) {
	raw := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer mock-wx-")
	return strconv.ParseInt(raw, 10, 64)
}

func writeError(c *gin.Context, err error) {
	if appErr, ok := err.(*apperr.AppError); ok {
		response.Error(c, appErr)
		return
	}
	response.Error(c, apperr.Internal(err))
}
