package setting

import (
	"encoding/json"

	"gorm.io/gorm"

	"pawprint/backend/internal/pkg/apperr"
)

// Service handles settings business logic.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// GetAll returns all settings for a store (including global ones).
func (s *Service) GetAll(storeID int64) (map[string]interface{}, error) {
	settings, err := s.repo.GetAll(storeID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	result := make(map[string]interface{})
	for _, setting := range settings {
		var v interface{}
		if err := json.Unmarshal([]byte(setting.Value), &v); err == nil {
			result[setting.Key] = v
		} else {
			result[setting.Key] = setting.Value
		}
	}
	return result, nil
}

// Get returns a specific setting value.
func (s *Service) Get(storeID int64, key string) (interface{}, error) {
	setting, err := s.repo.Get(storeID, key)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperr.NotFound("设置项不存在: " + key)
		}
		return nil, apperr.Internal(err)
	}
	var v interface{}
	json.Unmarshal([]byte(setting.Value), &v)
	return v, nil
}

// Set upserts a setting value (JSON-encoded).
func (s *Service) Set(storeID int64, key string, value interface{}, updatedBy int64) error {
	valJSON, err := json.Marshal(value)
	if err != nil {
		return apperr.BadRequest("无效的设置值")
	}

	var storeIDPtr *int64
	if storeID > 0 {
		storeIDPtr = &storeID
	}
	var updatedByPtr *int64
	if updatedBy > 0 {
		updatedByPtr = &updatedBy
	}

	setting := &SystemSetting{
		StoreID:   storeIDPtr,
		Key:       key,
		Value:     string(valJSON),
		UpdatedBy: updatedByPtr,
	}
	return s.repo.Upsert(setting)
}
