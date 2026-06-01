package dashboard

import (
	"time"

	"gorm.io/gorm"

	"pawprint/backend/internal/pkg/timeutil"
)

// Repository defines the data access interface for dashboard.
type Repository interface {
	GetRevenueToday(storeID int64, tz string) (int64, error)
	GetAppointmentCountToday(storeID int64, tz string) (int64, error)
	GetPetsInStore(storeID int64, tz string) (int64, error)
	GetNewMembersToday(storeID int64, tz string) (int64, error)
	GetRevenueTrend(storeID int64, days int, tz string) ([]RevenuePoint, error)
	GetTodayAppointments(storeID int64, tz string) ([]AppointmentTimelineItem, error)
	GetPopularServices(storeID int64, month time.Time) ([]ServiceRank, error)
	GetInventoryAlerts(storeID int64) ([]InventoryAlert, error)
	GetMemberComposition(storeID int64) ([]TierComposition, error)
}

type repo struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repo{db: db}
}

func (r *repo) GetRevenueToday(storeID int64, tz string) (int64, error) {
	start := timeutil.StartOfDay(time.Now().UTC(), tz)
	end := timeutil.EndOfDay(time.Now().UTC(), tz)
	var amount int64
	err := r.db.Table("settlements").
		Select("COALESCE(SUM(paid_amount), 0)").
		Where("store_id = ? AND status = 'paid' AND paid_at >= ? AND paid_at < ?", storeID, start, end).
		Scan(&amount).Error
	return amount, err
}

func (r *repo) GetAppointmentCountToday(storeID int64, tz string) (int64, error) {
	start := timeutil.StartOfDay(time.Now().UTC(), tz)
	end := timeutil.EndOfDay(time.Now().UTC(), tz)
	var count int64
	err := r.db.Table("appointments").
		Where("store_id = ? AND scheduled_start >= ? AND scheduled_start < ? AND deleted_at IS NULL", storeID, start, end).
		Count(&count).Error
	return count, err
}

func (r *repo) GetPetsInStore(storeID int64, tz string) (int64, error) {
	now := time.Now().UTC()
	// Boarding: checked_in pets
	var boardingCount int64
	r.db.Table("boarding_orders").
		Where("store_id = ? AND status = 'checked_in' AND deleted_at IS NULL", storeID).
		Count(&boardingCount)

	// Appointments: arrived or in_progress today
	start := timeutil.StartOfDay(now, tz)
	end := timeutil.EndOfDay(now, tz)
	var apptCount int64
	r.db.Table("appointments").
		Where("store_id = ? AND status IN ('arrived','in_progress') AND scheduled_start >= ? AND scheduled_start < ? AND deleted_at IS NULL",
			storeID, start, end).
		Count(&apptCount)

	// Sum of distinct pets: boarding pets + today's appointment pets
	// Use a raw query to count distinct pet_ids across both sources
	var total int64
	err := r.db.Raw(`
		SELECT COUNT(DISTINCT pet_id) FROM (
			SELECT pet_id FROM boarding_orders
			WHERE store_id = ? AND status = 'checked_in' AND deleted_at IS NULL
			UNION
			SELECT pet_id FROM appointments
			WHERE store_id = ? AND status IN ('arrived','in_progress')
			  AND scheduled_start >= ? AND scheduled_start < ?
			  AND deleted_at IS NULL AND pet_id IS NOT NULL
		) AS pets
	`, storeID, storeID, start, end).Scan(&total).Error
	_ = boardingCount + apptCount
	return total, err
}

func (r *repo) GetNewMembersToday(storeID int64, tz string) (int64, error) {
	start := timeutil.StartOfDay(time.Now().UTC(), tz)
	end := timeutil.EndOfDay(time.Now().UTC(), tz)
	var count int64
	err := r.db.Table("customers").
		Where("register_store_id = ? AND created_at >= ? AND created_at < ? AND deleted_at IS NULL", storeID, start, end).
		Count(&count).Error
	return count, err
}

func (r *repo) GetRevenueTrend(storeID int64, days int, tz string) ([]RevenuePoint, error) {
	start := timeutil.StartOfDay(time.Now().UTC().AddDate(0, 0, -days+1), tz)
	end := timeutil.EndOfDay(time.Now().UTC(), tz)

	var points []RevenuePoint
	err := r.db.Table("settlements").
		Select("to_char(paid_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD') as date, COALESCE(SUM(paid_amount), 0) as amount").
		Where("store_id = ? AND status = 'paid' AND paid_at >= ? AND paid_at < ?", storeID, start, end).
		Group("to_char(paid_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD')").
		Order("date ASC").
		Scan(&points).Error
	return points, err
}

func (r *repo) GetTodayAppointments(storeID int64, tz string) ([]AppointmentTimelineItem, error) {
	start := timeutil.StartOfDay(time.Now().UTC(), tz)
	end := timeutil.EndOfDay(time.Now().UTC(), tz)

	var items []AppointmentTimelineItem
	err := r.db.Table("appointments a").
		Select("a.id, COALESCE(p.name, '') as pet_name, COALESCE(cu.name, a.contact_name, '散客') as customer_name, ai.service_name, a.scheduled_start, a.scheduled_end, a.status, COALESCE(s.name, '') as station_name").
		Joins("LEFT JOIN pets p ON p.id = a.pet_id").
		Joins("LEFT JOIN customers cu ON cu.id = a.customer_id").
		Joins("LEFT JOIN appointment_items ai ON ai.appointment_id = a.id").
		Joins("LEFT JOIN stations s ON s.id = a.station_id").
		Where("a.store_id = ? AND a.scheduled_start >= ? AND a.scheduled_start < ? AND a.deleted_at IS NULL", storeID, start, end).
		Order("a.scheduled_start ASC").
		Scan(&items).Error
	return items, err
}

func (r *repo) GetPopularServices(storeID int64, month time.Time) ([]ServiceRank, error) {
	var ranks []ServiceRank
	err := r.db.Table("appointment_items ai").
		Select("ai.service_name, COUNT(*) as count").
		Joins("JOIN appointments a ON a.id = ai.appointment_id").
		Where("a.store_id = ? AND a.deleted_at IS NULL AND date_trunc('month', a.scheduled_start) = date_trunc('month', ?::timestamptz)", storeID, month).
		Group("ai.service_name").
		Order("count DESC").
		Limit(10).
		Scan(&ranks).Error
	return ranks, err
}

func (r *repo) GetInventoryAlerts(storeID int64) ([]InventoryAlert, error) {
	var alerts []InventoryAlert
	err := r.db.Table("inventory i").
		Select("i.product_id, p.name as product_name, i.quantity, i.safety_stock, COALESCE(p.unit, '') as unit").
		Joins("JOIN products p ON p.id = i.product_id AND p.deleted_at IS NULL").
		Where("i.store_id = ? AND i.quantity <= i.safety_stock AND i.safety_stock > 0", storeID).
		Order("(i.quantity::float / NULLIF(i.safety_stock, 0)) ASC").
		Scan(&alerts).Error
	return alerts, err
}

func (r *repo) GetMemberComposition(storeID int64) ([]TierComposition, error) {
	var comps []TierComposition
	err := r.db.Table("customers c").
		Select("mt.name as tier_name, COUNT(*) as count").
		Joins("JOIN membership_tiers mt ON mt.id = c.tier_id").
		Where("c.deleted_at IS NULL").
		Group("mt.name, mt.sort").
		Order("mt.sort ASC").
		Scan(&comps).Error
	return comps, err
}
