package pet

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/response"
)

// Handler processes pet HTTP requests.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Create handles POST /api/v1/pets.
func (h *Handler) Create(c *gin.Context) {
	var req CreatePetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("参数校验失败"))
		return
	}
	p, err := h.svc.Create(req)
	if err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.SuccessCreated(c, p)
}

// Get handles GET /api/v1/pets/:id.
func (h *Handler) Get(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	detail, err := h.svc.GetDetail(id)
	if err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, detail)
}

// AddHealthRecord handles POST /api/v1/pets/:id/health.
func (h *Handler) AddHealthRecord(c *gin.Context) {
	petID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req HealthRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("参数校验失败"))
		return
	}
	if err := h.svc.AddHealthRecord(petID, req); err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.SuccessCreated(c, nil)
}

// AddWeightRecord handles POST /api/v1/pets/:id/weights.
func (h *Handler) AddWeightRecord(c *gin.Context) {
	petID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req WeightRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("参数校验失败"))
		return
	}
	if err := h.svc.AddWeightRecord(petID, req.WeightG); err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.SuccessCreated(c, nil)
}

// ListByCustomer handles GET /api/v1/customers/:id/pets.
func (h *Handler) ListByCustomer(c *gin.Context) {
	customerID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	list, err := h.svc.ListByCustomer(customerID)
	if err != nil {
		response.Error(c, apperr.Internal(err))
		return
	}
	if list == nil { list = []Pet{} }
	response.Success(c, list)
}
