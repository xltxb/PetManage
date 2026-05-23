package appointment

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/xltxb/PetManage/internal/notification"
	"github.com/xltxb/PetManage/internal/schedule"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Appointment represents an appointment record.
type Appointment struct {
	ID              int64     `json:"id"`
	MerchantID      int64     `json:"merchant_id"`
	MemberID        int64     `json:"member_id"`
	PetID           int64     `json:"pet_id"`
	ServiceItemID   int64     `json:"service_item_id"`
	EmployeeID      int64     `json:"employee_id"`
	AppointmentTime time.Time `json:"appointment_time"`
	Status          string    `json:"status"`
	Remark          string    `json:"remark"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// AppointmentDetail joins related entity names for the list view.
type AppointmentDetail struct {
	Appointment
	MemberName      string `json:"member_name"`
	MemberPhone     string `json:"member_phone"`
	PetName         string `json:"pet_name"`
	ServiceItemName string `json:"service_item_name"`
	EmployeeName    string `json:"employee_name"`
}

// CreateAppointmentRequest is the request body for creating an appointment.
type CreateAppointmentRequest struct {
	MemberID        int64  `json:"member_id"`
	PetID           int64  `json:"pet_id"`
	ServiceItemID   int64  `json:"service_item_id"`
	EmployeeID      int64  `json:"employee_id"`
	AppointmentTime string `json:"appointment_time"` // RFC3339 format
	Remark          string `json:"remark"`
}

// ListParams holds optional filters and pagination for listing appointments.
type ListParams struct {
	Status   string
	Page     int
	PageSize int
}

// ListResult wraps the appointments list with pagination info.
type ListResult struct {
	Appointments []AppointmentDetail `json:"appointments"`
	Total        int                 `json:"total"`
	Page         int                 `json:"page"`
	PageSize     int                 `json:"page_size"`
}

// Service provides appointment management operations.
type Service struct {
	db          *sql.DB
	notifSvc    *notification.Service
	scheduleSvc *schedule.Service
}

// NewService creates a new appointment Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// SetNotificationService sets the notification service for sending appointment notifications.
func (s *Service) SetNotificationService(notifSvc *notification.Service) {
	s.notifSvc = notifSvc
}

// SetScheduleService sets the schedule service for shift validation during appointment creation.
func (s *Service) SetScheduleService(scheduleSvc *schedule.Service) {
	s.scheduleSvc = scheduleSvc
}

const appointmentColumns = `a.id, a.merchant_id, a.member_id, a.pet_id, a.service_item_id, a.employee_id, a.appointment_time, a.status, a.remark, a.created_at, a.updated_at`

const appointmentInsertColumns = `id, merchant_id, member_id, pet_id, service_item_id, employee_id, appointment_time, status, remark, created_at, updated_at`

func scanAppointmentRow(row *sql.Row) (*Appointment, error) {
	apt := &Appointment{}
	err := row.Scan(
		&apt.ID, &apt.MerchantID, &apt.MemberID, &apt.PetID,
		&apt.ServiceItemID, &apt.EmployeeID, &apt.AppointmentTime,
		&apt.Status, &apt.Remark, &apt.CreatedAt, &apt.UpdatedAt,
	)
	return apt, err
}

func scanAppointmentRows(rows *sql.Rows) (*AppointmentDetail, error) {
	d := &AppointmentDetail{}
	err := rows.Scan(
		&d.ID, &d.MerchantID, &d.MemberID, &d.PetID,
		&d.ServiceItemID, &d.EmployeeID, &d.AppointmentTime,
		&d.Status, &d.Remark, &d.CreatedAt, &d.UpdatedAt,
		&d.MemberName, &d.MemberPhone, &d.PetName,
		&d.ServiceItemName, &d.EmployeeName,
	)
	return d, err
}

// Create creates a new appointment with conflict detection.
func (s *Service) Create(ctx context.Context, merchantID int64, req CreateAppointmentRequest) (*Appointment, error) {
	// Validate required fields.
	var missing []string
	if req.MemberID <= 0 {
		missing = append(missing, "member_id")
	}
	if req.PetID <= 0 {
		missing = append(missing, "pet_id")
	}
	if req.ServiceItemID <= 0 {
		missing = append(missing, "service_item_id")
	}
	if req.EmployeeID <= 0 {
		missing = append(missing, "employee_id")
	}
	if strings.TrimSpace(req.AppointmentTime) == "" {
		missing = append(missing, "appointment_time")
	}
	if len(missing) > 0 {
		return nil, apperrors.NewValidationError("missing required fields: " + strings.Join(missing, ", "))
	}

	apptTime, err := time.Parse(time.RFC3339, req.AppointmentTime)
	if err != nil {
		return nil, apperrors.NewValidationError("invalid appointment_time format, use RFC3339: " + err.Error())
	}
	if apptTime.Before(time.Now()) {
		return nil, apperrors.NewValidationError("appointment_time must be in the future")
	}

	// Verify member exists and belongs to this merchant.
	var memberExists bool
	err = s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM members WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL)`,
		req.MemberID, merchantID,
	).Scan(&memberExists)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to verify member", err)
	}
	if !memberExists {
		return nil, apperrors.NewNotFoundError("member not found")
	}

	// Verify pet belongs to this member and merchant.
	var petExists bool
	err = s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM pets WHERE id = $1 AND member_id = $2 AND merchant_id = $3 AND deleted_at IS NULL)`,
		req.PetID, req.MemberID, merchantID,
	).Scan(&petExists)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to verify pet", err)
	}
	if !petExists {
		return nil, apperrors.NewNotFoundError("pet not found or does not belong to this member")
	}

	// Verify service item exists.
	var svcExists bool
	err = s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM service_items WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL AND status = 'active')`,
		req.ServiceItemID, merchantID,
	).Scan(&svcExists)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to verify service item", err)
	}
	if !svcExists {
		return nil, apperrors.NewNotFoundError("service item not found or inactive")
	}

	// Verify employee exists and is active.
	var empExists bool
	err = s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM employees WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL AND status = 'active')`,
		req.EmployeeID, merchantID,
	).Scan(&empExists)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to verify employee", err)
	}
	if !empExists {
		return nil, apperrors.NewNotFoundError("employee not found or inactive")
	}

	// Verify employee is on duty at the requested time.
	if s.scheduleSvc != nil {
		onDuty, err := s.scheduleSvc.IsOnDuty(ctx, merchantID, req.EmployeeID, apptTime)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to check employee schedule", err)
		}
		if !onDuty {
			return nil, apperrors.NewValidationError("the technician is not on duty at the selected time")
		}
	}

	// Detect conflict: same technician at the same time slot (within 1 hour window).
	var conflictCount int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM appointments
		 WHERE employee_id = $1 AND merchant_id = $2 AND deleted_at IS NULL
		 AND status NOT IN ('cancelled')
		 AND appointment_time = $3`,
		req.EmployeeID, merchantID, apptTime,
	).Scan(&conflictCount)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to check appointment conflict", err)
	}
	if conflictCount > 0 {
		return nil, apperrors.NewConflictError("the technician already has an appointment at this time")
	}

	// Create the appointment with status 'pending' (待确认).
	apt, err := scanAppointmentRow(s.db.QueryRowContext(ctx,
		`INSERT INTO appointments (merchant_id, member_id, pet_id, service_item_id, employee_id, appointment_time, status, remark)
		 VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7)
		 RETURNING `+appointmentInsertColumns,
		merchantID, req.MemberID, req.PetID, req.ServiceItemID, req.EmployeeID, apptTime, req.Remark,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create appointment", err)
	}

	if s.notifSvc != nil {
		s.notifSvc.SendAppointmentNotification(ctx, merchantID, apt.ID,
			apt.MemberID, "member", "created",
			map[string]string{"appointment_time": apt.AppointmentTime.Format("2006-01-02 15:04")})
		s.notifSvc.SendAppointmentNotification(ctx, merchantID, apt.ID,
			apt.EmployeeID, "employee", "created",
			map[string]string{"appointment_time": apt.AppointmentTime.Format("2006-01-02 15:04")})
	}

	return apt, nil
}

