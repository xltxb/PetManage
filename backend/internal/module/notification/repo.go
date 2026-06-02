package notification

import (
	"time"

	"gorm.io/gorm"
)

// Repository defines the data access interface for notifications.
type Repository interface {
	CreateLog(log *NotificationLog) error
	FindTemplate(code, channel string) (*NotificationTemplate, error)
	FindPendingLogs(limit int) ([]NotificationLog, error)
	FindVaccineDue(now time.Time, days int) ([]VaccineDuePet, error)
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
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
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

func (r *repo) FindVaccineDue(now time.Time, days int) ([]VaccineDuePet, error) {
	var rows []VaccineDuePet
	err := r.db.Table("pet_health_records phr").
		Select("p.store_id, p.customer_id, p.id AS pet_id, p.name AS pet_name, phr.next_due_at AS due_at").
		Joins("JOIN pets p ON p.id = phr.pet_id AND p.deleted_at IS NULL").
		Where("phr.type = ? AND phr.next_due_at >= ? AND phr.next_due_at < ?", "vaccine", now, now.AddDate(0, 0, days)).
		Scan(&rows).Error
	return rows, err
}
