package notification

import "time"

// NotificationTemplate mirrors notification_templates.
type NotificationTemplate struct {
	ID      int64  `gorm:"primaryKey" json:"id"`
	Code    string `gorm:"size:32" json:"code"`
	Channel string `gorm:"size:16" json:"channel"`
	Title   string `gorm:"size:128" json:"title"`
	Content string `gorm:"type:text" json:"content"`
	Status  int16  `json:"status"`
}

func (NotificationTemplate) TableName() string { return "notification_templates" }

// NotificationLog mirrors notification_logs.
type NotificationLog struct {
	ID           int64      `gorm:"primaryKey" json:"id"`
	StoreID      int64      `json:"store_id"`
	CustomerID   *int64     `json:"customer_id"`
	TemplateCode string     `gorm:"size:32" json:"template_code"`
	Channel      string     `gorm:"size:16" json:"channel"`
	Payload      string     `gorm:"type:jsonb" json:"payload"`
	Status       string     `gorm:"size:16;default:pending" json:"status"`
	Error        string     `gorm:"size:255" json:"error"`
	RetryCount   int16      `json:"retry_count"`
	ScheduledAt  *time.Time `json:"scheduled_at"`
	SentAt       *time.Time `json:"sent_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (NotificationLog) TableName() string { return "notification_logs" }

type VaccineDuePet struct {
	StoreID    int64
	CustomerID int64
	PetID      int64
	PetName    string
	DueAt      time.Time
}

// Statuses
const (
	StatusPending = "pending"
	StatusSent    = "sent"
	StatusFailed  = "failed"
	StatusSkipped = "skipped"
)

// Channels
const (
	ChannelInApp    = "inapp"
	ChannelSMS      = "sms"
	ChannelWechatMp = "wechat_mp"
)