// List returns a filtered and paginated list of appointments.
func (s *Service) List(ctx context.Context, merchantID int64, params ListParams) (*ListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}

	var conditions []string
	var args []interface{}
	args = append(args, merchantID)
	conditions = append(conditions, "a.merchant_id = $1")
	conditions = append(conditions, "a.deleted_at IS NULL")
	argIdx := 2

	if params.Status != "" {
		conditions = append(conditions, "a.status = $"+strconv.Itoa(argIdx))
		args = append(args, params.Status)
		argIdx++
	}

	whereClause := strings.Join(conditions, " AND ")

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM appointments a WHERE `+whereClause,
		args...,
	).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count appointments", err)
	}

	offset := (params.Page - 1) * params.PageSize
	queryArgs := append(args, params.PageSize, offset)

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+appointmentColumns+`,
		 COALESCE(m.name, ''), COALESCE(m.phone, ''),
		 COALESCE(p.name, ''),
		 COALESCE(si.name, ''),
		 COALESCE(e.name, '')
		 FROM appointments a
		 LEFT JOIN members m ON m.id = a.member_id
		 LEFT JOIN pets p ON p.id = a.pet_id
		 LEFT JOIN service_items si ON si.id = a.service_item_id
		 LEFT JOIN employees e ON e.id = a.employee_id
		 WHERE `+whereClause+
			` ORDER BY a.appointment_time ASC LIMIT $`+strconv.Itoa(argIdx)+` OFFSET $`+strconv.Itoa(argIdx+1),
		queryArgs...,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list appointments", err)
	}
	defer rows.Close()

	var appointments []AppointmentDetail
	for rows.Next() {
		d, err := scanAppointmentRows(rows)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to scan appointment", err)
		}
		appointments = append(appointments, *d)
	}
	if appointments == nil {
		appointments = []AppointmentDetail{}
	}

	return &ListResult{
		Appointments: appointments,
		Total:        total,
		Page:         params.Page,
		PageSize:     params.PageSize,
	}, rows.Err()
}

// GetByID returns a single appointment with joined entity names.
func (s *Service) GetByID(ctx context.Context, merchantID, appointmentID int64) (*AppointmentDetail, error) {
	d := &AppointmentDetail{}
	err := s.db.QueryRowContext(ctx,
		`SELECT `+appointmentColumns+`,
		 COALESCE(m.name, ''), COALESCE(m.phone, ''),
		 COALESCE(p.name, ''),
		 COALESCE(si.name, ''),
		 COALESCE(e.name, '')
		 FROM appointments a
		 LEFT JOIN members m ON m.id = a.member_id
		 LEFT JOIN pets p ON p.id = a.pet_id
		 LEFT JOIN service_items si ON si.id = a.service_item_id
		 LEFT JOIN employees e ON e.id = a.employee_id
		 WHERE a.id = $1 AND a.merchant_id = $2 AND a.deleted_at IS NULL`,
		appointmentID, merchantID,
	).Scan(
		&d.ID, &d.MerchantID, &d.MemberID, &d.PetID,
		&d.ServiceItemID, &d.EmployeeID, &d.AppointmentTime,
		&d.Status, &d.Remark, &d.CreatedAt, &d.UpdatedAt,
		&d.MemberName, &d.MemberPhone, &d.PetName,
		&d.ServiceItemName, &d.EmployeeName,
	)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("appointment not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get appointment", err)
	}
	return d, nil
}

// ChangeLog represents a single change history entry.
type ChangeLog struct {
	ID            int64           `json:"id"`
	AppointmentID int64           `json:"appointment_id"`
	Action        string          `json:"action"`
	OldValue      json.RawMessage `json:"old_value"`
	NewValue      json.RawMessage `json:"new_value"`
	OperatorID    int64           `json:"operator_id"`
	Reason        string          `json:"reason"`
	CreatedAt     time.Time       `json:"created_at"`
}

// Confirm confirms a pending appointment, changing status to 'confirmed'.
func (s *Service) Confirm(ctx context.Context, merchantID, appointmentID int64) (*AppointmentDetail, error) {
	apt, err := s.GetByID(ctx, merchantID, appointmentID)
	if err != nil {
		return nil, err
	}
	if apt.Status != "pending" {
		return nil, apperrors.NewValidationError("only pending appointments can be confirmed, current status: " + apt.Status)
	}

	oldStatus := apt.Status
	newStatus := "confirmed"

	result, err := s.db.ExecContext(ctx,
		`UPDATE appointments SET status = $1, updated_at = NOW()
		 WHERE id = $2 AND merchant_id = $3 AND deleted_at IS NULL`,
		newStatus, appointmentID, merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to confirm appointment", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, apperrors.NewNotFoundError("appointment not found")
	}

	s.logChange(ctx, merchantID, appointmentID, "confirmed",
		map[string]string{"status": oldStatus},
		map[string]string{"status": newStatus},
		"")

	if s.notifSvc != nil {
		s.notifSvc.SendAppointmentNotification(ctx, merchantID, appointmentID,
			apt.MemberID, "member", "confirmed",
			map[string]string{"appointment_time": apt.AppointmentTime.Format("2006-01-02 15:04")})
	}

	return s.GetByID(ctx, merchantID, appointmentID)
}

// RescheduleRequest is the request body for rescheduling an appointment.
type RescheduleRequest struct {
	NewTime string `json:"new_time"`
	Reason  string `json:"reason"`
}

// Reschedule changes the appointment time with conflict detection.
func (s *Service) Reschedule(ctx context.Context, merchantID, appointmentID int64, req RescheduleRequest) (*AppointmentDetail, error) {
	apt, err := s.GetByID(ctx, merchantID, appointmentID)
	if err != nil {
		return nil, err
	}
	if apt.Status == "cancelled" || apt.Status == "picked_up" {
		return nil, apperrors.NewValidationError("terminal appointments cannot be rescheduled")
	}

	if strings.TrimSpace(req.NewTime) == "" {
		return nil, apperrors.NewValidationError("new_time is required")
	}

	newTime, err := time.Parse(time.RFC3339, req.NewTime)
	if err != nil {
		return nil, apperrors.NewValidationError("invalid new_time format, use RFC3339: " + err.Error())
	}
	if newTime.Before(time.Now()) {
		return nil, apperrors.NewValidationError("new_time must be in the future")
	}

	var conflictCount int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM appointments
		 WHERE employee_id = $1 AND merchant_id = $2 AND deleted_at IS NULL
		 AND status NOT IN ('cancelled')
		 AND appointment_time = $3 AND id != $4`,
		apt.EmployeeID, merchantID, newTime, appointmentID,
	).Scan(&conflictCount)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to check appointment conflict", err)
	}
	if conflictCount > 0 {
		return nil, apperrors.NewConflictError("the technician already has an appointment at this time")
	}

	oldTime := apt.AppointmentTime.Format(time.RFC3339)

	result, err := s.db.ExecContext(ctx,
		`UPDATE appointments SET appointment_time = $1, updated_at = NOW()
		 WHERE id = $2 AND merchant_id = $3 AND deleted_at IS NULL`,
		newTime, appointmentID, merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to reschedule appointment", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, apperrors.NewNotFoundError("appointment not found")
	}

	s.logChange(ctx, merchantID, appointmentID, "rescheduled",
		map[string]string{"appointment_time": oldTime},
		map[string]string{"appointment_time": req.NewTime, "reason": req.Reason},
		req.Reason)

	if s.notifSvc != nil {
		timeStr := newTime.Format("2006-01-02 15:04")
		s.notifSvc.SendAppointmentNotification(ctx, merchantID, appointmentID,
			apt.MemberID, "member", "rescheduled",
			map[string]string{"appointment_time": timeStr})
		s.notifSvc.SendAppointmentNotification(ctx, merchantID, appointmentID,
			apt.EmployeeID, "employee", "rescheduled",
			map[string]string{"appointment_time": timeStr})
	}

	return s.GetByID(ctx, merchantID, appointmentID)
}

