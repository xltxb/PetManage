package analytics

import (
	"time"

	"gorm.io/gorm"
)

// Repository defines the data access interface for analytics.
type Repository interface {
	GetRevenueTrend(storeID int64, months int) ([]RevenueTrendPoint, error)
	GetServiceBreakdown(storeID int64, start, end time.Time) ([]ServiceBreakdown, error)
	GetPeakHours(storeID int64, start, end time.Time) ([]PeakHourPoint, error)
	GetRetentionFunnel(storeID int64) ([]RetentionBucket, error)
}

type repo struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repo{db: db}
}

func (r *repo) GetRevenueTrend(storeID int64, months int) ([]RevenueTrendPoint, error) {
	var pts []RevenueTrendPoint
	err := r.db.Table("settlements").
		Select("to_char(paid_at, 'YYYY-MM') as month, COALESCE(SUM(paid_amount), 0) as amount").
		Where("store_id = ? AND status = 'paid' AND paid_at >= now() - (? || ' months')::interval", storeID, months).
		Group("to_char(paid_at, 'YYYY-MM')").
		Order("month ASC").
		Scan(&pts).Error
	return pts, err
}

func (r *repo) GetServiceBreakdown(storeID int64, start, end time.Time) ([]ServiceBreakdown, error) {
	var breakdown []ServiceBreakdown
	err := r.db.Table("settlement_items si").
		Select("COALESCE(sc.name, '其他') as category_name, COALESCE(SUM(si.amount), 0) as amount").
		Joins("JOIN settlements s ON s.id = si.settlement_id AND s.status = 'paid'").
		Joins("LEFT JOIN service_offerings so ON so.id = si.source_id AND si.source_type = 'appointment'").
		Joins("LEFT JOIN services sv ON sv.id = so.service_id").
		Joins("LEFT JOIN service_categories sc ON sc.id = sv.category_id").
		Where("s.store_id = ? AND s.paid_at >= ? AND s.paid_at < ?", storeID, start, end).
		Group("sc.name").
		Order("amount DESC").
		Scan(&breakdown).Error
	// Calculate percentages
	var total int64
	for _, b := range breakdown { total += b.Amount }
	for i := range breakdown {
		if total > 0 {
			breakdown[i].Percentage = float64(breakdown[i].Amount) / float64(total) * 100
		}
	}
	return breakdown, err
}

func (r *repo) GetPeakHours(storeID int64, start, end time.Time) ([]PeakHourPoint, error) {
	var pts []PeakHourPoint
	err := r.db.Table("settlements").
		Select("EXTRACT(HOUR FROM paid_at)::int as hour, COUNT(*) as count").
		Where("store_id = ? AND status = 'paid' AND paid_at >= ? AND paid_at < ?", storeID, start, end).
		Group("EXTRACT(HOUR FROM paid_at)").
		Order("hour ASC").
		Scan(&pts).Error
	return pts, err
}

func (r *repo) GetRetentionFunnel(storeID int64) ([]RetentionBucket, error) {
	var buckets []RetentionBucket
	err := r.db.Raw(`
		SELECT
			CASE
				WHEN visit_count = 1 THEN '1次'
				WHEN visit_count BETWEEN 2 AND 3 THEN '2-3次'
				WHEN visit_count BETWEEN 4 AND 6 THEN '4-6次'
				ELSE '7次+'
			END as bucket,
			COUNT(*) as count
		FROM (
			SELECT c.id, COUNT(s.id) as visit_count
			FROM customers c
			JOIN settlements s ON s.customer_id = c.id
				AND s.status = 'paid' AND s.store_id = ?
			WHERE c.deleted_at IS NULL
			GROUP BY c.id
		) sub
		GROUP BY bucket
		ORDER BY MIN(visit_count) ASC
	`, storeID).Scan(&buckets).Error
	return buckets, err
}
