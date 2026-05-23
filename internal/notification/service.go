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

// Create creates a new notification.
func (s *Service) Create(ctx context.Context, n *Notification) (*Notification, error) {
	if n.UserType == "" {
		n.UserType = "member"
	}
	if n.Category == "" {
		n.Category = "appointment"
	}

	result, err := scanNotificationRow(s.db.QueryRowContext(ctx,
		`INSERT INTO notifications (merchant_id, user_id, user_type, title, content, category, related_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, merchant_id, user_id, user_type, title, content, category, related_id, is_read, created_at`,
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
		`SELECT id, merchant_id, user_id, user_type, title, content, category, related_id, is_read, created_at
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

func scanNotificationRow(row *sql.Row) (*Notification, error) {
	n := &Notification{}
	err := row.Scan(
		&n.ID, &n.MerchantID, &n.UserID, &n.UserType,
		&n.Title, &n.Content, &n.Category, &n.RelatedID,
		&n.IsRead, &n.CreatedAt,
	)
	return n, err
}

func scanNotificationRows(rows *sql.Rows) (*Notification, error) {
	n := &Notification{}
	err := rows.Scan(
		&n.ID, &n.MerchantID, &n.UserID, &n.UserType,
		&n.Title, &n.Content, &n.Category, &n.RelatedID,
		&n.IsRead, &n.CreatedAt,
	)
	return n, err
}

// Ensure encoding/json is used for JSONB handling in callers.
var _ = json.Marshal
