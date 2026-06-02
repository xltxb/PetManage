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
	FindConsumptionRecords(petID int64) ([]ConsumptionRecord, error)
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
func (r *repo) FindConsumptionRecords(petID int64) ([]ConsumptionRecord, error) {
	var list []ConsumptionRecord
	err := r.db.Raw(`
		SELECT * FROM (
			SELECT
				'appointment' AS type,
				a.id AS source_id,
				a.scheduled_start AS occurred_at,
				COALESCE(NULLIF(ai.service_name, ''), '预约服务') AS title,
				COALESCE(a.total_amount, 0) AS amount,
				a.status AS status
			FROM appointments a
			LEFT JOIN LATERAL (
				SELECT service_name
				FROM appointment_items
				WHERE appointment_id = a.id
				ORDER BY id ASC
				LIMIT 1
			) ai ON true
			WHERE a.pet_id = ? AND a.deleted_at IS NULL

			UNION ALL

			SELECT
				'boarding' AS type,
				b.id AS source_id,
				COALESCE(b.actual_check_out, b.actual_check_in, b.planned_check_out, b.planned_check_in) AS occurred_at,
				'寄养服务' AS title,
				COALESCE(b.total_amount, 0) AS amount,
				b.status AS status
			FROM boarding_orders b
			WHERE b.pet_id = ? AND b.deleted_at IS NULL

			UNION ALL

			SELECT
				'settlement' AS type,
				s.id AS source_id,
				COALESCE(s.paid_at, s.created_at) AS occurred_at,
				COALESCE(NULLIF(si.name, ''), s.code, '结算单') AS title,
				COALESCE(s.paid_amount, s.total_amount, 0) AS amount,
				s.status AS status
			FROM settlements s
			JOIN settlement_items si ON si.settlement_id = s.id
			LEFT JOIN appointments a ON si.source_type = 'appointment' AND si.source_id = a.id
			LEFT JOIN boarding_orders b ON si.source_type IN ('boarding_order', 'boarding') AND si.source_id = b.id
			WHERE a.pet_id = ? OR b.pet_id = ?
		) rows
		ORDER BY occurred_at DESC, source_id DESC
	`, petID, petID, petID, petID).Scan(&list).Error
	return list, err
}
