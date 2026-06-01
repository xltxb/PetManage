package pagination

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// Parse extracts page and page_size from query parameters with defaults.
func Parse(c *gin.Context) (page int, pageSize int) {
	page = DefaultPage
	pageSize = DefaultPageSize

	if v, err := strconv.Atoi(c.Query("page")); err == nil && v > 0 {
		page = v
	}
	if v, err := strconv.Atoi(c.Query("page_size")); err == nil && v > 0 {
		if v > MaxPageSize {
			v = MaxPageSize
		}
		pageSize = v
	}
	return
}

// Offset calculates the database offset for a page and page size.
func Offset(page, pageSize int) int {
	return (page - 1) * pageSize
}
