package schedule

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Shift type constants.
const (
	ShiftMorning = "morning"
	ShiftEvening = "evening"
	ShiftRest    = "rest"
)

// ShiftHours defines the time range for each shift type.
var ShiftHours = map[string]struct{ Start, End string }{
	ShiftMorning: {Start: "09:00", End: "17:00"},
	ShiftEvening: {Start: "13:00", End: "21:00"},
}

// Schedule represents an employee's schedule for a single day.
type Schedule struct {
	ID           int64     `json:"id"`
	MerchantID   int64     `json:"merchant_id"`
	EmployeeID   int64     `json:"employee_id"`
	ScheduleDate string    `json:"schedule_date"`
	ShiftType    string    `json:"shift_type"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// DaySchedule is a single day schedule entry used in batch requests.
type DaySchedule struct {
	Date      string `json:"date"`
	ShiftType string `json:"shift_type"`
}

// BatchSetRequest sets schedules for an employee across multiple days.
type BatchSetRequest struct {
	EmployeeID int64         `json:"employee_id"`
	Schedules  []DaySchedule `json:"schedules"`
}

// CopyWeekRequest copies schedules from one employee+week to another employee+week.
type CopyWeekRequest struct {
	FromEmployeeID int64  `json:"from_employee_id"`
	ToEmployeeID   int64  `json:"to_employee_id"`
	FromWeekStart  string `json:"from_week_start"`
	ToWeekStart    string `json:"to_week_start"`
}

// Service provides employee schedule management operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new schedule Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

const scheduleColumns = `id, merchant_id, employee_id, schedule_date, shift_type, created_at, updated_at`

func scanSchedule(row *sql.Row) (*Schedule, error) {
	s := &Schedule{}
	err := row.Scan(&s.ID, &s.MerchantID, &s.EmployeeID, &s.ScheduleDate, &s.ShiftType, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

func scanSchedules(rows *sql.Rows) ([]Schedule, error) {
	var out []Schedule
	for rows.Next() {
		var s Schedule
		if err := rows.Scan(&s.ID, &s.MerchantID, &s.EmployeeID, &s.ScheduleDate, &s.ShiftType, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan schedule", err)
		}
		out = append(out, s)
	}
	if out == nil {
		out = []Schedule{}
	}
	return out, rows.Err()
}

// Upsert creates or updates a single schedule entry for an employee on a date.
func (s *Service) Upsert(ctx context.Context, merchantID, employeeID int64, date, shiftType string) (*Schedule, error) {
	if shiftType != ShiftMorning && shiftType != ShiftEvening && shiftType != ShiftRest {
		return nil, apperrors.NewValidationError("invalid shift_type, must be morning/evening/rest")
	}

	var sch Schedule
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO employee_schedules (merchant_id, employee_id, schedule_date, shift_type)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (employee_id, schedule_date) WHERE deleted_at IS NULL
		 DO UPDATE SET shift_type = $4, updated_at = NOW()
		 RETURNING `+scheduleColumns,
		merchantID, employeeID, date, shiftType,
	).Scan(&sch.ID, &sch.MerchantID, &sch.EmployeeID, &sch.ScheduleDate, &sch.ShiftType, &sch.CreatedAt, &sch.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to upsert schedule", err)
	}
	return &sch, nil
}

// BatchSet sets schedules for an employee for multiple days at once.
func (s *Service) BatchSet(ctx context.Context, merchantID int64, req BatchSetRequest) ([]Schedule, error) {
	if req.EmployeeID <= 0 {
		return nil, apperrors.NewValidationError("employee_id is required")
	}
	if len(req.Schedules) == 0 {
		return nil, apperrors.NewValidationError("schedules must not be empty")
	}

	var result []Schedule
	for _, ds := range req.Schedules {
		sch, err := s.Upsert(ctx, merchantID, req.EmployeeID, ds.Date, ds.ShiftType)
		if err != nil {
			return nil, err
		}
		result = append(result, *sch)
	}
	return result, nil
}

