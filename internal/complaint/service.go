package complaint

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/xltxb/PetManage/internal/middleware"
	apperrors "github.com/xltxb/PetManage/pkg/apperrors"
)

// Ticket represents a complaint ticket.
type Ticket struct {
	ID            int64      `json:"id"`
	MerchantID    int64      `json:"merchant_id"`
	MerchantName  string     `json:"merchant_name,omitempty"`
	ComplaintType string     `json:"complaint_type"`
	Description   string     `json:"description"`
	Status        string     `json:"status"`
	AssignedTo    *int64     `json:"assigned_to"`
	AssigneeName  string     `json:"assignee_name,omitempty"`
	Resolution    string     `json:"resolution"`
	RevisitNotes  string     `json:"revisit_notes"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// CreateTicketRequest is the input for creating a complaint ticket.
type CreateTicketRequest struct {
	MerchantID    int64  `json:"merchant_id"`
	ComplaintType string `json:"complaint_type"`
	Description   string `json:"description"`
}

// AssignTicketRequest is the input for assigning a ticket.
type AssignTicketRequest struct {
	AssignedTo int64 `json:"assigned_to"`
}

// UpdateProgressRequest is the input for updating ticket progress.
type UpdateProgressRequest struct {
	Resolution string `json:"resolution"`
}

// UpdateStatusRequest is the input for changing ticket status.
type UpdateStatusRequest struct {
	Status       string `json:"status"`
	RevisitNotes string `json:"revisit_notes,omitempty"`
}

// ListParams holds filter/pagination parameters for listing tickets.
type ListParams struct {
	MerchantID    int64  `json:"merchant_id"`
	Status        string `json:"status"`
	ComplaintType string `json:"complaint_type"`
	AssignedTo    int64  `json:"assigned_to"`
	Page          int    `json:"page"`
	PageSize      int    `json:"page_size"`
}

// ListResponse is a paginated ticket list.
type ListResponse struct {
	Tickets  []Ticket `json:"tickets"`
	Total    int      `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
}

// ComplaintStats holds complaint rate statistics.
type ComplaintStats struct {
	TotalComplaints   int              `json:"total_complaints"`
	PendingCount      int              `json:"pending_count"`
	ProcessingCount   int              `json:"processing_count"`
	ResolvedCount     int              `json:"resolved_count"`
	RevisitedCount    int              `json:"revisited_count"`
	MerchantBreakdown []MerchantBreak  `json:"merchant_breakdown"`
}

// MerchantBreak holds per-merchant complaint stats.
type MerchantBreak struct {
	MerchantID      int64   `json:"merchant_id"`
	MerchantName    string  `json:"merchant_name"`
	ComplaintCount  int     `json:"complaint_count"`
	ComplaintRate   float64 `json:"complaint_rate"`
}

// Service handles complaint ticket operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new complaint Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// CreateTicket creates a new complaint ticket.
func (s *Service) CreateTicket(ctx context.Context, req CreateTicketRequest) (*Ticket, error) {
	if req.MerchantID == 0 {
		return nil, apperrors.NewValidationError("merchant_id is required")
	}
	if req.ComplaintType == "" {
		return nil, apperrors.NewValidationError("complaint_type is required")
	}
	if req.Description == "" {
		return nil, apperrors.NewValidationError("description is required")
	}
	validTypes := map[string]bool{"service": true, "product": true, "staff": true, "pricing": true, "other": true}
	if !validTypes[req.ComplaintType] {
		return nil, apperrors.NewValidationError("complaint_type must be one of: service, product, staff, pricing, other")
	}

	var merchantExists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM merchants WHERE id = $1 AND deleted_at IS NULL)`, req.MerchantID).Scan(&merchantExists)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to check merchant", err)
	}
	if !merchantExists {
		return nil, apperrors.NewNotFoundError("merchant not found")
	}

	var t Ticket
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO complaint_tickets (merchant_id, complaint_type, description)
		 VALUES ($1, $2, $3)
		 RETURNING id, merchant_id, complaint_type, description, status, assigned_to, resolution, revisit_notes, created_at, updated_at`,
		req.MerchantID, req.ComplaintType, req.Description,
	).Scan(&t.ID, &t.MerchantID, &t.ComplaintType, &t.Description, &t.Status, &t.AssignedTo, &t.Resolution, &t.RevisitNotes, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create ticket", err)
	}

	s.recordLog(ctx, 0, "create_complaint", "complaint", t.ID, map[string]interface{}{
		"merchant_id":    req.MerchantID,
		"complaint_type": req.ComplaintType,
	})
	return &t, nil
}

