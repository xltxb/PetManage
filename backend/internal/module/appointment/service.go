package appointment

import (
	"errors"
	"strconv"
	"time"

	"gorm.io/gorm"

	"pawprint/backend/internal/module/notification"
	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/errcode"
)

// Service handles appointment business logic.
type Service struct {
	repo     Repository
	notifier Notifier
}

type Notifier interface {
	Send(notification.SendRequest) error
}

type Option func(*Service)

func WithNotifier(n Notifier) Option {
	return func(s *Service) {
		s.notifier = n
	}
}

func NewService(repo Repository, opts ...Option) *Service {
	s := &Service{repo: repo}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Create creates a new appointment with items after validating no resource conflict.
func (s *Service) Create(req CreateAppointmentRequest) (*Appointment, error) {
	if req.StationID > 0 {
		conflict, err := s.repo.CheckResourceConflict(req.StoreID, req.StationID, req.ScheduledStart, req.ScheduledEnd, 0)
		if err != nil {
			return nil, apperr.Internal(err)
		}
		if conflict {
			return nil, apperr.New(errcode.ResourceConflict, "该工位在所选时段已被占用")
		}
	}

	// Calculate totals from items
	totalAmount, durationMin := s.CalculateTotals(req.Items)
	if req.TotalAmount == 0 {
		req.TotalAmount = totalAmount
	}
	if req.ScheduledEnd.IsZero() && durationMin > 0 {
		req.ScheduledEnd = req.ScheduledStart.Add(time.Duration(durationMin) * time.Minute)
	}

	var customerID, petID, stationID, staffUserID, createdBy *int64
	if req.CustomerID > 0 {
		customerID = &req.CustomerID
	}
	if req.PetID > 0 {
		petID = &req.PetID
	}
	if req.StationID > 0 {
		stationID = &req.StationID
	}
	if req.StaffUserID > 0 {
		staffUserID = &req.StaffUserID
	}
	if req.CreatedBy > 0 {
		createdBy = &req.CreatedBy
	}

	source := req.Source
	if source == 0 {
		source = 1 // default: backend
	}

	a := &Appointment{
		StoreID:        req.StoreID,
		CustomerID:     customerID,
		PetID:          petID,
		Source:         source,
		Status:         StatusPending,
		ScheduledStart: req.ScheduledStart,
		ScheduledEnd:   req.ScheduledEnd,
		StationID:      stationID,
		StaffUserID:    staffUserID,
		ContactName:    req.ContactName,
		ContactPhone:   req.ContactPhone,
		TotalAmount:    req.TotalAmount,
		Remark:         req.Remark,
		CreatedBy:      createdBy,
	}

	if err := s.repo.Create(a); err != nil {
		return nil, apperr.Internal(err)
	}

	// Create appointment items
	items := make([]AppointmentItem, len(req.Items))
	for i, item := range req.Items {
		var itemStationID *int64
		if item.StationID > 0 {
			sid := item.StationID
			itemStationID = &sid
		}
		items[i] = AppointmentItem{
			AppointmentID:     a.ID,
			ServiceOfferingID: item.ServiceOfferingID,
			ServiceName:       item.ServiceName,
			Price:             item.Price,
			DurationMin:       item.DurationMin,
			StationID:         itemStationID,
		}
	}
	if err := s.repo.CreateItems(items); err != nil {
		return nil, apperr.Internal(err)
	}

	if s.notifier != nil && a.CustomerID != nil {
		_ = s.notifier.Send(notification.SendRequest{
			StoreID:      a.StoreID,
			CustomerID:   *a.CustomerID,
			TemplateCode: "appointment_confirmed",
			Channel:      notification.ChannelInApp,
			Payload: map[string]string{
				"appointment_id": strconv.FormatInt(a.ID, 10),
				"start_at":       a.ScheduledStart.Format(time.RFC3339),
			},
		})
	}

	return a, nil
}

// Transition performs a state machine transition on an appointment.
func (s *Service) Transition(id, storeID int64, action string) error {
	a, err := s.repo.FindByIDWithStore(id, storeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.NotFound("预约不存在")
		}
		return apperr.Internal(err)
	}

	if !IsValidTransition(a.Status, action) {
		return apperr.New(errcode.StateTransitionInvalid,
			"预约状态不可从 "+statusLabel(a.Status)+" 变更为 "+actionLabel(action))
	}

	a.Status = targetStatus(action)
	if action == ActionCancel {
		a.CancelledReason = "" // set by handler if provided
	}

	if err := s.repo.Update(a); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

// GetByID returns an appointment by ID with store validation.
func (s *Service) GetByID(id, storeID int64) (*Appointment, error) {
	a, err := s.repo.FindByIDWithStore(id, storeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("预约不存在")
		}
		return nil, apperr.Internal(err)
	}
	return a, nil
}

// List returns a paginated list of appointments for a store.
func (s *Service) List(storeID int64, status string, start, end time.Time, page, pageSize int) ([]Appointment, int64, error) {
	return s.repo.ListByStore(storeID, status, start, end, page, pageSize)
}

