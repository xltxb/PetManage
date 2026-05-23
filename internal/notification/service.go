package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Notification represents a notification record.
type Notification struct {
	ID         int64     `json:"id"`
	MerchantID int64     `json:"merchant_id"`
	UserID     int64     `json:"user_id"`
	UserType   string    `json:"user_type"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Category   string    `json:"category"`
	RelatedID  int64     `json:"related_id"`
	IsRead     bool      `json:"is_read"`
	SendStatus string    `json:"send_status"`
	Channel    string    `json:"channel"`
	CreatedAt  time.Time `json:"created_at"`
}

// Service provides notification management.
type Service struct {
	db *sql.DB
}

// NewService creates a new notification Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

const notificationColumns = `id, merchant_id, user_id, user_type, title, content, category, related_id, is_read, send_status, channel, created_at`

// Create creates a new notification.
func (s *Service) Create(ctx context.Context, n *Notification) (*Notification, error) {
	if n.UserType == "" {
		n.UserType = "member"
	}
	if n.Category == "" {
		n.Category = "appointment"
	}

	result, err := scanNotificationRow(s.db.QueryRowContext(ctx,
		`INSERT INTO notifications (merchant_id, user_id, user_type, title, content, category, related_id, send_status, channel)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 'success', 'system')
		 RETURNING `+notificationColumns,
		n.MerchantID, n.UserID, n.UserType, n.Title, n.Content, n.Category, n.RelatedID,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create notification", err)
	}
	return result, nil
}

// ListByUser returns notifications for a specific user.
func (s *Service) ListByUser(ctx context.Context, merchantID, userID int64, userType string, limit int) ([]Notification, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+notificationColumns+`
		 FROM notifications
		 WHERE merchant_id = $1 AND user_id = $2 AND user_type = $3
		 ORDER BY created_at DESC LIMIT $4`,
		merchantID, userID, userType, limit,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list notifications", err)
	}
	defer rows.Close()

	var notifications []Notification
	for rows.Next() {
		n, err := scanNotificationRows(rows)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to scan notification", err)
		}
		notifications = append(notifications, *n)
	}
	if notifications == nil {
		notifications = []Notification{}
	}
	return notifications, rows.Err()
}

// ListParams holds optional filters for listing notifications.
type ListParams struct {
	Category string
	Status   string
	UserType string
	Page     int
	PageSize int
}

// ListResult wraps notification list with pagination info.
type ListResult struct {
	Notifications []Notification `json:"notifications"`
	Total         int            `json:"total"`
	Page          int            `json:"page"`
	PageSize      int            `json:"page_size"`
}

// List returns a filtered and paginated list of notifications for a merchant.
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
	conditions = append(conditions, "merchant_id = $1")
	argIdx := 2

	if params.Category != "" {
		conditions = append(conditions, "category = $"+itoa(argIdx))
		args = append(args, params.Category)
		argIdx++
	}
	if params.Status != "" {
		conditions = append(conditions, "send_status = $"+itoa(argIdx))
		args = append(args, params.Status)
		argIdx++
	}
	if params.UserType != "" {
		conditions = append(conditions, "user_type = $"+itoa(argIdx))
		args = append(args, params.UserType)
		argIdx++
	}

	whereClause := ""
	for i, c := range conditions {
		if i == 0 {
			whereClause = c
		} else {
			whereClause += " AND " + c
		}
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notifications WHERE `+whereClause,
		args...,
	).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count notifications", err)
	}

	offset := (params.Page - 1) * params.PageSize
	queryArgs := append(args, params.PageSize, offset)

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+notificationColumns+`
		 FROM notifications
		 WHERE `+whereClause+`
		 ORDER BY created_at DESC LIMIT $`+itoa(argIdx)+` OFFSET $`+itoa(argIdx+1),
		queryArgs...,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list notifications", err)
	}
	defer rows.Close()

	var notes []Notification
	for rows.Next() {
		n, err := scanNotificationRows(rows)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to scan notification", err)
		}
		notes = append(notes, *n)
	}
	if notes == nil {
		notes = []Notification{}
	}

	return &ListResult{
		Notifications: notes,
		Total:         total,
		Page:          params.Page,
		PageSize:      params.PageSize,
	}, rows.Err()
}

