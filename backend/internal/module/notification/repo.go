package notification

import (
	"gorm.io/gorm"
)

// Repository defines the data access interface for notifications.
type Repository interface {
	CreateLog(log *NotificationLog) error
	FindTemplate(code, channel string) (*NotificationTemplate, error)
	FindPendingLogs(limit int) ([]NotificationLog, error)
}

type repo struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repo{db: db}
}

func (r *repo) CreateLog(log *NotificationLog) error {
	return r.db.Create(log).Error
}

func (r *repo) FindTemplate(code, channel string) (*NotificationTemplate, error) {
	var t NotificationTemplate
	err := r.db.Where("code = ? AND channel = ? AND status = 1", code, channel).First(&t).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound { return nil, nil }
		return nil, err
	}
	return &t, nil
}

func (r *repo) FindPendingLogs(limit int) ([]NotificationLog, error) {
	var logs []NotificationLog
	err := r.db.Where("status = ? AND retry_count < 3", StatusPending).
		Order("created_at ASC").Limit(limit).Find(&logs).Error
	return logs, err
}
