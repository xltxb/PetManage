package setting

import "gorm.io/gorm"

// Repository defines the data access interface for settings.
type Repository interface {
	GetAll(storeID int64) ([]SystemSetting, error)
	Get(storeID int64, key string) (*SystemSetting, error)
	Upsert(s *SystemSetting) error
}

type repo struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repo{db: db}
}

func (r *repo) GetAll(storeID int64) ([]SystemSetting, error) {
	var settings []SystemSetting
	err := r.db.Where("store_id = ? OR store_id IS NULL", storeID).
		Order("key ASC, CASE WHEN store_id IS NULL THEN 0 ELSE 1 END ASC").Find(&settings).Error
	return settings, err
}

func (r *repo) Get(storeID int64, key string) (*SystemSetting, error) {
	var s SystemSetting
	err := r.db.Where("key = ? AND (store_id = ? OR store_id IS NULL)", key, storeID).
		First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *repo) Upsert(s *SystemSetting) error {
	query := r.db.Where("key = ?", s.Key)
	if s.StoreID == nil {
		query = query.Where("store_id IS NULL")
	} else {
		query = query.Where("store_id = ?", *s.StoreID)
	}

	return query.Assign(map[string]interface{}{
		"value":      s.Value,
		"updated_by": s.UpdatedBy,
		"updated_at": gorm.Expr("now()"),
	}).FirstOrCreate(s).Error
}
