package wx

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"pawprint/backend/internal/module/appointment"
	"pawprint/backend/internal/module/member"
)

type fakeRepo struct {
	customer *member.Customer
	offering *ServiceOffering
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		offering: &ServiceOffering{ID: 8, Name: "全套SPA", Price: 26800, DurationMin: 90},
	}
}

func (f *fakeRepo) FindCustomerByOpenID(openID string) (*member.Customer, error) {
	if f.customer == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return f.customer, nil
}

func (f *fakeRepo) CreateCustomer(c *member.Customer) error {
	c.ID = 1
	f.customer = c
	return nil
}

func (f *fakeRepo) ListBookableOfferings(storeID int64) ([]ServiceOffering, error) {
	return []ServiceOffering{*f.offering}, nil
}

func (f *fakeRepo) FindOffering(id, storeID int64) (*ServiceOffering, error) {
	if f.offering == nil || f.offering.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	return f.offering, nil
}

type fakeAppointmentCreator struct {
	req appointment.CreateAppointmentRequest
}

func (f *fakeAppointmentCreator) Create(req appointment.CreateAppointmentRequest) (*appointment.Appointment, error) {
	f.req = req
	return &appointment.Appointment{ID: 99, StoreID: req.StoreID, CustomerID: &req.CustomerID, Source: req.Source}, nil
}

type fakeSettingsProvider struct {
	settings map[string]interface{}
	err      error
}

func (f fakeSettingsProvider) GetAll(storeID int64) (map[string]interface{}, error) {
	return f.settings, f.err
}

func TestMockLoginCreatesCustomerForCode(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, nil)

	resp, err := svc.MockLogin(LoginRequest{Code: "mock-openid-001", StoreID: 1})
	if err != nil {
		t.Fatalf("MockLogin error = %v", err)
	}
	if resp.CustomerID == 0 || resp.Token == "" {
		t.Fatalf("response = %#v", resp)
	}
	if repo.customer.Source != 2 {
		t.Fatalf("customer source = %d, want 2", repo.customer.Source)
	}
}

func TestCreateAppointmentUsesAppointmentRules(t *testing.T) {
	repo := newFakeRepo()
	appt := &fakeAppointmentCreator{}
	svc := NewService(repo, appt)
	start := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)

	_, err := svc.CreateAppointment(1, CreateAppointmentRequest{
		StoreID: 1, PetID: 2, ScheduledStart: start, ServiceOfferingID: 8,
	})
	if err != nil {
		t.Fatalf("CreateAppointment error = %v", err)
	}
	if appt.req.Source != 2 || len(appt.req.Items) != 1 {
		t.Fatalf("appointment request = %#v", appt.req)
	}
}

func TestCreateAppointmentRejectsWhenOnlineBookingDisabled(t *testing.T) {
	repo := newFakeRepo()
	appt := &fakeAppointmentCreator{}
	svc := NewService(repo, appt, WithSettings(fakeSettingsProvider{
		settings: map[string]interface{}{"feature.online_booking_enabled": false},
	}))

	_, err := svc.CreateAppointment(1, CreateAppointmentRequest{
		StoreID: 1, PetID: 2, ScheduledStart: time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC), ServiceOfferingID: 8,
	})
	if err == nil {
		t.Fatal("expected online booking disabled error")
	}
	if appt.req.StoreID != 0 {
		t.Fatalf("appointment should not be created when online booking is disabled: %#v", appt.req)
	}
}

func TestCancelAppointmentRejectsLateCancellation(t *testing.T) {
	svc := NewService(newFakeRepo(), &fakeLateCancelAppointmentService{
		appt: &appointment.Appointment{ID: 99, StoreID: 1, ScheduledStart: time.Now().UTC().Add(90 * time.Minute)},
	})

	err := svc.CancelAppointment(1, 99, time.Now().UTC())
	if err == nil {
		t.Fatal("expected late cancellation error")
	}
}

func TestCancelAppointmentUsesConfiguredDeadline(t *testing.T) {
	svc := NewService(newFakeRepo(), &fakeLateCancelAppointmentService{
		appt: &appointment.Appointment{ID: 99, StoreID: 1, ScheduledStart: time.Now().UTC().Add(90 * time.Minute)},
	}, WithSettings(fakeSettingsProvider{
		settings: map[string]interface{}{"appointment.cancel_deadline_hours": float64(1)},
	}))

	err := svc.CancelAppointment(1, 99, time.Now().UTC())
	if err != nil {
		t.Fatalf("CancelAppointment error = %v", err)
	}
}

type fakeLateCancelAppointmentService struct {
	appt *appointment.Appointment
}

func (f *fakeLateCancelAppointmentService) Create(req appointment.CreateAppointmentRequest) (*appointment.Appointment, error) {
	return nil, errors.New("not used")
}

func (f *fakeLateCancelAppointmentService) GetByID(id, storeID int64) (*appointment.Appointment, error) {
	return f.appt, nil
}

func (f *fakeLateCancelAppointmentService) Transition(id, storeID int64, action string) error {
	return nil
}
