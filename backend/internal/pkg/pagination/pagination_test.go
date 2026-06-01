package pagination

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func TestParseDefaults(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	page, pageSize := Parse(c)
	if page != 1 {
		t.Errorf("default page = %d, want 1", page)
	}
	if pageSize != 20 {
		t.Errorf("default pageSize = %d, want 20", pageSize)
	}
}

func TestParseValues(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/?page=3&page_size=10", nil)

	page, pageSize := Parse(c)
	if page != 3 {
		t.Errorf("page = %d, want 3", page)
	}
	if pageSize != 10 {
		t.Errorf("pageSize = %d, want 10", pageSize)
	}
}

func TestParseMaxPageSize(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/?page_size=500", nil)

	_, pageSize := Parse(c)
	if pageSize != 100 {
		t.Errorf("pageSize = %d, want 100 (max)", pageSize)
	}
}

func TestOffset(t *testing.T) {
	if got := Offset(1, 20); got != 0 {
		t.Errorf("Offset(1,20) = %d, want 0", got)
	}
	if got := Offset(3, 10); got != 20 {
		t.Errorf("Offset(3,10) = %d, want 20", got)
	}
}
