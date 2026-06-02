package setting

import "time"

// SystemSetting mirrors system_settings table.
type SystemSetting struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	StoreID   *int64    `json:"store_id"`
	Key       string    `gorm:"uniqueIndex:idx_setting_store_key;size:64" json:"key"`
	Value     string    `gorm:"type:jsonb" json:"value"`
	UpdatedBy *int64    `json:"updated_by"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (SystemSetting) TableName() string { return "system_settings" }
