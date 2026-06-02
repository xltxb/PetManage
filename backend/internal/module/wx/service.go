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
}

func NewService(repo Repository, appointments AppointmentCreator) *Service {
	return &Service{repo: repo, appointments: appointments}
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
	if appt.ScheduledStart.Sub(now.UTC()) < 2*time.Hour {
		return apperr.BadRequest("距预约开始不足2小时，不能自助取消")
	}
	return manager.Transition(appointmentID, storeID, appointment.ActionCancel)
}

func wxPhone(code string) string {
	phone := "wx" + code
	if len(phone) > 20 {
		return phone[:20]
	}
	return phone
}
