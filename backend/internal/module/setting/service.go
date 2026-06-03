package setting

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"

	"gorm.io/gorm"

	"pawprint/backend/internal/pkg/apperr"
)

// Service handles settings business logic.
type Service struct {
	repo Repository
}

var timeOfDayPattern = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

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
	if err := validateSettingValue(key, value); err != nil {
		return err
	}

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

func validateSettingValue(key string, value interface{}) error {
	switch key {
	case "feature.sms_enabled", "feature.wechat_enabled", "feature.online_booking_enabled",
		"inventory.allow_negative", "member.allow_downgrade":
		if _, ok := value.(bool); !ok {
			return apperr.BadRequest(fmt.Sprintf("%s 必须是开关值", key))
		}
	case "appointment.cancel_deadline_hours", "appointment.visit_reminder_hours":
		if _, ok := wholeNumberInRange(value, 0, 720); !ok {
			return apperr.BadRequest(fmt.Sprintf("%s 必须是 0-720 的整数", key))
		}
	case "pet.vaccine_remind_days", "member.churn_days":
		if _, ok := wholeNumberInRange(value, 1, 3650); !ok {
			return apperr.BadRequest(fmt.Sprintf("%s 必须是 1-3650 的整数", key))
		}
	case "store.business_hours":
		return validateBusinessHours(value)
	case "boarding.checkout_rule":
		return validateCheckoutRule(value)
	case "points.rule":
		return validatePointsRule(value)
	default:
		return apperr.BadRequest("不支持的设置项: " + key)
	}
	return nil
}

func validateBusinessHours(value interface{}) error {
	obj, ok := objectMap(value)
	if !ok {
		return apperr.BadRequest("营业时间必须是对象")
	}
	open, ok := obj["open"].(string)
	if !ok || !timeOfDayPattern.MatchString(open) {
		return apperr.BadRequest("开门时间格式必须是 HH:mm")
	}
	closeAt, ok := obj["close"].(string)
	if !ok || !timeOfDayPattern.MatchString(closeAt) {
		return apperr.BadRequest("打烊时间格式必须是 HH:mm")
	}
	if open == closeAt {
		return apperr.BadRequest("开门时间和打烊时间不能相同")
	}
	return nil
}

func validateCheckoutRule(value interface{}) error {
	obj, ok := objectMap(value)
	if !ok {
		return apperr.BadRequest("寄养退房规则必须是对象")
	}
	round, ok := obj["round"].(string)
	if !ok || (round != "ceil" && round != "floor" && round != "round") {
		return apperr.BadRequest("寄养计费取整规则无效")
	}
	if _, ok := wholeNumberInRange(obj["min_nights"], 1, 365); !ok {
		return apperr.BadRequest("最少计费晚数必须是 1-365 的整数")
	}
	if _, ok := obj["apply_member_discount"].(bool); !ok {
		return apperr.BadRequest("寄养会员折扣必须是开关值")
	}
	return nil
}

func validatePointsRule(value interface{}) error {
	obj, ok := objectMap(value)
	if !ok {
		return apperr.BadRequest("积分规则必须是对象")
	}
	perYuan, ok := numberValue(obj["per_yuan"])
	if !ok || perYuan < 0 || perYuan > 100 {
		return apperr.BadRequest("每元积分必须是 0-100 的数字")
	}
	if _, ok := obj["by_tier_rate"].(bool); !ok {
		return apperr.BadRequest("按等级倍率必须是开关值")
	}
	if _, ok := obj["recharge_earn"].(bool); !ok {
		return apperr.BadRequest("充值赠送积分必须是开关值")
	}
	return nil
}

func objectMap(value interface{}) (map[string]interface{}, bool) {
	obj, ok := value.(map[string]interface{})
	return obj, ok && obj != nil
}

func wholeNumberInRange(value interface{}, minValue, maxValue int64) (int64, bool) {
	n, ok := numberValue(value)
	if !ok || math.Trunc(n) != n {
		return 0, false
	}
	intValue := int64(n)
	return intValue, intValue >= minValue && intValue <= maxValue
}

func numberValue(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	default:
		return 0, false
	}
}
