package pet

import (
	"gorm.io/gorm"
)

// Repository defines the data access interface for pets.
type Repository interface {
	Create(p *Pet) error
	FindByID(id int64) (*Pet, error)
	Update(p *Pet) error
	ListByCustomer(customerID int64) ([]Pet, error)
	CreateHealthRecord(r *HealthRecord) error
	FindHealthRecords(petID int64) ([]HealthRecord, error)
	CreateWeightRecord(r *WeightRecord) error
	FindWeightRecords(petID int64) ([]WeightRecord, error)
}

type repo struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repo{db: db}
}

func (r *repo) Create(p *Pet) error { return r.db.Create(p).Error }
func (r *repo) FindByID(id int64) (*Pet, error) {
	var p Pet
	err := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&p).Error
	return &p, err
}
func (r *repo) Update(p *Pet) error { return r.db.Save(p).Error }
func (r *repo) ListByCustomer(customerID int64) ([]Pet, error) {
	var list []Pet
	err := r.db.Where("customer_id = ? AND deleted_at IS NULL", customerID).Find(&list).Error
	return list, err
}
func (r *repo) CreateHealthRecord(hr *HealthRecord) error { return r.db.Create(hr).Error }
func (r *repo) FindHealthRecords(petID int64) ([]HealthRecord, error) {
	var list []HealthRecord
	err := r.db.Where("pet_id = ?", petID).Order("created_at DESC").Find(&list).Error
	return list, err
}
func (r *repo) CreateWeightRecord(wr *WeightRecord) error { return r.db.Create(wr).Error }
func (r *repo) FindWeightRecords(petID int64) ([]WeightRecord, error) {
	var list []WeightRecord
	err := r.db.Where("pet_id = ?", petID).Order("recorded_at ASC").Find(&list).Error
	return list, err
}
