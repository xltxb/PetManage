package boarding

import (
	"time"

	"gorm.io/gorm"
)

// Repository defines the data access interface for boarding.
type Repository interface {
	FindRoomByID(roomID int64) (*BoardingRoom, error)
	FindFreeRoom(storeID, roomTypeID int64) (*BoardingRoom, error)
	FindOrderByID(id, storeID int64) (*BoardingOrder, error)
	UpdateRoom(r *BoardingRoom) error
	CreateOrder(o *BoardingOrder) error
	UpdateOrder(o *BoardingOrder) error
	CreateCareLog(cl *CareLog) error
	FindCareLogs(orderID int64, date time.Time) ([]CareLog, error)
	ListOrders(storeID int64, status string, page, pageSize int) ([]BoardingOrder, int64, error)
	WithTx(fn func(Repository) error) error
}

type repo struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repo{db: db}
}

func (r *repo) FindRoomByID(roomID int64) (*BoardingRoom, error) {
	var room BoardingRoom
	err := r.db.First(&room, roomID).Error
	return &room, err
}

func (r *repo) FindFreeRoom(storeID, roomTypeID int64) (*BoardingRoom, error) {
	var room BoardingRoom
	err := r.db.Where("store_id = ? AND room_type_id = ? AND status = ?", storeID, roomTypeID, RoomStatusFree).
		First(&room).Error
	return &room, err
}

func (r *repo) FindOrderByID(id, storeID int64) (*BoardingOrder, error) {
	var o BoardingOrder
	err := r.db.Where("id = ? AND store_id = ? AND deleted_at IS NULL", id, storeID).First(&o).Error
	return &o, err
}

func (r *repo) UpdateRoom(room *BoardingRoom) error {
	return r.db.Save(room).Error
}

func (r *repo) CreateOrder(o *BoardingOrder) error {
	return r.db.Create(o).Error
}

func (r *repo) UpdateOrder(o *BoardingOrder) error {
	return r.db.Save(o).Error
}

func (r *repo) CreateCareLog(cl *CareLog) error {
	return r.db.Create(cl).Error
}

func (r *repo) FindCareLogs(orderID int64, date time.Time) ([]CareLog, error) {
	var logs []CareLog
	err := r.db.Where("boarding_order_id = ? AND log_date = ?", orderID, date.Format("2006-01-02")).
		Find(&logs).Error
	return logs, err
}

func (r *repo) ListOrders(storeID int64, status string, page, pageSize int) ([]BoardingOrder, int64, error) {
	var list []BoardingOrder
	var total int64
	q := r.db.Where("store_id = ? AND deleted_at IS NULL", storeID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	q.Model(&BoardingOrder{}).Count(&total)
	offset := (page - 1) * pageSize
	err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (r *repo) WithTx(fn func(Repository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return fn(&repo{db: tx})
	})
}
