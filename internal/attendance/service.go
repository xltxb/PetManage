package attendance

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// AttendanceRecord represents a daily check-in/check-out record.
type AttendanceRecord struct {
	ID               int64      `json:"id"`
	MerchantID       int64      `json:"merchant_id"`
	EmployeeID       int64      `json:"employee_id"`
	RecordDate       string     `json:"record_date"`
	CheckInTime      *string    `json:"check_in_time"`
	CheckOutTime     *string    `json:"check_out_time"`
	Status           string     `json:"status"`
	LateMinutes      int        `json:"late_minutes"`
	EarlyLeaveMinutes int       `json:"early_leave_minutes"`
	Notes            string     `json:"notes"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// LeaveRequest represents a leave application.
type LeaveRequest struct {
	ID           int64      `json:"id"`
	MerchantID   int64      `json:"merchant_id"`
	EmployeeID   int64      `json:"employee_id"`
	EmployeeName string     `json:"employee_name"`
	EmployeeNo   string     `json:"employee_no"`
	LeaveType    string     `json:"leave_type"`
	StartDate    string     `json:"start_date"`
	EndDate      string     `json:"end_date"`
	Reason       string     `json:"reason"`
	Status       string     `json:"status"`
	ReviewedBy   *int64     `json:"reviewed_by"`
	ReviewRemark string     `json:"review_remark"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// OvertimeRecord represents an overtime registration.
type OvertimeRecord struct {
	ID            int64      `json:"id"`
	MerchantID    int64      `json:"merchant_id"`
	EmployeeID    int64      `json:"employee_id"`
	EmployeeName  string     `json:"employee_name"`
	EmployeeNo    string     `json:"employee_no"`
	OvertimeDate  string     `json:"overtime_date"`
	StartTime     string     `json:"start_time"`
	EndTime       string     `json:"end_time"`
	DurationHours float64    `json:"duration_hours"`
	Reason        string     `json:"reason"`
	Status        string     `json:"status"`
	ReviewedBy    *int64     `json:"reviewed_by"`
	ReviewRemark  string     `json:"review_remark"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// CreateLeaveRequest is the request body for applying leave.
type CreateLeaveRequest struct {
	LeaveType string `json:"leave_type"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Reason    string `json:"reason"`
}

// ReviewLeaveRequest is the request body for reviewing leave.
type ReviewLeaveRequest struct {
	Status   string `json:"status"`
	Remark   string `json:"remark"`
}

// CreateOvertimeRequest is the request body for registering overtime.
type CreateOvertimeRequest struct {
	OvertimeDate string `json:"overtime_date"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	Reason       string `json:"reason"`
}

// ReviewOvertimeRequest is the request body for reviewing overtime.
type ReviewOvertimeRequest struct {
	Status string `json:"status"`
	Remark string `json:"remark"`
}

// LeaveListParams holds filters for listing leave requests.
type LeaveListParams struct {
	EmployeeID int64
	Status     string
	Page       int
	PageSize   int
}

// LeaveListResult wraps leave requests with pagination.
type LeaveListResult struct {
	Leaves   []LeaveRequest `json:"leaves"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

// OvertimeListParams holds filters for listing overtime records.
type OvertimeListParams struct {
	EmployeeID int64
	Status     string
	Page       int
	PageSize   int
}

// OvertimeListResult wraps overtime records with pagination.
type OvertimeListResult struct {
	OvertimeRecords []OvertimeRecord `json:"overtime_records"`
	Total           int              `json:"total"`
	Page            int              `json:"page"`
	PageSize        int              `json:"page_size"`
}

// AttendanceStats holds attendance statistics.
type AttendanceStats struct {
	Period         string            `json:"period"`
	TotalDays      int               `json:"total_days"`
	PresentDays    int               `json:"present_days"`
	AbsentDays     int               `json:"absent_days"`
	LateCount      int               `json:"late_count"`
	EarlyLeaveCount int              `json:"early_leave_count"`
	LeaveDays      int               `json:"leave_days"`
	LeaveByType    map[string]int    `json:"leave_by_type"`
	OvertimeHours  float64           `json:"overtime_hours"`
	Records        []AttendanceRecord `json:"records,omitempty"`
}

// StatsParams holds filters for attendance stats queries.
type StatsParams struct {
	EmployeeID int64
	StartDate  string
	EndDate    string
	Type       string // daily, monthly
}

// Service provides employee attendance operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new attendance Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// GetEmployeeIDByUser looks up the employee_id for a platform user.
func (s *Service) GetEmployeeIDByUser(ctx context.Context, merchantID, userID int64, out *int64) (int64, error) {
	var employeeID sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT employee_id FROM platform_users
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL AND status = 'active'`,
		userID, merchantID,
	).Scan(&employeeID)
	if err != nil || !employeeID.Valid {
		return 0, err
	}
	id := employeeID.Int64
	if out != nil {
		*out = id
	}
	return id, nil
}

// CheckIn records a check-in for the given employee.
func (s *Service) CheckIn(ctx context.Context, merchantID, employeeID int64) (*AttendanceRecord, error) {
	today := time.Now().Format("2006-01-02")
	now := time.Now()

	// Check if already checked in today.
	var existingID int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM attendance_records
		 WHERE merchant_id = $1 AND employee_id = $2 AND record_date = $3 AND deleted_at IS NULL`,
		merchantID, employeeID, today,
	).Scan(&existingID)
	if err == nil {
		return nil, apperrors.NewConflictError("already checked in today")
	}
	if err != sql.ErrNoRows {
		return nil, apperrors.NewInternalError("failed to check attendance", err)
	}

	// Determine late status (work start at 09:00).
	status := "normal"
	lateMinutes := 0
	workStart := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())
	if now.After(workStart) {
		status = "late"
		lateMinutes = int(math.Ceil(now.Sub(workStart).Minutes()))
	}

	checkInStr := now.Format(time.RFC3339)

	var record AttendanceRecord
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO attendance_records (merchant_id, employee_id, record_date, check_in_time, status, late_minutes)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, merchant_id, employee_id, record_date, check_in_time,
		           status, late_minutes, early_leave_minutes, COALESCE(notes, ''),
		           created_at, updated_at`,
		merchantID, employeeID, today, checkInStr, status, lateMinutes,
	).Scan(&record.ID, &record.MerchantID, &record.EmployeeID, &record.RecordDate,
		&record.CheckInTime, &record.Status, &record.LateMinutes,
		&record.EarlyLeaveMinutes, &record.Notes, &record.CreatedAt, &record.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create attendance record", err)
	}

	return &record, nil
}

// CheckOut records a check-out for the given employee.
func (s *Service) CheckOut(ctx context.Context, merchantID, employeeID int64) (*AttendanceRecord, error) {
	today := time.Now().Format("2006-01-02")
	now := time.Now()

	// Check if there's a check-in record without check-out.
	var record AttendanceRecord
	err := s.db.QueryRowContext(ctx,
		`SELECT id, merchant_id, employee_id, record_date, check_in_time,
		        status, late_minutes, early_leave_minutes, COALESCE(notes, ''),
		        created_at, updated_at
		 FROM attendance_records
		 WHERE merchant_id = $1 AND employee_id = $2 AND record_date = $3
		 AND deleted_at IS NULL AND check_out_time IS NULL`,
		merchantID, employeeID, today,
	).Scan(&record.ID, &record.MerchantID, &record.EmployeeID, &record.RecordDate,
		&record.CheckInTime, &record.Status, &record.LateMinutes,
		&record.EarlyLeaveMinutes, &record.Notes, &record.CreatedAt, &record.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewValidationError("no check-in record found for today")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to find attendance record", err)
	}

	// Determine early leave (work end at 18:00).
	workEnd := time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, now.Location())
	earlyMinutes := 0
	newStatus := record.Status
	if now.Before(workEnd) {
		earlyMinutes = int(math.Ceil(workEnd.Sub(now).Minutes()))
		if newStatus == "normal" {
			newStatus = "early_leave"
		}
	}

	checkOutStr := now.Format(time.RFC3339)

	var updated AttendanceRecord
	err = s.db.QueryRowContext(ctx,
		`UPDATE attendance_records
		 SET check_out_time = $1, status = $2, early_leave_minutes = $3, updated_at = NOW()
		 WHERE id = $4 AND deleted_at IS NULL
		 RETURNING id, merchant_id, employee_id, record_date, check_in_time, check_out_time,
		           status, late_minutes, early_leave_minutes, COALESCE(notes, ''),
		           created_at, updated_at`,
		checkOutStr, newStatus, earlyMinutes, record.ID,
	).Scan(&updated.ID, &updated.MerchantID, &updated.EmployeeID, &updated.RecordDate,
		&updated.CheckInTime, &updated.CheckOutTime, &updated.Status,
		&updated.LateMinutes, &updated.EarlyLeaveMinutes, &updated.Notes,
		&updated.CreatedAt, &updated.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update attendance record", err)
	}

	return &updated, nil
}

// GetTodayStatus returns today's attendance status for the employee.
func (s *Service) GetTodayStatus(ctx context.Context, merchantID, employeeID int64) (*AttendanceRecord, error) {
	today := time.Now().Format("2006-01-02")

	var record AttendanceRecord
	err := s.db.QueryRowContext(ctx,
		`SELECT id, merchant_id, employee_id, record_date, check_in_time, check_out_time,
		        status, late_minutes, early_leave_minutes, COALESCE(notes, ''),
		        created_at, updated_at
		 FROM attendance_records
		 WHERE merchant_id = $1 AND employee_id = $2 AND record_date = $3 AND deleted_at IS NULL`,
		merchantID, employeeID, today,
	).Scan(&record.ID, &record.MerchantID, &record.EmployeeID, &record.RecordDate,
		&record.CheckInTime, &record.CheckOutTime, &record.Status,
		&record.LateMinutes, &record.EarlyLeaveMinutes, &record.Notes,
		&record.CreatedAt, &record.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil // no record yet today
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get today's attendance", err)
	}

	return &record, nil
}

// ApplyLeave creates a leave request.
func (s *Service) ApplyLeave(ctx context.Context, merchantID, employeeID int64, req CreateLeaveRequest) (*LeaveRequest, error) {
	if req.LeaveType == "" {
		req.LeaveType = "personal"
	}
	if req.StartDate == "" || req.EndDate == "" {
		return nil, apperrors.NewValidationError("start_date and end_date are required")
	}

	if req.LeaveType != "annual" && req.LeaveType != "sick" && req.LeaveType != "personal" && req.LeaveType != "other" {
		return nil, apperrors.NewValidationError("leave_type must be annual, sick, personal, or other")
	}

	var lr LeaveRequest
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO leave_requests (merchant_id, employee_id, leave_type, start_date, end_date, reason)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, merchant_id, employee_id, leave_type, start_date, end_date,
		           COALESCE(reason, ''), status, created_at, updated_at`,
		merchantID, employeeID, req.LeaveType, req.StartDate, req.EndDate, req.Reason,
	).Scan(&lr.ID, &lr.MerchantID, &lr.EmployeeID, &lr.LeaveType,
		&lr.StartDate, &lr.EndDate, &lr.Reason, &lr.Status,
		&lr.CreatedAt, &lr.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create leave request", err)
	}

	// Fetch employee name.
	s.fillEmployeeName(ctx, &lr)

	return &lr, nil
}