// CancelRequest is the request body for cancelling an appointment.
type CancelRequest struct {
	Reason string `json:"reason"`
}

// Cancel cancels an appointment, releasing the technician's time slot.
func (s *Service) Cancel(ctx context.Context, merchantID, appointmentID int64, req CancelRequest) (*AppointmentDetail, error) {
	apt, err := s.GetByID(ctx, merchantID, appointmentID)
	if err != nil {
		return nil, err
	}
	if apt.Status == "cancelled" {
		return nil, apperrors.NewValidationError("appointment is already cancelled")
	}
	if apt.Status == "picked_up" {
		return nil, apperrors.NewValidationError("picked up appointments cannot be cancelled")
	}

	oldStatus := apt.Status

	result, err := s.db.ExecContext(ctx,
		`UPDATE appointments SET status = 'cancelled', remark = CASE WHEN $1 = '' THEN remark ELSE $1 END, updated_at = NOW()
		 WHERE id = $2 AND merchant_id = $3 AND deleted_at IS NULL`,
		req.Reason, appointmentID, merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to cancel appointment", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, apperrors.NewNotFoundError("appointment not found")
	}

	s.logChange(ctx, merchantID, appointmentID, "cancelled",
		map[string]string{"status": oldStatus},
		map[string]string{"status": "cancelled", "reason": req.Reason},
		req.Reason)

	if s.notifSvc != nil {
		s.notifSvc.SendAppointmentNotification(ctx, merchantID, appointmentID,
			apt.MemberID, "member", "cancelled",
			map[string]string{"reason": req.Reason})
		s.notifSvc.SendAppointmentNotification(ctx, merchantID, appointmentID,
			apt.EmployeeID, "employee", "cancelled",
			map[string]string{"reason": req.Reason})
	}

	return s.GetByID(ctx, merchantID, appointmentID)
}

