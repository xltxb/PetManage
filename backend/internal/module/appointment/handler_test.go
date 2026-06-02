package appointment

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

func setupAppointmentRouter(svc *Service) *gin.Engine {
	r := gin.New()
	h := NewHandler(svc)
	group := r.Group("/api/v1")
	group.Use(func(c *gin.Context) {
		c.Set("current_store_id", int64(1))
		c.Next()
	})
	group.POST("/appointments", h.Create)
	group.POST("/appointments/:id/transitions", h.Transition)
	group.GET("/appointments/:id", h.Get)
	return r
}

func TestCreateAppointmentHandler(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)
	router := setupAppointmentRouter(svc)

	start := time.Now().Add(2 * time.Hour)
	body, _ := json.Marshal(CreateAppointmentRequest{
		CustomerID:     1,
		PetID:          1,
		ScheduledStart: start,
		ScheduledEnd:   start.Add(90 * time.Minute),
		StationID:      1,
		TotalAmount:    26800,
		Items: []CreateAppointmentItem{
			{ServiceOfferingID: 1, ServiceName: "全套SPA", Price: 26800, DurationMin: 90},
		},
	})

	req := httptest.NewRequest("POST", "/api/v1/appointments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
}

func TestCreateAppointmentHandlerConflict(t *testing.T) {
	repo := newMockRepo()
	repo.conflictExists = true
	svc := NewService(repo)
	router := setupAppointmentRouter(svc)

	start := time.Now().Add(2 * time.Hour)
	body, _ := json.Marshal(CreateAppointmentRequest{
		ScheduledStart: start,
		ScheduledEnd:   start.Add(time.Hour),
		StationID:      1,
		Items:          []CreateAppointmentItem{},
	})

	req := httptest.NewRequest("POST", "/api/v1/appointments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestTransitionHandler(t *testing.T) {
	repo := newMockRepo()
	repo.appointments[1] = &Appointment{ID: 1, StoreID: 1, Status: "pending"}
	svc := NewService(repo)
	router := setupAppointmentRouter(svc)

	body, _ := json.Marshal(TransitionRequest{Action: "arrive"})
	req := httptest.NewRequest("POST", "/api/v1/appointments/1/transitions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if repo.appointments[1].Status != "arrived" {
		t.Errorf("status = %q, want arrived", repo.appointments[1].Status)
	}
}

func TestGetAppointmentHandler(t *testing.T) {
	repo := newMockRepo()
	repo.appointments[1] = &Appointment{ID: 1, StoreID: 1, Status: "pending"}
	svc := NewService(repo)
	router := setupAppointmentRouter(svc)

	req := httptest.NewRequest("GET", "/api/v1/appointments/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}