// ListLeaves returns leave requests with optional filters.
func (s *Service) ListLeaves(ctx context.Context, merchantID int64, params LeaveListParams) (*LeaveListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}

	where := "l.merchant_id = $1 AND l.deleted_at IS NULL"
	args := []interface{}{merchantID}
	argIdx := 2

	if params.EmployeeID > 0 {
		where += fmt.Sprintf(" AND l.employee_id = $%d", argIdx)
		args = append(args, params.EmployeeID)
		argIdx++
	}
	if params.Status != "" {
		where += fmt.Sprintf(" AND l.status = $%d", argIdx)
		args = append(args, params.Status)
		argIdx++
	}

	var total int
	countArgs := append([]interface{}{}, args...)
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM leave_requests l WHERE `+where,
		countArgs...,
	).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count leave requests", err)
	}

	offset := (params.Page - 1) * params.PageSize
	queryArgs := append(args, params.PageSize, offset)
	query := fmt.Sprintf(
		`SELECT l.id, l.merchant_id, l.employee_id,
		        COALESCE(e.name, ''), COALESCE(e.employee_no, ''),
		        l.leave_type, l.start_date, l.end_date,
		        COALESCE(l.reason, ''), l.status, l.reviewed_by,
		        COALESCE(l.review_remark, ''), l.created_at, l.updated_at
		 FROM leave_requests l
		 LEFT JOIN employees e ON l.employee_id = e.id AND e.deleted_at IS NULL
		 WHERE %s
		 ORDER BY l.created_at DESC
		 LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)

	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list leave requests", err)
	}
	defer rows.Close()

	leaves := make([]LeaveRequest, 0)
	for rows.Next() {
		var lr LeaveRequest
		if err := rows.Scan(&lr.ID, &lr.MerchantID, &lr.EmployeeID,
			&lr.EmployeeName, &lr.EmployeeNo,
			&lr.LeaveType, &lr.StartDate, &lr.EndDate,
			&lr.Reason, &lr.Status, &lr.ReviewedBy,
			&lr.ReviewRemark, &lr.CreatedAt, &lr.UpdatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan leave request", err)
		}
		leaves = append(leaves, lr)
	}

	return &LeaveListResult{
		Leaves:   leaves,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, nil
}

