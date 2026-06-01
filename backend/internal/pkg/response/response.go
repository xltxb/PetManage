package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/pkg/apperr"
)

// Response is the unified API response envelope.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ListResult holds paginated list data.
type ListResult struct {
	List     interface{} `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// ListResponse wraps a paginated list in the standard envelope.
type ListResponse struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    ListResult `json:"data"`
}

// Success sends a 200 OK response.
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

// SuccessCreated sends a 201 Created response.
func SuccessCreated(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

// NoContent sends a 204 No Content response.
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Error sends an error response based on the AppError.
func Error(c *gin.Context, ae *apperr.AppError) {
	c.AbortWithStatusJSON(ae.HTTPStatus, Response{
		Code:    ae.Code,
		Message: ae.Message,
	})
}

// List sends a paginated list response.
func List(c *gin.Context, list interface{}, total int64, page, pageSize int) {
	c.JSON(http.StatusOK, ListResponse{
		Code:    0,
		Message: "ok",
		Data: ListResult{
			List:     list,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}