// MarkRead marks a notification as read.
func (s *Service) MarkRead(ctx context.Context, merchantID, notificationID int64) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE notifications SET is_read = TRUE
		 WHERE id = $1 AND merchant_id = $2`,
		notificationID, merchantID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to mark notification as read", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.NewNotFoundError("notification not found")
	}
	return nil
}

// SendAppointmentNotification is a helper that creates appointment-related notifications.
func (s *Service) SendAppointmentNotification(ctx context.Context, merchantID int64, appointmentID int64, userID int64, userType string, action string, details map[string]string) {
	title := ""
	content := ""

	switch action {
	case "created":
		title = "预约已创建"
		content = fmt.Sprintf("您的预约已创建，服务时间：%s，请等待确认", details["appointment_time"])
	case "confirmed":
		title = "预约已确认"
		content = fmt.Sprintf("您的预约已确认，服务时间：%s", details["appointment_time"])
	case "rescheduled":
		title = "预约已改期"
		content = fmt.Sprintf("您的预约已改期至 %s", details["appointment_time"])
	case "cancelled":
		title = "预约已取消"
		reason := details["reason"]
		if reason != "" {
			content = fmt.Sprintf("您的预约已取消，原因：%s", reason)
		} else {
			content = "您的预约已取消"
		}
	case "arrived":
		title = "宠物已到店"
		content = fmt.Sprintf("预约时间 %s 的宠物已到店，请准备开始服务", details["appointment_time"])
	case "completed":
		title = "服务已完成"
		content = "您的宠物服务已完成，请到店取宠"
	case "upcoming":
		title = "服务即将开始"
		content = fmt.Sprintf("您预约的服务将于 %s 开始，请准时到达", details["appointment_time"])
	}

	if title == "" {
		return
	}

	n := &Notification{
		MerchantID: merchantID,
		UserID:     userID,
		UserType:   userType,
		Title:      title,
		Content:    content,
		Category:   "appointment",
		RelatedID:  appointmentID,
	}

	s.Create(ctx, n) // fire-and-forget; errors are logged by the service layer
}

// SendUpcomingReminders finds appointments starting within the next 60 minutes
// and sends reminder notifications to members.
func (s *Service) SendUpcomingReminders(ctx context.Context) (int, error) {
	now := time.Now()
	windowEnd := now.Add(60 * time.Minute)

	rows, err := s.db.QueryContext(ctx,
		`SELECT a.id, a.merchant_id, a.member_id, a.employee_id, a.appointment_time,
		        COALESCE(m.name, ''), COALESCE(e.name, '')
		 FROM appointments a
		 LEFT JOIN members m ON m.id = a.member_id
		 LEFT JOIN employees e ON e.id = a.employee_id
		 WHERE a.status = 'confirmed'
		 AND a.deleted_at IS NULL
		 AND a.appointment_time > $1
		 AND a.appointment_time <= $2
		 AND a.id NOT IN (
		     SELECT related_id FROM notifications
		     WHERE category = 'appointment' AND title = '服务即将开始'
		     AND created_at > NOW() - INTERVAL '2 hours'
		 )`,
		now, windowEnd,
	)
	if err != nil {
		return 0, apperrors.NewInternalError("failed to query upcoming appointments", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, merchantID, memberID, employeeID int64
		var apptTime time.Time
		var memberName, employeeName string
		if err := rows.Scan(&id, &merchantID, &memberID, &employeeID, &apptTime, &memberName, &employeeName); err != nil {
			continue
		}

		timeStr := apptTime.Format("2006-01-02 15:04")
		s.SendAppointmentNotification(ctx, merchantID, id, memberID, "member", "upcoming",
			map[string]string{"appointment_time": timeStr})

		if employeeID > 0 {
			s.SendAppointmentNotification(ctx, merchantID, id, employeeID, "employee", "upcoming",
				map[string]string{"appointment_time": timeStr})
		}
		count++
	}

	return count, rows.Err()
}

func scanNotificationRow(row *sql.Row) (*Notification, error) {
	n := &Notification{}
	err := row.Scan(
		&n.ID, &n.MerchantID, &n.UserID, &n.UserType,
		&n.Title, &n.Content, &n.Category, &n.RelatedID,
		&n.IsRead, &n.SendStatus, &n.Channel, &n.CreatedAt,
	)
	return n, err
}

func scanNotificationRows(rows *sql.Rows) (*Notification, error) {
	n := &Notification{}
	err := rows.Scan(
		&n.ID, &n.MerchantID, &n.UserID, &n.UserType,
		&n.Title, &n.Content, &n.Category, &n.RelatedID,
		&n.IsRead, &n.SendStatus, &n.Channel, &n.CreatedAt,
	)
	return n, err
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}

// Ensure encoding/json is used for JSONB handling in callers.
var _ = json.Marshal