// ReviewLeave approves or rejects a leave request.
func (s *Service) ReviewLeave(ctx context.Context, merchantID, leaveID, reviewerEmployeeID int64, req ReviewLeaveRequest) (*LeaveRequest, error) {
	if req.Status != "approved" && req.Status != "rejected" {
		return nil, apperrors.NewValidationError("status must be approved or rejected")
	}

	var reviewerID interface{}
	if reviewerEmployeeID > 0 {
		reviewerID = reviewerEmployeeID
	}

	var lr LeaveRequest
	var reviewedBy sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`UPDATE leave_requests
		 SET status = $1, reviewed_by = $2, review_remark = $3, updated_at = NOW()
		 WHERE id = $4 AND merchant_id = $5 AND deleted_at IS NULL AND status = 'pending'
		 RETURNING id, merchant_id, employee_id, leave_type, start_date, end_date,
		           COALESCE(reason, ''), status, reviewed_by,
		           COALESCE(review_remark, ''), created_at, updated_at`,
		req.Status, reviewerID, req.Remark, leaveID, merchantID,
	).Scan(&lr.ID, &lr.MerchantID, &lr.EmployeeID, &lr.LeaveType,
		&lr.StartDate, &lr.EndDate, &lr.Reason, &lr.Status,
		&reviewedBy, &lr.ReviewRemark, &lr.CreatedAt, &lr.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("leave request not found or already reviewed")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to review leave request", err)
	}

	if reviewedBy.Valid {
		lr.ReviewedBy = &reviewedBy.Int64
	}

	s.fillEmployeeName(ctx, &lr)

	return &lr, nil
}