// AssignTicket assigns a ticket to a platform user for handling.
func (s *Service) AssignTicket(ctx context.Context, ticketID int64, req AssignTicketRequest) (*Ticket, error) {
	if req.AssignedTo == 0 {
		return nil, apperrors.NewValidationError("assigned_to is required")
	}

	var userExists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM platform_users WHERE id = $1 AND deleted_at IS NULL)`, req.AssignedTo).Scan(&userExists)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to check user", err)
	}
	if !userExists {
		return nil, apperrors.NewNotFoundError("user not found")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	var currentStatus string
	err = tx.QueryRowContext(ctx, `SELECT status FROM complaint_tickets WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, ticketID).Scan(&currentStatus)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("ticket not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query ticket", err)
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE complaint_tickets SET assigned_to = $1, status = CASE WHEN status = 'pending' THEN 'processing' ELSE status END, updated_at = NOW()
		 WHERE id = $2`,
		req.AssignedTo, ticketID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to assign ticket", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit", err)
	}

	t, err := s.GetTicket(ctx, ticketID)
	if err != nil {
		return nil, err
	}

	s.recordLog(ctx, 0, "assign_complaint", "complaint", ticketID, map[string]interface{}{
		"assigned_to": req.AssignedTo,
	})
	return t, nil
}

// UpdateProgress updates the resolution/progress of a ticket.
func (s *Service) UpdateProgress(ctx context.Context, ticketID int64, req UpdateProgressRequest) (*Ticket, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	var currentStatus string
	err = tx.QueryRowContext(ctx, `SELECT status FROM complaint_tickets WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, ticketID).Scan(&currentStatus)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("ticket not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query ticket", err)
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE complaint_tickets SET resolution = $1, updated_at = NOW() WHERE id = $2`,
		req.Resolution, ticketID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update progress", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit", err)
	}

	t, err := s.GetTicket(ctx, ticketID)
	if err != nil {
		return nil, err
	}

	s.recordLog(ctx, 0, "update_complaint_progress", "complaint", ticketID, map[string]interface{}{
		"resolution": req.Resolution,
	})
	return t, nil
}

// UpdateStatus transitions a ticket to a new status.
func (s *Service) UpdateStatus(ctx context.Context, ticketID int64, req UpdateStatusRequest) (*Ticket, error) {
	validStatuses := map[string]bool{"pending": true, "processing": true, "resolved": true, "revisited": true}
	if !validStatuses[req.Status] {
		return nil, apperrors.NewValidationError("status must be one of: pending, processing, resolved, revisited")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	var currentStatus string
	err = tx.QueryRowContext(ctx, `SELECT status FROM complaint_tickets WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, ticketID).Scan(&currentStatus)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("ticket not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query ticket", err)
	}

	// Validate status transitions
	validTransition := false
	switch currentStatus {
	case "pending":
		validTransition = req.Status == "processing"
	case "processing":
		validTransition = req.Status == "resolved"
	case "resolved":
		validTransition = req.Status == "revisited"
	case "revisited":
		validTransition = false
	}
	if !validTransition {
		return nil, apperrors.NewValidationError("invalid status transition: cannot change from " + currentStatus + " to " + req.Status)
	}

	if req.Status == "revisited" && req.RevisitNotes == "" {
		return nil, apperrors.NewValidationError("revisit_notes is required when transitioning to revisited")
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE complaint_tickets SET status = $1, revisit_notes = CASE WHEN $3 != '' THEN $3 ELSE revisit_notes END, updated_at = NOW() WHERE id = $2`,
		req.Status, ticketID, req.RevisitNotes)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update status", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit", err)
	}

	t, err := s.GetTicket(ctx, ticketID)
	if err != nil {
		return nil, err
	}

	s.recordLog(ctx, 0, "update_complaint_status", "complaint", ticketID, map[string]interface{}{
		"from_status": currentStatus,
		"to_status":   req.Status,
	})
	return t, nil
}

// GetTicket retrieves a single ticket with merchant and assignee names.
func (s *Service) GetTicket(ctx context.Context, ticketID int64) (*Ticket, error) {
	var t Ticket
	err := s.db.QueryRowContext(ctx,
		`SELECT ct.id, ct.merchant_id, m.name AS merchant_name, ct.complaint_type, ct.description,
		        ct.status, ct.assigned_to, COALESCE(pu.display_name, pu.username, '') AS assignee_name,
		        ct.resolution, ct.revisit_notes, ct.created_at, ct.updated_at
		 FROM complaint_tickets ct
		 LEFT JOIN merchants m ON ct.merchant_id = m.id
		 LEFT JOIN platform_users pu ON ct.assigned_to = pu.id
		 WHERE ct.id = $1 AND ct.deleted_at IS NULL`, ticketID,
	).Scan(&t.ID, &t.MerchantID, &t.MerchantName, &t.ComplaintType, &t.Description,
		&t.Status, &t.AssignedTo, &t.AssigneeName,
		&t.Resolution, &t.RevisitNotes, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("ticket not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query ticket", err)
	}
	return &t, nil
}

// ListTickets returns a filtered, paginated list of tickets.
func (s *Service) ListTickets(ctx context.Context, params ListParams) (*ListResponse, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	conditions = append(conditions, "ct.deleted_at IS NULL")

	if params.MerchantID > 0 {
		conditions = append(conditions, fmt.Sprintf("ct.merchant_id = $%d", argIdx))
		args = append(args, params.MerchantID)
		argIdx++
	}
	if params.Status != "" {
		conditions = append(conditions, fmt.Sprintf("ct.status = $%d", argIdx))
		args = append(args, params.Status)
		argIdx++
	}
	if params.ComplaintType != "" {
		conditions = append(conditions, fmt.Sprintf("ct.complaint_type = $%d", argIdx))
		args = append(args, params.ComplaintType)
		argIdx++
	}
	if params.AssignedTo > 0 {
		conditions = append(conditions, fmt.Sprintf("ct.assigned_to = $%d", argIdx))
		args = append(args, params.AssignedTo)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + conditions[0]
		for _, c := range conditions[1:] {
			whereClause += " AND " + c
		}
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM complaint_tickets ct " + whereClause
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count tickets", err)
	}

	offset := (params.Page - 1) * params.PageSize
	dataQuery := fmt.Sprintf(
		`SELECT ct.id, ct.merchant_id, m.name AS merchant_name, ct.complaint_type, ct.description,
		        ct.status, ct.assigned_to, COALESCE(pu.display_name, pu.username, '') AS assignee_name,
		        ct.resolution, ct.revisit_notes, ct.created_at, ct.updated_at
		 FROM complaint_tickets ct
		 LEFT JOIN merchants m ON ct.merchant_id = m.id
		 LEFT JOIN platform_users pu ON ct.assigned_to = pu.id
		 %s
		 ORDER BY ct.created_at DESC
		 LIMIT $%d OFFSET $%d`, whereClause, argIdx, argIdx+1)

	dataArgs := append(args, params.PageSize, offset)
	rows, err := s.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query tickets", err)
	}
	defer rows.Close()

	var tickets []Ticket
	for rows.Next() {
		var t Ticket
		if err := rows.Scan(&t.ID, &t.MerchantID, &t.MerchantName, &t.ComplaintType, &t.Description,
			&t.Status, &t.AssignedTo, &t.AssigneeName, &t.Resolution, &t.RevisitNotes, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan ticket", err)
		}
		tickets = append(tickets, t)
	}

	return &ListResponse{
		Tickets:  tickets,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, nil
}

// GetComplaintStats returns complaint rate statistics.
func (s *Service) GetComplaintStats(ctx context.Context) (*ComplaintStats, error) {
	stats := &ComplaintStats{}

	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM complaint_tickets WHERE deleted_at IS NULL`).Scan(&stats.TotalComplaints)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count total complaints", err)
	}

	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM complaint_tickets WHERE deleted_at IS NULL AND status = 'pending'`).Scan(&stats.PendingCount)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count pending", err)
	}

	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM complaint_tickets WHERE deleted_at IS NULL AND status = 'processing'`).Scan(&stats.ProcessingCount)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count processing", err)
	}

	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM complaint_tickets WHERE deleted_at IS NULL AND status = 'resolved'`).Scan(&stats.ResolvedCount)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count resolved", err)
	}

	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM complaint_tickets WHERE deleted_at IS NULL AND status = 'revisited'`).Scan(&stats.RevisitedCount)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count revisited", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT m.id, m.name, COUNT(ct.id) AS complaint_count
		 FROM merchants m
		 LEFT JOIN complaint_tickets ct ON ct.merchant_id = m.id AND ct.deleted_at IS NULL
		 WHERE m.deleted_at IS NULL
		 GROUP BY m.id, m.name
		 ORDER BY complaint_count DESC`)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query merchant breakdown", err)
	}
	defer rows.Close()

	for rows.Next() {
		var mb MerchantBreak
		if err := rows.Scan(&mb.MerchantID, &mb.MerchantName, &mb.ComplaintCount); err != nil {
			return nil, apperrors.NewInternalError("failed to scan breakdown", err)
		}
		if stats.TotalComplaints > 0 {
			mb.ComplaintRate = float64(mb.ComplaintCount) / float64(stats.TotalComplaints) * 100
		}
		stats.MerchantBreakdown = append(stats.MerchantBreakdown, mb)
	}

	return stats, nil
}

func (s *Service) recordLog(ctx context.Context, userID int64, action, targetType string, targetID int64, detail map[string]interface{}) {
	detailJSON, _ := json.Marshal(detail)
	ip := middleware.ClientIPFromContext(ctx)
	_, _ = s.db.ExecContext(ctx,
		`INSERT INTO operation_logs (user_id, action, target_type, target_id, detail, ip_address)
		 VALUES ($1, $2, $3, $4, $5::jsonb, $6)`,
		userID, action, targetType, targetID, string(detailJSON), ip,
	)
}