// Arrive marks a confirmed appointment as arrived (pet checked in at store).
func (s *Service) Arrive(ctx context.Context, merchantID, appointmentID int64) (*AppointmentDetail, error) {
	apt, err := s.GetByID(ctx, merchantID, appointmentID)
	if err != nil {
		return nil, err
	}
	if apt.Status != "confirmed" {
		return nil, apperrors.NewValidationError("only confirmed appointments can be marked as arrived, current status: " + apt.Status)
	}

	oldStatus := apt.Status
	newStatus := "arrived"

	result, err := s.db.ExecContext(ctx,
		`UPDATE appointments SET status = $1, updated_at = NOW()
		 WHERE id = $2 AND merchant_id = $3 AND deleted_at IS NULL`,
		newStatus, appointmentID, merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update appointment", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, apperrors.NewNotFoundError("appointment not found")
	}

	s.logChange(ctx, merchantID, appointmentID, "arrived",
		map[string]string{"status": oldStatus},
		map[string]string{"status": newStatus},
		"")

	if s.notifSvc != nil {
		s.notifSvc.SendAppointmentNotification(ctx, merchantID, appointmentID,
			apt.EmployeeID, "employee", "arrived",
			map[string]string{"appointment_time": apt.AppointmentTime.Format("2006-01-02 15:04")})
	}

	return s.GetByID(ctx, merchantID, appointmentID)
}

