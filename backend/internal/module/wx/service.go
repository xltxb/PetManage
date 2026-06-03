package wx

import (
	"errors"
	"strconv"
	"time"

	"gorm.io/gorm"

	"pawprint/backend/internal/module/appointment"
	"pawprint/backend/internal/module/member"
	"pawprint/backend/internal/pkg/apperr"
)

type AppointmentCreator interface {
	Create(appointment.CreateAppointmentRequest) (*appointment.Appointment, error)
}

type appointmentManager interface {
	AppointmentCreator
	GetByID(id, storeID int64) (*appointment.Appointment, error)
	Transition(id, storeID int64, action string) error
}

type Service struct {
	repo         Repository
	appointments AppointmentCreator
	settings     SettingsProvider
}

type SettingsProvider interface {
	GetAll(storeID int64) (map[string]interface{}, error)
}

type Option func(*Service)

func WithSettings(p SettingsProvider) Option {
	return func(s *Service) {
		s.settings = p
	}
}

func NewService(repo Repository, appointments AppointmentCreator, opts ...Option) *Service {
	s := &Service{repo: repo, appointments: appointments}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) MockLogin(req LoginRequest) (*LoginResponse, error) {
	customer, err := s.repo.FindCustomerByOpenID(req.Code)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		customer = &member.Customer{
			Name:            "微信顾客",
			Phone:           wxPhone(req.Code),
			Source:          2,
			WechatOpenID:    req.Code,
			RegisterStoreID: &req.StoreID,
		}
		if err := s.repo.CreateCustomer(customer); err != nil {
			return nil, apperr.Internal(err)
		}
	} else if err != nil {
		return nil, apperr.Internal(err)
	}

	return &LoginResponse{
		CustomerID: customer.ID,
		Token:      "mock-wx-" + strconv.FormatInt(customer.ID, 10),
	}, nil
}

func (s *Service) ListServiceOfferings(storeID int64) ([]ServiceOffering, error) {
	offerings, err := s.repo.ListBookableOfferings(storeID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return offerings, nil
}

func (s *Service) CreateAppointment(customerID int64, req CreateAppointmentRequest) (*appointment.Appointment, error) {
	if s.appointments == nil {
		return nil, apperr.Internal(errors.New("appointment service not configured"))
	}
	enabled, err := s.onlineBookingEnabled(req.StoreID)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, apperr.BadRequest("线上预约已关闭，请联系门店预约")
	}
	offering, err := s.repo.FindOffering(req.ServiceOfferingID, req.StoreID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("服务项目不存在或不可线上预约")
		}
		return nil, apperr.Internal(err)
	}

	return s.appointments.Create(appointment.CreateAppointmentRequest{
		StoreID:        req.StoreID,
		CustomerID:     customerID,
		PetID:          req.PetID,
		Source:         2,
		ScheduledStart: req.ScheduledStart,
		Items: []appointment.CreateAppointmentItem{{
			ServiceOfferingID: offering.ID,
			ServiceName:       offering.Name,
			Price:             offering.Price,
			DurationMin:       offering.DurationMin,
		}},
	})
}

func (s *Service) CancelAppointment(storeID, appointmentID int64, now time.Time) error {
	manager, ok := s.appointments.(appointmentManager)
	if !ok {
		return apperr.Internal(errors.New("appointment cancellation service not configured"))
	}
	appt, err := manager.GetByID(appointmentID, storeID)
	if err != nil {
		return err
	}
	deadlineHours, err := s.cancelDeadlineHours(storeID)
	if err != nil {
		return err
	}
	if appt.ScheduledStart.Sub(now.UTC()) < time.Duration(deadlineHours)*time.Hour {
		return apperr.BadRequest("距预约开始不足" + strconv.Itoa(deadlineHours) + "小时，不能自助取消")
	}
	return manager.Transition(appointmentID, storeID, appointment.ActionCancel)
}

func (s *Service) onlineBookingEnabled(storeID int64) (bool, error) {
	settings, err := s.storeSettings(storeID)
	if err != nil {
		return false, err
	}
	value, ok := settings["feature.online_booking_enabled"].(bool)
	if !ok {
		return true, nil
	}
	return value, nil
}

func (s *Service) cancelDeadlineHours(storeID int64) (int, error) {
	settings, err := s.storeSettings(storeID)
	if err != nil {
		return 0, err
	}
	value, ok := numericSetting(settings["appointment.cancel_deadline_hours"])
	if !ok || value < 0 {
		return 2, nil
	}
	return int(value), nil
}

func (s *Service) storeSettings(storeID int64) (map[string]interface{}, error) {
	if s.settings == nil {
		return map[string]interface{}{}, nil
	}
	settings, err := s.settings.GetAll(storeID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return settings, nil
}

func numericSetting(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	default:
		return 0, false
	}
}

func wxPhone(code string) string {
	phone := "wx" + code
	if len(phone) > 20 {
		return phone[:20]
	}
	return phone
}
