package merchant

import (
	"context"
	"database/sql"
	"time"
)

// DashboardResponse is the merchant dashboard response.
type DashboardResponse struct {
	TodayRevenue         int64   `json:"today_revenue"`
	TodayOrders          int64   `json:"today_orders"`
	TodayNewMembers      int64   `json:"today_new_members"`
	TodayAppointments    int64   `json:"today_appointments"`
	TodayServiceComplete int64   `json:"today_service_complete"`
	StockWarnings        int64   `json:"stock_warnings"`
	PendingAppointments  int64   `json:"pending_appointments"`
	BirthdayReminders    int64   `json:"birthday_reminders"`
	RevenueTrend         []int64 `json:"revenue_trend"`
	MerchantID           int64   `json:"merchant_id"`
}

// GetDashboard returns the merchant dashboard data.
func (s *Service) GetDashboard(ctx context.Context, merchantID int64) (*DashboardResponse, error) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	resp := &DashboardResponse{
		MerchantID:  merchantID,
		RevenueTrend: make([]int64, 7),
	}

	// New members today for this merchant.
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM platform_users
		 WHERE deleted_at IS NULL AND merchant_id = $1 AND created_at >= $2`,
		merchantID, todayStart,
	).Scan(&resp.TodayNewMembers)

	// Revenue and orders: join orders → products, but currently no merchant link on either table.
	// Return 0 until order/service tables with merchant_id are added.

	// Appointments and services: not yet implemented.

	// 7-day revenue trend: query orders for last 7 days if merchant link exists.
	for i := 0; i < 7; i++ {
		dayStart := time.Date(now.Year(), now.Month(), now.Day()-i, 0, 0, 0, 0, now.Location())
		dayEnd := dayStart.Add(24 * time.Hour)
		var dayRevenue sql.NullInt64
		s.db.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(total_cents), 0) FROM orders
			 WHERE created_at >= $1 AND created_at < $2 AND status = 'completed'`,
			dayStart, dayEnd,
		).Scan(&dayRevenue)
		if dayRevenue.Valid {
			resp.RevenueTrend[6-i] = dayRevenue.Int64
		}
	}

	return resp, nil
}