// Start begins the service for an arrived appointment.
func (s *Service) Start(ctx context.Context, merchantID, appointmentID int64) (*AppointmentDetail, error) {
	apt, err := s.GetByID(ctx, merchantID, appointmentID)
	if err != nil {
		return nil, err
	}
	if apt.Status != "arrived" {
		return nil, apperrors.NewValidationError("only arrived appointments can be started, current status: " + apt.Status)
	}

	oldStatus := apt.Status
	newStatus := "in_progress"

	result, err := s.db.ExecContext(ctx,
		`UPDATE appointments SET status = $1, updated_at = NOW()
		 WHERE id = $2 AND merchant_id = $3 AND deleted_at IS NULL`,
		newStatus, appointmentID, merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update appointment", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, apperrors.NewNotFoundError("appointment not found")
	}

	s.logChange(ctx, merchantID, appointmentID, "started",
		map[string]string{"status": oldStatus},
		map[string]string{"status": newStatus},
		"")

	return s.GetByID(ctx, merchantID, appointmentID)
}

// Complete marks a service as finished and notifies the member to pick up their pet.
func (s *Service) Complete(ctx context.Context, merchantID, appointmentID int64) (*AppointmentDetail, error) {
	apt, err := s.GetByID(ctx, merchantID, appointmentID)
	if err != nil {
		return nil, err
	}
	if apt.Status != "in_progress" {
		return nil, apperrors.NewValidationError("only in-progress appointments can be completed, current status: " + apt.Status)
	}

	oldStatus := apt.Status
	newStatus := "completed"

	result, err := s.db.ExecContext(ctx,
		`UPDATE appointments SET status = $1, updated_at = NOW()
		 WHERE id = $2 AND merchant_id = $3 AND deleted_at IS NULL`,
		newStatus, appointmentID, merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update appointment", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, apperrors.NewNotFoundError("appointment not found")
	}

	s.logChange(ctx, merchantID, appointmentID, "completed",
		map[string]string{"status": oldStatus},
		map[string]string{"status": newStatus},
		"")

	if s.notifSvc != nil {
		s.notifSvc.SendAppointmentNotification(ctx, merchantID, appointmentID,
			apt.MemberID, "member", "completed",
			map[string]string{"appointment_time": apt.AppointmentTime.Format("2006-01-02 15:04")})
	}

	return s.GetByID(ctx, merchantID, appointmentID)
}

