package appointment

import (
	"time"

	"gorm.io/gorm"
)

// Repository defines the data access interface for appointments.
type Repository interface {
	FindByID(id int64) (*Appointment, error)
	FindByIDWithStore(id, storeID int64) (*Appointment, error)
	CheckResourceConflict(storeID, stationID int64, start, end time.Time, excludeID int64) (bool, error)
	Create(a *Appointment) error
	Update(a *Appointment) error
	CreateItems(items []AppointmentItem) error
	FindItems(appointmentID int64) ([]AppointmentItem, error)
	ListByStore(storeID int64, status string, start, end time.Time, page, pageSize int) ([]Appointment, int64, error)
}

type repo struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repo{db: db}
}

func (r *repo) FindByID(id int64) (*Appointment, error) {
	var a Appointment
	err := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&a).Error
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *repo) FindByIDWithStore(id, storeID int64) (*Appointment, error) {
	var a Appointment
	err := r.db.Where("id = ? AND store_id = ? AND deleted_at IS NULL", id, storeID).First(&a).Error
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *repo) CheckResourceConflict(storeID, stationID int64, start, end time.Time, excludeID int64) (bool, error) {
	var count int64
	query := r.db.Table("appointments").
		Where("store_id = ? AND station_id = ? AND deleted_at IS NULL", storeID, stationID).
		Where("status NOT IN ('cancelled','no_show','completed')").
		Where("scheduled_start < ? AND scheduled_end > ?", end, start)

	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}

	err := query.Count(&count).Error
	return count > 0, err
}

func (r *repo) Create(a *Appointment) error {
	return r.db.Create(a).Error
}

func (r *repo) Update(a *Appointment) error {
	return r.db.Save(a).Error
}

func (r *repo) CreateItems(items []AppointmentItem) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.Create(&items).Error
}

func (r *repo) FindItems(appointmentID int64) ([]AppointmentItem, error) {
	var items []AppointmentItem
	err := r.db.Where("appointment_id = ?", appointmentID).Find(&items).Error
	return items, err
}

func (r *repo) ListByStore(storeID int64, status string, start, end time.Time, page, pageSize int) ([]Appointment, int64, error) {
	var list []Appointment
	var total int64

	q := r.db.Where("store_id = ? AND deleted_at IS NULL", storeID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if !start.IsZero() {
		q = q.Where("scheduled_start >= ?", start)
	}
	if !end.IsZero() {
		q = q.Where("scheduled_start < ?", end)
	}

	if err := q.Model(&Appointment{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := q.Order("scheduled_start ASC").Offset(offset).Limit(pageSize).Find(&list).Error
	return list, total, err
}
