package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/errcode"
)

func init() { gin.SetMode(gin.TestMode) }

func createTestContext(w *httptest.ResponseRecorder) *gin.Context {
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	return c
}

func TestSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	c := createTestContext(w)

	Success(c, map[string]string{"key": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var body Response
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != 0 {
		t.Errorf("code = %d, want 0", body.Code)
	}
}

func TestSuccessCreated(t *testing.T) {
	w := httptest.NewRecorder()
	c := createTestContext(w)

	SuccessCreated(c, nil)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
}

func TestError(t *testing.T) {
	w := httptest.NewRecorder()
	c := createTestContext(w)

	Error(c, apperr.New(errcode.InsufficientStock))

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
	var body Response
	json.Unmarshal(w.Body.Bytes(), &body)
	if body.Code != errcode.InsufficientStock {
		t.Errorf("code = %d, want %d", body.Code, errcode.InsufficientStock)
	}
}

func TestList(t *testing.T) {
	w := httptest.NewRecorder()
	c := createTestContext(w)

	items := []string{"a", "b"}
	List(c, items, int64(len(items)), 1, 20)

	var body ListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != 0 {
		t.Errorf("code = %d", body.Code)
	}
	if body.Data.Total != 2 {
		t.Errorf("total = %d, want 2", body.Data.Total)
	}
}

func TestNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	r.NoRoute(func(ctx *gin.Context) { NoContent(ctx) })
	r.ServeHTTP(w, c.Request)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}
