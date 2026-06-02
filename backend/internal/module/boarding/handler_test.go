package boarding

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func setupBoardingRouter(svc *Service) *gin.Engine {
	r := gin.New()
	h := NewHandler(svc)
	group := r.Group("/api/v1")
	group.Use(func(c *gin.Context) {
		c.Set("current_store_id", int64(1))
		c.Next()
	})
	group.POST("/boarding-orders/check-in", h.CheckIn)
	group.POST("/boarding-orders/:id/check-out", h.CheckOut)
	group.POST("/boarding-orders/:id/care-logs", h.PostCareLog)
	return r
}

func TestCheckInHandler(t *testing.T) {
	repo := newMockRepo()
	repo.rooms[5] = &BoardingRoom{ID: 5, StoreID: 1, Code: "S05", Status: RoomStatusFree}
	svc := NewService(repo)
	router := setupBoardingRouter(svc)

	body, _ := json.Marshal(CheckInRequest{
		CustomerID:     1,
		PetID:          1,
		RoomID:         5,
		RoomTypeCode:   "small",
		PricePerNight:  8800,
		PlannedCheckIn:  time.Now(),
		PlannedCheckOut: time.Now().Add(3 * 24 * time.Hour),
	})

	req := httptest.NewRequest("POST", "/api/v1/boarding-orders/check-in", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
}

func TestCheckInHandlerRoomNotFree(t *testing.T) {
	repo := newMockRepo()
	repo.rooms[1] = &BoardingRoom{ID: 1, StoreID: 1, Code: "S01", Status: RoomStatusOccupied}
	svc := NewService(repo)
	router := setupBoardingRouter(svc)

	body, _ := json.Marshal(CheckInRequest{RoomID: 1})
	req := httptest.NewRequest("POST", "/api/v1/boarding-orders/check-in", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestCheckOutHandler(t *testing.T) {
	repo := newMockRepo()
	checkIn := time.Now().Add(-2 * 24 * time.Hour)
	repo.orders[1] = &BoardingOrder{
		ID: 1, StoreID: 1, Status: StatusCheckedIn,
		PricePerNight: 12800, ActualCheckIn: &checkIn,
		RoomTypeSnapshot: "medium",
	}
	repo.rooms[10] = &BoardingRoom{ID: 10, StoreID: 1, Status: RoomStatusOccupied}
	repo.orders[1].RoomID = int64Ptr(10)
	svc := NewService(repo)
	router := setupBoardingRouter(svc)

	req := httptest.NewRequest("POST", "/api/v1/boarding-orders/1/check-out", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func int64Ptr(i int64) *int64 { return &i }