// List returns schedule entries matching the given filters.
func (s *Service) List(ctx context.Context, merchantID, employeeID int64, startDate, endDate string) ([]Schedule, error) {
	var conditions []string
	var args []interface{}
	args = append(args, merchantID)
	conditions = append(conditions, "merchant_id = $1")
	conditions = append(conditions, "deleted_at IS NULL")
	argIdx := 2

	if employeeID > 0 {
		conditions = append(conditions, "employee_id = $"+itoa(argIdx))
		args = append(args, employeeID)
		argIdx++
	}
	if startDate != "" {
		conditions = append(conditions, "schedule_date >= $"+itoa(argIdx))
		args = append(args, startDate)
		argIdx++
	}
	if endDate != "" {
		conditions = append(conditions, "schedule_date <= $"+itoa(argIdx))
		args = append(args, endDate)
		argIdx++
	}

	query := `SELECT ` + scheduleColumns + ` FROM employee_schedules WHERE ` + strings.Join(conditions, " AND ") + ` ORDER BY schedule_date ASC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list schedules", err)
	}
	defer rows.Close()
	return scanSchedules(rows)
}

// CopyWeek copies schedules from one employee for a given week to another employee for another week.
func (s *Service) CopyWeek(ctx context.Context, merchantID int64, req CopyWeekRequest) error {
	if req.FromEmployeeID <= 0 || req.ToEmployeeID <= 0 {
		return apperrors.NewValidationError("from_employee_id and to_employee_id are required")
	}
	if req.FromWeekStart == "" || req.ToWeekStart == "" {
		return apperrors.NewValidationError("from_week_start and to_week_start are required")
	}

	fromStart, err := time.Parse("2006-01-02", req.FromWeekStart)
	if err != nil {
		return apperrors.NewValidationError("invalid from_week_start date format, use YYYY-MM-DD")
	}
	toStart, err := time.Parse("2006-01-02", req.ToWeekStart)
	if err != nil {
		return apperrors.NewValidationError("invalid to_week_start date format, use YYYY-MM-DD")
	}

	fromEnd := fromStart.AddDate(0, 0, 6)

	// Read source schedules.
	srcSchedules, err := s.List(ctx, merchantID, req.FromEmployeeID,
		fromStart.Format("2006-01-02"), fromEnd.Format("2006-01-02"))
	if err != nil {
		return err
	}

	// Calculate day offset between the two weeks.
	dayOffset := int(toStart.Sub(fromStart).Hours() / 24)

	for _, src := range srcSchedules {
		// schedule_date may come back as "2026-05-25T00:00:00Z" from the DATE column via Go driver.
		dateOnly := src.ScheduleDate
		if idx := strings.Index(dateOnly, "T"); idx > 0 {
			dateOnly = dateOnly[:idx]
		}
		srcDate, err := time.Parse("2006-01-02", dateOnly)
		if err != nil {
			continue
		}
		targetDate := srcDate.AddDate(0, 0, dayOffset).Format("2006-01-02")
		if _, err := s.Upsert(ctx, merchantID, req.ToEmployeeID, targetDate, src.ShiftType); err != nil {
			return err
		}
	}
	return nil
}

// OnDutyEmployee represents an employee who is on duty for a given time.
type OnDutyEmployee struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Position string `json:"position"`
}

// GetOnDutyEmployees returns employees who are on duty (not rest) at a specific appointment time.
func (s *Service) GetOnDutyEmployees(ctx context.Context, merchantID int64, appointmentTime time.Time) ([]OnDutyEmployee, error) {
	dateStr := appointmentTime.Format("2006-01-02")
	timeStr := appointmentTime.Format("15:04")

	rows, err := s.db.QueryContext(ctx,
		`SELECT e.id, e.name, e.position
		 FROM employees e
		 INNER JOIN employee_schedules es ON e.id = es.employee_id
		 WHERE e.merchant_id = $1
		   AND e.status = 'active'
		   AND e.deleted_at IS NULL
		   AND es.merchant_id = $1
		   AND es.schedule_date = $2
		   AND es.shift_type != 'rest'
		   AND es.deleted_at IS NULL
		   AND (
		     (es.shift_type = 'morning' AND $3::time >= '09:00'::time AND $3::time < '17:00'::time)
		     OR
		     (es.shift_type = 'evening' AND $3::time >= '13:00'::time AND $3::time < '21:00'::time)
		   )
		 ORDER BY e.name ASC`,
		merchantID, dateStr, timeStr,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get on-duty employees", err)
	}
	defer rows.Close()

	var employees []OnDutyEmployee
	for rows.Next() {
		var e OnDutyEmployee
		if err := rows.Scan(&e.ID, &e.Name, &e.Position); err != nil {
			return nil, apperrors.NewInternalError("failed to scan on-duty employee", err)
		}
		employees = append(employees, e)
	}
	if employees == nil {
		employees = []OnDutyEmployee{}
	}
	return employees, rows.Err()
}

// IsOnDuty checks if an employee is on duty at a specific time.
func (s *Service) IsOnDuty(ctx context.Context, merchantID, employeeID int64, appointmentTime time.Time) (bool, error) {
	dateStr := appointmentTime.Format("2006-01-02")
	timeStr := appointmentTime.Format("15:04")

	var shiftType string
	err := s.db.QueryRowContext(ctx,
		`SELECT shift_type FROM employee_schedules
		 WHERE merchant_id = $1 AND employee_id = $2 AND schedule_date = $3
		   AND shift_type != 'rest' AND deleted_at IS NULL`,
		merchantID, employeeID, dateStr,
	).Scan(&shiftType)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, apperrors.NewInternalError("failed to check schedule", err)
	}

	hours, ok := ShiftHours[shiftType]
	if !ok {
		return false, nil
	}

	// Parse shift start/end and appointment time.
	t, err := time.Parse("15:04", timeStr)
	if err != nil {
		return false, nil
	}
	start, err := time.Parse("15:04", hours.Start)
	if err != nil {
		return false, nil
	}
	end, err := time.Parse("15:04", hours.End)
	if err != nil {
		return false, nil
	}

	return (t.After(start) || t.Equal(start)) && t.Before(end), nil
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
