package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func TestGetSummaryHandler(t *testing.T) {
	repo := &mockRepo{
		revenueToday:     50000,
		appointmentCount: 3,
		petsInStore:      5,
	}

	svc := NewService(repo, "Asia/Shanghai")
	handler := NewHandler(svc)

	r := gin.New()
	r.GET("/api/v1/dashboard/summary", func(c *gin.Context) {
		c.Set("current_store_id", int64(1))
		handler.GetSummary(c)
	})

	req := httptest.NewRequest("GET", "/api/v1/dashboard/summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp struct {
		Code int              `json:"code"`
		Data DashboardSummary `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("code = %d, want 0", resp.Code)
	}
	if resp.Data.RevenueToday != 50000 {
		t.Errorf("RevenueToday = %d, want 50000", resp.Data.RevenueToday)
	}
}

func TestGetSummaryHandlerNoStore(t *testing.T) {
	repo := &mockRepo{}
	svc := NewService(repo, "Asia/Shanghai")
	handler := NewHandler(svc)

	r := gin.New()
	r.GET("/api/v1/dashboard/summary", handler.GetSummary)

	req := httptest.NewRequest("GET", "/api/v1/dashboard/summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
