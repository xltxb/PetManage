package setting

import (
	"errors"
	"testing"

	"pawprint/backend/internal/pkg/apperr"
)

type fakeSettingRepo struct {
	upserted *SystemSetting
}

func (f *fakeSettingRepo) GetAll(storeID int64) ([]SystemSetting, error) {
	return nil, nil
}

func (f *fakeSettingRepo) Get(storeID int64, key string) (*SystemSetting, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeSettingRepo) Upsert(s *SystemSetting) error {
	copy := *s
	f.upserted = &copy
	return nil
}

func TestSetRejectsUnsupportedSettingKey(t *testing.T) {
	repo := &fakeSettingRepo{}
	svc := NewService(repo)

	err := svc.Set(1, "feature.raw_json_escape_hatch", true, 9)

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("Set error = %v, want AppError", err)
	}
	if repo.upserted != nil {
		t.Fatalf("unsupported key should not be persisted")
	}
}

func TestSetValidatesSettingValueShape(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value interface{}
	}{
		{name: "boolean flag requires bool", key: "feature.sms_enabled", value: "true"},
		{name: "business hours require open", key: "store.business_hours", value: map[string]interface{}{"close": "21:00"}},
		{name: "business hours require valid time", key: "store.business_hours", value: map[string]interface{}{"open": "24:00", "close": "21:00"}},
		{name: "checkout round enum", key: "boarding.checkout_rule", value: map[string]interface{}{"round": "bankers", "min_nights": float64(1), "apply_member_discount": false}},
		{name: "checkout min nights positive", key: "boarding.checkout_rule", value: map[string]interface{}{"round": "ceil", "min_nights": float64(0), "apply_member_discount": false}},
		{name: "positive integer setting", key: "appointment.cancel_deadline_hours", value: float64(-1)},
		{name: "points rule per yuan non negative", key: "points.rule", value: map[string]interface{}{"per_yuan": float64(-1), "by_tier_rate": true, "recharge_earn": false}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeSettingRepo{}
			svc := NewService(repo)

			err := svc.Set(1, tt.key, tt.value, 9)

			var appErr *apperr.AppError
			if !errors.As(err, &appErr) {
				t.Fatalf("Set error = %v, want AppError", err)
			}
			if repo.upserted != nil {
				t.Fatalf("invalid value should not be persisted")
			}
		})
	}
}

func TestSetPersistsSupportedSetting(t *testing.T) {
	repo := &fakeSettingRepo{}
	svc := NewService(repo)

	err := svc.Set(1, "store.business_hours", map[string]interface{}{"open": "09:00", "close": "21:00"}, 9)
	if err != nil {
		t.Fatalf("Set error = %v", err)
	}
	if repo.upserted == nil {
		t.Fatalf("supported setting was not persisted")
	}
	if repo.upserted.Value != `{"close":"21:00","open":"09:00"}` {
		t.Fatalf("Value = %s", repo.upserted.Value)
	}
}
