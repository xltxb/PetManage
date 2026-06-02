package appointment

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"pawprint/backend/internal/module/notification"
)

// mockRepo implements Repository for testing
type mockRepo struct {
	appointments   map[int64]*Appointment
	nextID         int64
	conflictExists bool
	findErr        error
	createErr      error
	updateErr      error
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		appointments: make(map[int64]*Appointment),
		nextID:       1,
	}
}

func (m *mockRepo) FindByID(id int64) (*Appointment, error) {
	a, ok := m.appointments[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return a, m.findErr
}

func (m *mockRepo) FindByIDWithStore(id, storeID int64) (*Appointment, error) {
	a, ok := m.appointments[id]
	if !ok || a.StoreID != storeID {
		return nil, gorm.ErrRecordNotFound
	}
	return a, m.findErr
}

func (m *mockRepo) CheckResourceConflict(storeID, stationID int64, start, end time.Time, excludeID int64) (bool, error) {
	return m.conflictExists, nil
}

func (m *mockRepo) Create(a *Appointment) error {
	if m.createErr != nil {
		return m.createErr
	}
	a.ID = m.nextID
	m.nextID++
	m.appointments[a.ID] = a
	return nil
}

func (m *mockRepo) Update(a *Appointment) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.appointments[a.ID] = a
	return nil
}

func (m *mockRepo) CreateItems(items []AppointmentItem) error                { return nil }
func (m *mockRepo) FindItems(appointmentID int64) ([]AppointmentItem, error) { return nil, nil }
func (m *mockRepo) ListByStore(storeID int64, status string, start, end time.Time, page, pageSize int) ([]Appointment, int64, error) {
	return nil, 0, nil
}

type fakeNotifier struct {
	sent []notification.SendRequest
}

func (f *fakeNotifier) Send(req notification.SendRequest) error {
	f.sent = append(f.sent, req)
	return nil
}

// --- State Machine Tests ---

func TestStateMachineValidTransitions(t *testing.T) {
	tests := []struct {
		from   string
		action string
		valid  bool
	}{
		{"pending", "arrive", true},
		{"pending", "cancel", true},
		{"pending", "no_show", true},
		{"arrived", "start", true},
		{"arrived", "cancel", true},
		{"in_progress", "complete", true},
		// Invalid transitions
		{"pending", "complete", false},
		{"completed", "cancel", false},
		{"cancelled", "arrive", false},
		{"no_show", "arrive", false},
		{"in_progress", "cancel", false},
	}

	for _, tt := range tests {
		t.Run(tt.from+"->"+tt.action, func(t *testing.T) {
			got := IsValidTransition(tt.from, tt.action)
			if got != tt.valid {
				t.Errorf("IsValidTransition(%s, %s) = %v, want %v", tt.from, tt.action, got, tt.valid)
			}
		})
	}
}

func TestTransitionArrive(t *testing.T) {
	repo := newMockRepo()
	now := time.Now()
	repo.appointments[1] = &Appointment{
		ID: 1, StoreID: 1, Status: "pending",
		ScheduledStart: now.Add(time.Hour),
	}
	svc := NewService(repo)

	err := svc.Transition(1, 1, "arrive")
	if err != nil {
		t.Fatalf("Transition arrive error: %v", err)
	}
	if repo.appointments[1].Status != "arrived" {
		t.Errorf("Status = %q, want arrived", repo.appointments[1].Status)
	}
}

func TestTransitionInvalid(t *testing.T) {
	repo := newMockRepo()
	repo.appointments[1] = &Appointment{ID: 1, StoreID: 1, Status: "pending"}
	svc := NewService(repo)

	err := svc.Transition(1, 1, "complete")
	if err == nil {
		t.Fatal("expected error for invalid transition")
	}
}

func TestTransitionCompleteFlow(t *testing.T) {
	repo := newMockRepo()
	repo.appointments[1] = &Appointment{ID: 1, StoreID: 1, Status: "pending"}
	svc := NewService(repo)

	steps := []string{"arrive", "start", "complete"}
	for _, step := range steps {
		if err := svc.Transition(1, 1, step); err != nil {
			t.Fatalf("Transition %s error: %v", step, err)
		}
	}
	if repo.appointments[1].Status != "completed" {
		t.Errorf("final Status = %q, want completed", repo.appointments[1].Status)
	}
}

// --- Creation Tests ---

func TestCreateAppointment(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	start := time.Now().Add(2 * time.Hour)
	end := start.Add(90 * time.Minute)

	req := CreateAppointmentRequest{
		StoreID:        1,
		CustomerID:     1,
		PetID:          1,
		ScheduledStart: start,
		ScheduledEnd:   end,
		StationID:      1,
		StaffUserID:    4,
		TotalAmount:    26800,
		Items: []CreateAppointmentItem{
			{ServiceOfferingID: 1, ServiceName: "全套SPA·小型犬", Price: 26800, DurationMin: 90, StationID: 1},
		},
		CreatedBy: 3,
	}

	a, err := svc.Create(req)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if a.Status != "pending" {
		t.Errorf("Status = %q, want pending", a.Status)
	}
	if a.TotalAmount != 26800 {
		t.Errorf("TotalAmount = %d, want 26800", a.TotalAmount)
	}
}

func TestCreateAppointmentSendsConfirmationNotification(t *testing.T) {
	repo := newMockRepo()
	notifier := &fakeNotifier{}
	svc := NewService(repo, WithNotifier(notifier))
	start := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)

	appt, err := svc.Create(CreateAppointmentRequest{
		StoreID:        1,
		CustomerID:     100,
		PetID:          200,
		StationID:      10,
		ScheduledStart: start,
		Items: []CreateAppointmentItem{
			{ServiceOfferingID: 1, ServiceName: "全套SPA", Price: 26800, DurationMin: 90},
		},
	})
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}
	if appt.ScheduledEnd.Sub(start) != 90*time.Minute {
		t.Fatalf("scheduled end = %s, want 90m after start", appt.ScheduledEnd)
	}
	if len(notifier.sent) != 1 || notifier.sent[0].TemplateCode != "appointment_confirmed" {
		t.Fatalf("sent notifications = %#v", notifier.sent)
	}
}

func TestCreateAppointmentResourceConflict(t *testing.T) {
	repo := newMockRepo()
	repo.conflictExists = true
	svc := NewService(repo)

	start := time.Now().Add(2 * time.Hour)
	req := CreateAppointmentRequest{
		StoreID:        1,
		StationID:      1,
		ScheduledStart: start,
		ScheduledEnd:   start.Add(time.Hour),
		Items:          []CreateAppointmentItem{},
	}

	_, err := svc.Create(req)
	if err == nil {
		t.Fatal("expected resource conflict error")
	}
	if ae, ok := err.(interface{ CodeVal() int }); ok {
		_ = ae
	}
}

func TestCalculateTotalAmount(t *testing.T) {
	svc := NewService(newMockRepo())
	items := []CreateAppointmentItem{
		{Price: 26800, DurationMin: 90},
		{Price: 8800, DurationMin: 45},
	}
	amount, duration := svc.CalculateTotals(items)
	if amount != 35600 {
		t.Errorf("amount = %d, want 35600", amount)
	}
	if duration != 135 {
		t.Errorf("duration = %d, want 135", duration)
	}
}
