package inventory

import (
	"context"
	"database/sql"
	"time"
)

// AlertType constants.
const (
	AlertTypeLowStock   = "low_stock"
	AlertTypeNearExpiry = "near_expiry"
	AlertTypeExpired    = "expired"
)

// AlertItem represents a single inventory alert.
type AlertItem struct {
	ID         int64   `json:"id"`
	MerchantID int64   `json:"merchant_id"`
	ProductID  int64   `json:"product_id"`
	Name       string  `json:"name"`
	Barcode    string  `json:"barcode"`
	Stock      int     `json:"stock"`
	AlertStock int     `json:"alert_stock"`
	ExpiryDate *string `json:"expiry_date"`
	AlertType  string  `json:"alert_type"`
	DaysLeft   *int    `json:"days_left"`
	Status     string  `json:"status"`
}

// AlertListParams holds filter/pagination for alert queries.
type AlertListParams struct {
	AlertType string
	Page      int
	PageSize  int
}

// AlertListResult wraps alert list with pagination.
type AlertListResult struct {
	Alerts   []AlertItem `json:"alerts"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// GetAlerts returns inventory alerts with filtering and pagination.
// Alert priority: expired > near_expiry > low_stock.
func (s *Service) GetAlerts(ctx context.Context, merchantID int64, params AlertListParams) (*AlertListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}

	// Build WHERE clause with priority: expired > near_expiry > low_stock.
	var whereClause string
	switch params.AlertType {
	case AlertTypeLowStock:
		whereClause = `AND p.alert_stock > 0 AND p.stock < p.alert_stock
			AND (p.expiry_date IS NULL OR p.expiry_date > CURRENT_DATE + INTERVAL '30 days')`
	case AlertTypeNearExpiry:
		whereClause = `AND p.expiry_date IS NOT NULL AND p.expiry_date >= CURRENT_DATE AND p.expiry_date <= CURRENT_DATE + INTERVAL '30 days'`
	case AlertTypeExpired:
		whereClause = `AND p.expiry_date IS NOT NULL AND p.expiry_date < CURRENT_DATE`
	default:
		// "all" — union of all three types with priority (no double-counting).
		whereClause = `AND (
			(p.expiry_date IS NOT NULL AND p.expiry_date < CURRENT_DATE)
			OR (p.expiry_date IS NOT NULL AND p.expiry_date >= CURRENT_DATE AND p.expiry_date <= CURRENT_DATE + INTERVAL '30 days')
			OR (p.alert_stock > 0 AND p.stock < p.alert_stock AND (p.expiry_date IS NULL OR p.expiry_date > CURRENT_DATE + INTERVAL '30 days'))
		)`
	}

	// Count query.
	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM products p
		 WHERE p.deleted_at IS NULL AND p.merchant_id = $1 AND p.status = 'active' `+whereClause,
		merchantID,
	).Scan(&total)
	if err != nil {
		return nil, err
	}

	// Data query.
	offset := (params.Page - 1) * params.PageSize
	rows, err := s.db.QueryContext(ctx,
		`SELECT p.id, p.merchant_id, p.name, p.barcode, p.stock, p.alert_stock,
		        p.expiry_date, p.status,
		        CASE
		            WHEN p.expiry_date IS NOT NULL AND p.expiry_date < CURRENT_DATE THEN 'expired'
		            WHEN p.expiry_date IS NOT NULL AND p.expiry_date >= CURRENT_DATE AND p.expiry_date <= CURRENT_DATE + INTERVAL '30 days' THEN 'near_expiry'
		            WHEN p.alert_stock > 0 AND p.stock < p.alert_stock THEN 'low_stock'
		        END AS alert_type,
		        CASE
		            WHEN p.expiry_date IS NOT NULL THEN (p.expiry_date - CURRENT_DATE)
		        END AS days_left
		 FROM products p
		 WHERE p.deleted_at IS NULL AND p.merchant_id = $1 AND p.status = 'active' `+whereClause+`
		 ORDER BY
		     CASE
		         WHEN p.expiry_date IS NOT NULL AND p.expiry_date < CURRENT_DATE THEN 0
		         WHEN p.expiry_date IS NOT NULL AND p.expiry_date <= CURRENT_DATE + INTERVAL '30 days' THEN 1
		         ELSE 2
		     END,
		     p.expiry_date ASC NULLS LAST,
		     (p.stock * 100 / NULLIF(p.alert_stock, 0)) ASC NULLS LAST
		 LIMIT $2 OFFSET $3`,
		merchantID, params.PageSize, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	alerts := make([]AlertItem, 0)
	for rows.Next() {
		var a AlertItem
		var expiryDate sql.NullString
		var alertType sql.NullString
		var daysLeft sql.NullInt64
		if err := rows.Scan(
			&a.ID, &a.MerchantID, &a.Name, &a.Barcode, &a.Stock, &a.AlertStock,
			&expiryDate, &a.Status, &alertType, &daysLeft,
		); err != nil {
			return nil, err
		}
		a.ProductID = a.ID
		a.AlertType = "low_stock"
		if alertType.Valid {
			a.AlertType = alertType.String
		}
		if daysLeft.Valid {
			d := int(daysLeft.Int64)
			a.DaysLeft = &d
		}
		if expiryDate.Valid {
			val := expiryDate.String
			a.ExpiryDate = &val
			// Parse as date only.
			if t, err := time.Parse("2006-01-02T15:04:05Z", val); err == nil {
				dateStr := t.Format("2006-01-02")
				a.ExpiryDate = &dateStr
			}
		}
		alerts = append(alerts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &AlertListResult{
		Alerts:   alerts,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, nil
}
