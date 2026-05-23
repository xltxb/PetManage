package appointment

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

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
	db *sql.DB
}

// NewService creates a new appointment Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
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
