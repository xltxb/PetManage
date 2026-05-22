package operationlog

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// QueryParams holds filter and pagination parameters for operation log queries.
type QueryParams struct {
	UserID     *int64  `json:"user_id"`
	Action     *string `json:"action"`
	TargetType *string `json:"target_type"`
	StartTime  *string `json:"start_time"`
	EndTime    *string `json:"end_time"`
	Page       int     `json:"page"`
	PageSize   int     `json:"page_size"`
}

// LogEntry is a single operation log record with username.
type LogEntry struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	Username   string `json:"username,omitempty"`
	Action     string `json:"action"`
	TargetType string `json:"target_type"`
	TargetID   int64  `json:"target_id"`
	Detail     string `json:"detail,omitempty"`
	IPAddress  string `json:"ip_address,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// ListResponse is the paginated response for operation log queries.
type ListResponse struct {
	Logs     []LogEntry `json:"logs"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}

// Service provides operation log query capabilities.
type Service struct {
	db *sql.DB
}

// New creates a new Service.
func New(db *sql.DB) *Service {
	return &Service{db: db}
}

// defaults sets sensible defaults for pagination.
func (p *QueryParams) defaults() {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.PageSize <= 0 {
		p.PageSize = 20
	}
	if p.PageSize > 100 {
		p.PageSize = 100
	}
}

// Query retrieves operation logs with filtering and pagination.
func (s *Service) Query(ctx context.Context, params QueryParams) (*ListResponse, error) {
	params.defaults()

	var conditions []string
	var args []interface{}
	argIdx := 1

	if params.UserID != nil {
		conditions = append(conditions, fmt.Sprintf("ol.user_id = $%d", argIdx))
		args = append(args, *params.UserID)
		argIdx++
	}
	if params.Action != nil && *params.Action != "" {
		conditions = append(conditions, fmt.Sprintf("ol.action = $%d", argIdx))
		args = append(args, *params.Action)
		argIdx++
	}
	if params.TargetType != nil && *params.TargetType != "" {
		conditions = append(conditions, fmt.Sprintf("ol.target_type = $%d", argIdx))
		args = append(args, *params.TargetType)
		argIdx++
	}
	if params.StartTime != nil && *params.StartTime != "" {
		conditions = append(conditions, fmt.Sprintf("ol.created_at >= $%d::timestamptz", argIdx))
		args = append(args, *params.StartTime)
		argIdx++
	}
	if params.EndTime != nil && *params.EndTime != "" {
		conditions = append(conditions, fmt.Sprintf("ol.created_at <= $%d::timestamptz", argIdx))
		args = append(args, *params.EndTime)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count query.
	countQuery := fmt.Sprintf(
		`SELECT COUNT(*) FROM operation_logs ol %s`, whereClause,
	)
	var total int64
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to count operation logs",
			Err:     err,
		}
	}

	// Data query with left join on platform_users for username.
	offset := (params.Page - 1) * params.PageSize
	dataQuery := fmt.Sprintf(
		`SELECT ol.id, ol.user_id, COALESCE(u.username, ''), ol.action, ol.target_type,
		        ol.target_id, COALESCE(ol.detail::text, ''), COALESCE(ol.ip_address, ''), ol.created_at
		 FROM operation_logs ol
		 LEFT JOIN platform_users u ON ol.user_id = u.id
		 %s
		 ORDER BY ol.created_at DESC
		 LIMIT $%d OFFSET $%d`,
		whereClause, argIdx, argIdx+1,
	)
	args = append(args, params.PageSize, offset)

	rows, err := s.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to query operation logs",
			Err:     err,
		}
	}
	defer rows.Close()

	var logs []LogEntry
	for rows.Next() {
		var entry LogEntry
		var createdAt time.Time
		if err := rows.Scan(&entry.ID, &entry.UserID, &entry.Username, &entry.Action,
			&entry.TargetType, &entry.TargetID, &entry.Detail, &entry.IPAddress, &createdAt); err != nil {
			return nil, &apperrors.AppError{
				Code:    apperrors.CodeInternalError,
				Message: "failed to scan operation log",
				Err:     err,
			}
		}
		entry.CreatedAt = createdAt.Format(time.RFC3339)
		logs = append(logs, entry)
	}

	if logs == nil {
		logs = []LogEntry{}
	}

	return &ListResponse{
		Logs:     logs,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, nil
}
