package wx

import (
	"gorm.io/gorm"

	"pawprint/backend/internal/module/member"
)

type Repository interface {
	FindCustomerByOpenID(openID string) (*member.Customer, error)
	CreateCustomer(c *member.Customer) error
	ListBookableOfferings(storeID int64) ([]ServiceOffering, error)
	FindOffering(id, storeID int64) (*ServiceOffering, error)
}

type repo struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repo{db: db}
}

func (r *repo) FindCustomerByOpenID(openID string) (*member.Customer, error) {
	var c member.Customer
	err := r.db.Where("wechat_open_id = ? AND deleted_at IS NULL", openID).First(&c).Error
	return &c, err
}

func (r *repo) CreateCustomer(c *member.Customer) error {
	return r.db.Create(c).Error
}

func (r *repo) ListBookableOfferings(storeID int64) ([]ServiceOffering, error) {
	var offerings []ServiceOffering
	err := r.db.Table("service_offerings so").
		Select("so.id, s.name, so.price, so.duration_min").
		Joins("JOIN services s ON s.id = so.service_id AND s.deleted_at IS NULL").
		Where("so.store_id = ? AND so.bookable_online = true AND so.status = 1", storeID).
		Order("so.id ASC").
		Scan(&offerings).Error
	return offerings, err
}

func (r *repo) FindOffering(id, storeID int64) (*ServiceOffering, error) {
	var offering ServiceOffering
	err := r.db.Table("service_offerings so").
		Select("so.id, s.name, so.price, so.duration_min").
		Joins("JOIN services s ON s.id = so.service_id AND s.deleted_at IS NULL").
		Where("so.id = ? AND so.store_id = ? AND so.bookable_online = true AND so.status = 1", id, storeID).
		First(&offering).Error
	return &offering, err
}