func (s *Service) GetWeekSchedule(storeID, stationID int64, weekStart time.Time) (*WeekScheduleResponse, error) {
	start := dateStartUTC(weekStart)
	end := start.AddDate(0, 0, 7)
	list, _, err := s.repo.ListByStore(storeID, "", start, end, 1, 500)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	days := make([]WeekScheduleDay, 7)
	dayIndex := make(map[string]int, 7)
	for i := 0; i < 7; i++ {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		days[i] = WeekScheduleDay{Date: date, Appointments: []WeekScheduleAppointment{}}
		dayIndex[date] = i
	}

	for _, a := range list {
		if a.StationID == nil || *a.StationID != stationID {
			continue
		}
		date := a.ScheduledStart.UTC().Format("2006-01-02")
		idx, ok := dayIndex[date]
		if !ok {
			continue
		}
		days[idx].Appointments = append(days[idx].Appointments, WeekScheduleAppointment{
			ID:             a.ID,
			Status:         a.Status,
			ScheduledStart: a.ScheduledStart,
			ScheduledEnd:   a.ScheduledEnd,
			CustomerID:     a.CustomerID,
			PetID:          a.PetID,
			ContactName:    a.ContactName,
			TotalAmount:    a.TotalAmount,
		})
	}

	return &WeekScheduleResponse{
		StationID: stationID,
		WeekStart: start.Format("2006-01-02"),
		WeekEnd:   end.Format("2006-01-02"),
		Days:      days,
	}, nil
}

// CalculateTotals computes total amount and duration from appointment items.
func (s *Service) CalculateTotals(items []CreateAppointmentItem) (amount int64, durationMin int) {
	for _, item := range items {
		amount += item.Price
		durationMin += item.DurationMin
	}
	return
}

// GetAvailableSlots returns available 30-minute time slots for a station on a date.
func (s *Service) GetAvailableSlots(storeID, stationID int64, date time.Time) ([]string, error) {
	// Business hours 09:00-21:00 (from system_settings, default here)
	loc, _ := time.LoadLocation("Asia/Shanghai")
	openTime := time.Date(date.Year(), date.Month(), date.Day(), 9, 0, 0, 0, loc)
	closeTime := time.Date(date.Year(), date.Month(), date.Day(), 21, 0, 0, 0, loc)

	// Get existing appointments for the station on this day
	var existing []Appointment
	existing, _, err := s.repo.ListByStore(storeID, "", openTime, closeTime, 1, 200)
	if err != nil {
		return nil, err
	}

	// Filter to this station
	var blocked []timeBlock
	for _, a := range existing {
		if a.StationID != nil && *a.StationID == stationID {
			blocked = append(blocked, timeBlock{a.ScheduledStart, a.ScheduledEnd})
		}
	}

	// Generate 30-min slots
	var slots []string
	for t := openTime; t.Before(closeTime); t = t.Add(30 * time.Minute) {
		slotEnd := t.Add(30 * time.Minute)
		if slotEnd.After(closeTime) {
			break
		}
		if !isBlocked(t, slotEnd, blocked) {
			slots = append(slots, t.Format("15:04")+"-"+slotEnd.Format("15:04"))
		}
	}
	// Introduce a realistic gap around blocked times to prevent tight booking
	var filtered []string
	for _, s := range slots {
		parts := parseSlot(s) // "09:00-09:30"
		if !isBlocked(parts[0], parts[1], blocked) {
			filtered = append(filtered, s)
		}
	}
	if len(filtered) > 0 {
		return filtered, nil
	}
	return slots, nil
}

type timeBlock struct{ start, end time.Time }

func dateStartUTC(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

func isBlocked(start, end time.Time, blocked []timeBlock) bool {
	for _, b := range blocked {
		if start.Before(b.end) && end.After(b.start) {
			return true
		}
	}
	return false
}

func parseSlot(s string) []time.Time {
	// s is "09:00-09:30", return start and end as time.Time
	// Use a fixed date for parsing
	parts := []string{s[:5], s[6:]}
	t1, _ := time.Parse("15:04", parts[0])
	t2, _ := time.Parse("15:04", parts[1])
	return []time.Time{t1, t2}
}

func statusLabel(s string) string {
	switch s {
	case StatusPending:
		return "待到店"
	case StatusArrived:
		return "已到店"
	case StatusInProgress:
		return "进行中"
	case StatusCompleted:
		return "已完成"
	case StatusCancelled:
		return "已取消"
	case StatusNoShow:
		return "未到店"
	default:
		return s
	}
}

func actionLabel(a string) string {
	switch a {
	case ActionArrive:
		return "到店"
	case ActionStart:
		return "开始"
	case ActionComplete:
		return "完成"
	case ActionCancel:
		return "取消"
	case ActionNoShow:
		return "标记未到"
	default:
		return a
	}
}