// Pickup marks a completed appointment as picked up by the customer.
func (s *Service) Pickup(ctx context.Context, merchantID, appointmentID int64) (*AppointmentDetail, error) {
	apt, err := s.GetByID(ctx, merchantID, appointmentID)
	if err != nil {
		return nil, err
	}
	if apt.Status != "completed" {
		return nil, apperrors.NewValidationError("only completed appointments can be picked up, current status: " + apt.Status)
	}

	oldStatus := apt.Status
	newStatus := "picked_up"

	result, err := s.db.ExecContext(ctx,
		`UPDATE appointments SET status = $1, updated_at = NOW()
		 WHERE id = $2 AND merchant_id = $3 AND deleted_at IS NULL`,
		newStatus, appointmentID, merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update appointment", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, apperrors.NewNotFoundError("appointment not found")
	}

	s.logChange(ctx, merchantID, appointmentID, "picked_up",
		map[string]string{"status": oldStatus},
		map[string]string{"status": newStatus},
		"")

	return s.GetByID(ctx, merchantID, appointmentID)
}

// GetChangeLogs returns the change history for an appointment.
func (s *Service) GetChangeLogs(ctx context.Context, merchantID, appointmentID int64) ([]ChangeLog, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, appointment_id, action, COALESCE(old_value::text, 'null')::jsonb, COALESCE(new_value::text, 'null')::jsonb, COALESCE(operator_id, 0), reason, created_at
		 FROM appointment_change_logs
		 WHERE appointment_id = $1 AND merchant_id = $2
		 ORDER BY created_at ASC`,
		appointmentID, merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get change logs", err)
	}
	defer rows.Close()

	var logs []ChangeLog
	for rows.Next() {
		l := ChangeLog{}
		if err := rows.Scan(&l.ID, &l.AppointmentID, &l.Action, &l.OldValue, &l.NewValue, &l.OperatorID, &l.Reason, &l.CreatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan change log", err)
		}
		logs = append(logs, l)
	}
	if logs == nil {
		logs = []ChangeLog{}
	}
	return logs, rows.Err()
}

// logChange records a change in the appointment_change_logs table.
func (s *Service) logChange(ctx context.Context, merchantID, appointmentID int64, action string, oldVal, newVal map[string]string, reason string) {
	oldJSON, _ := json.Marshal(oldVal)
	newJSON, _ := json.Marshal(newVal)
	s.db.ExecContext(ctx,
		`INSERT INTO appointment_change_logs (merchant_id, appointment_id, action, old_value, new_value, reason)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		merchantID, appointmentID, action, oldJSON, newJSON, reason,
	)
}