// ApplyOvertime registers an overtime record.
func (s *Service) ApplyOvertime(ctx context.Context, merchantID, employeeID int64, req CreateOvertimeRequest) (*OvertimeRecord, error) {
	if req.OvertimeDate == "" || req.StartTime == "" || req.EndTime == "" {
		return nil, apperrors.NewValidationError("overtime_date, start_time, and end_time are required")
	}

	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		return nil, apperrors.NewValidationError("invalid start_time format, use RFC3339")
	}
	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		return nil, apperrors.NewValidationError("invalid end_time format, use RFC3339")
	}
	if !endTime.After(startTime) {
		return nil, apperrors.NewValidationError("end_time must be after start_time")
	}

	durationHours := math.Round(endTime.Sub(startTime).Hours()*10) / 10

	var or_ OvertimeRecord
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO overtime_records (merchant_id, employee_id, overtime_date, start_time, end_time, duration_hours, reason)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, merchant_id, employee_id, overtime_date, start_time, end_time,
		           duration_hours, COALESCE(reason, ''), status, created_at, updated_at`,
		merchantID, employeeID, req.OvertimeDate, req.StartTime, req.EndTime, durationHours, req.Reason,
	).Scan(&or_.ID, &or_.MerchantID, &or_.EmployeeID, &or_.OvertimeDate,
		&or_.StartTime, &or_.EndTime, &or_.DurationHours, &or_.Reason,
		&or_.Status, &or_.CreatedAt, &or_.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create overtime record", err)
	}

	s.fillOvertimeEmployeeName(ctx, &or_)

	return &or_, nil
}

// ListOvertime returns overtime records with optional filters.
func (s *Service) ListOvertime(ctx context.Context, merchantID int64, params OvertimeListParams) (*OvertimeListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}

	where := "o.merchant_id = $1 AND o.deleted_at IS NULL"
	args := []interface{}{merchantID}
	argIdx := 2

	if params.EmployeeID > 0 {
		where += fmt.Sprintf(" AND o.employee_id = $%d", argIdx)
		args = append(args, params.EmployeeID)
		argIdx++
	}
	if params.Status != "" {
		where += fmt.Sprintf(" AND o.status = $%d", argIdx)
		args = append(args, params.Status)
		argIdx++
	}

	var total int
	countArgs := append([]interface{}{}, args...)
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM overtime_records o WHERE `+where,
		countArgs...,
	).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count overtime records", err)
	}

	offset := (params.Page - 1) * params.PageSize
	queryArgs := append(args, params.PageSize, offset)
	query := fmt.Sprintf(
		`SELECT o.id, o.merchant_id, o.employee_id,
		        COALESCE(e.name, ''), COALESCE(e.employee_no, ''),
		        o.overtime_date, o.start_time, o.end_time,
		        o.duration_hours, COALESCE(o.reason, ''),
		        o.status, o.reviewed_by, COALESCE(o.review_remark, ''),
		        o.created_at, o.updated_at
		 FROM overtime_records o
		 LEFT JOIN employees e ON o.employee_id = e.id AND e.deleted_at IS NULL
		 WHERE %s
		 ORDER BY o.created_at DESC
		 LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)

	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list overtime records", err)
	}
	defer rows.Close()

	records := make([]OvertimeRecord, 0)
	for rows.Next() {
		var or_ OvertimeRecord
		if err := rows.Scan(&or_.ID, &or_.MerchantID, &or_.EmployeeID,
			&or_.EmployeeName, &or_.EmployeeNo,
			&or_.OvertimeDate, &or_.StartTime, &or_.EndTime,
			&or_.DurationHours, &or_.Reason,
			&or_.Status, &or_.ReviewedBy, &or_.ReviewRemark,
			&or_.CreatedAt, &or_.UpdatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan overtime record", err)
		}
		records = append(records, or_)
	}

	return &OvertimeListResult{
		OvertimeRecords: records,
		Total:           total,
		Page:            params.Page,
		PageSize:        params.PageSize,
	}, nil
}

// ReviewOvertime approves or rejects an overtime record.
func (s *Service) ReviewOvertime(ctx context.Context, merchantID, overtimeID, reviewerEmployeeID int64, req ReviewOvertimeRequest) (*OvertimeRecord, error) {
	if req.Status != "approved" && req.Status != "rejected" {
		return nil, apperrors.NewValidationError("status must be approved or rejected")
	}

	var reviewerID interface{}
	if reviewerEmployeeID > 0 {
		reviewerID = reviewerEmployeeID
	}

	var or_ OvertimeRecord
	var reviewedBy sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`UPDATE overtime_records
		 SET status = $1, reviewed_by = $2, review_remark = $3, updated_at = NOW()
		 WHERE id = $4 AND merchant_id = $5 AND deleted_at IS NULL AND status = 'pending'
		 RETURNING id, merchant_id, employee_id, overtime_date, start_time, end_time,
		           duration_hours, COALESCE(reason, ''), status, reviewed_by,
		           COALESCE(review_remark, ''), created_at, updated_at`,
		req.Status, reviewerID, req.Remark, overtimeID, merchantID,
	).Scan(&or_.ID, &or_.MerchantID, &or_.EmployeeID, &or_.OvertimeDate,
		&or_.StartTime, &or_.EndTime, &or_.DurationHours, &or_.Reason,
		&or_.Status, &reviewedBy, &or_.ReviewRemark,
		&or_.CreatedAt, &or_.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("overtime record not found or already reviewed")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to review overtime record", err)
	}

	if reviewedBy.Valid {
		or_.ReviewedBy = &reviewedBy.Int64
	}

	s.fillOvertimeEmployeeName(ctx, &or_)

	return &or_, nil
}

// GetStats returns attendance statistics.
func (s *Service) GetStats(ctx context.Context, merchantID int64, params StatsParams) (*AttendanceStats, error) {
	if params.StartDate == "" {
		params.StartDate = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	}
	if params.EndDate == "" {
		params.EndDate = time.Now().Format("2006-01-02")
	}

	stats := &AttendanceStats{
		Period:      params.StartDate + " ~ " + params.EndDate,
		LeaveByType: make(map[string]int),
	}

	// Work day count in the range (exclude weekends).
	startDate, _ := time.Parse("2006-01-02", params.StartDate)
	endDate, _ := time.Parse("2006-01-02", params.EndDate)
	totalDays := 0
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			totalDays++
		}
	}
	stats.TotalDays = totalDays

	// Attendance records stats.
	whereExtra := ""
	args := []interface{}{merchantID, params.StartDate, params.EndDate}
	if params.EmployeeID > 0 {
		whereExtra = " AND employee_id = $4"
		args = append(args, params.EmployeeID)
	}

	// Present days (has check-in).
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM attendance_records
		 WHERE merchant_id = $1 AND record_date >= $2 AND record_date <= $3
		 AND deleted_at IS NULL AND check_in_time IS NOT NULL`+whereExtra,
		args...,
	).Scan(&stats.PresentDays)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count present days", err)
	}

	// Late count.
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM attendance_records
		 WHERE merchant_id = $1 AND record_date >= $2 AND record_date <= $3
		 AND deleted_at IS NULL AND late_minutes > 0`+whereExtra,
		args...,
	).Scan(&stats.LateCount)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count late days", err)
	}

	// Early leave count.
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM attendance_records
		 WHERE merchant_id = $1 AND record_date >= $2 AND record_date <= $3
		 AND deleted_at IS NULL AND early_leave_minutes > 0`+whereExtra,
		args...,
	).Scan(&stats.EarlyLeaveCount)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count early leave days", err)
	}

	// Leave days (approved leave requests).
	leaveArgs := []interface{}{merchantID, params.StartDate, params.EndDate}
	if params.EmployeeID > 0 {
		leaveArgs = append(leaveArgs, params.EmployeeID)
	}
	leaveQuery := `SELECT COALESCE(SUM(end_date - start_date + 1), 0) FROM leave_requests
		 WHERE merchant_id = $1 AND status = 'approved' AND deleted_at IS NULL
		 AND start_date <= $3 AND end_date >= $2`
	if params.EmployeeID > 0 {
		leaveQuery += " AND employee_id = $4"
	}
	err = s.db.QueryRowContext(ctx, leaveQuery, leaveArgs...).Scan(&stats.LeaveDays)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count leave days", err)
	}

	// Leave by type.
	ltArgs := []interface{}{merchantID, params.StartDate, params.EndDate}
	if params.EmployeeID > 0 {
		ltArgs = append(ltArgs, params.EmployeeID)
	}
	ltQuery := `SELECT leave_type, COALESCE(SUM(end_date - start_date + 1), 0)
		 FROM leave_requests
		 WHERE merchant_id = $1 AND status = 'approved' AND deleted_at IS NULL
		 AND start_date <= $3 AND end_date >= $2`
	if params.EmployeeID > 0 {
		ltQuery += " AND employee_id = $4"
	}
	ltQuery += " GROUP BY leave_type"
	ltRows, err := s.db.QueryContext(ctx, ltQuery, ltArgs...)
	if err == nil {
		defer ltRows.Close()
		for ltRows.Next() {
			var lt string
			var days int
			if err := ltRows.Scan(&lt, &days); err == nil {
				stats.LeaveByType[lt] = days
			}
		}
	}

	// Overtime hours (approved).
	otArgs := []interface{}{merchantID, params.StartDate, params.EndDate}
	if params.EmployeeID > 0 {
		otArgs = append(otArgs, params.EmployeeID)
	}
	otQuery := `SELECT COALESCE(SUM(duration_hours), 0) FROM overtime_records
		 WHERE merchant_id = $1 AND status = 'approved' AND deleted_at IS NULL
		 AND overtime_date >= $2 AND overtime_date <= $3`
	if params.EmployeeID > 0 {
		otQuery += " AND employee_id = $4"
	}
	err = s.db.QueryRowContext(ctx, otQuery, otArgs...).Scan(&stats.OvertimeHours)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count overtime hours", err)
	}

	return stats, nil
}

// fillEmployeeName fetches and sets employee name/no on a LeaveRequest.
func (s *Service) fillEmployeeName(ctx context.Context, lr *LeaveRequest) {
	s.db.QueryRowContext(ctx,
		`SELECT COALESCE(name, ''), COALESCE(employee_no, '') FROM employees WHERE id = $1 AND deleted_at IS NULL`,
		lr.EmployeeID,
	).Scan(&lr.EmployeeName, &lr.EmployeeNo)
}

// fillOvertimeEmployeeName fetches and sets employee name/no on an OvertimeRecord.
func (s *Service) fillOvertimeEmployeeName(ctx context.Context, or_ *OvertimeRecord) {
	s.db.QueryRowContext(ctx,
		`SELECT COALESCE(name, ''), COALESCE(employee_no, '') FROM employees WHERE id = $1 AND deleted_at IS NULL`,
		or_.EmployeeID,
	).Scan(&or_.EmployeeName, &or_.EmployeeNo)
}
