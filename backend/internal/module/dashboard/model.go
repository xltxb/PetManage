package dashboard

import "time"

// DashboardSummary is the response for GET /api/v1/dashboard/summary.
type DashboardSummary struct {
	RevenueToday      int64                    `json:"revenue_today"`
	AppointmentCount  int64                    `json:"appointment_count"`
	PetsInStore       int64                    `json:"pets_in_store"`
	NewMembersCount   int64                    `json:"new_members_count"`
	RevenueTrend      []RevenuePoint           `json:"revenue_trend"`
	TodayAppointments []AppointmentTimelineItem `json:"today_appointments"`
	PopularServices   []ServiceRank            `json:"popular_services"`
	InventoryAlerts   []InventoryAlert         `json:"inventory_alerts"`
	MemberComposition []TierComposition        `json:"member_composition"`
}

// RevenuePoint is a single day's revenue for the trend chart.
type RevenuePoint struct {
	Date   string `json:"date"`
	Amount int64  `json:"amount"`
}

// AppointmentTimelineItem is a today's appointment entry for the timeline.
type AppointmentTimelineItem struct {
	ID             int64     `json:"id"`
	PetName        string    `json:"pet_name"`
	CustomerName   string    `json:"customer_name"`
	ServiceName    string    `json:"service_name"`
	ScheduledStart time.Time `json:"scheduled_start"`
	ScheduledEnd   time.Time `json:"scheduled_end"`
	Status         string    `json:"status"`
	StationName    string    `json:"station_name"`
}

// ServiceRank shows a service and its booking count this month.
type ServiceRank struct {
	ServiceName string `json:"service_name"`
	Count       int64  `json:"count"`
}

// InventoryAlert is a product below or at safety stock.
type InventoryAlert struct {
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name"`
	Quantity    int    `json:"quantity"`
	SafetyStock int    `json:"safety_stock"`
	Unit        string `json:"unit"`
}

// TierComposition is a count of members at a given tier.
type TierComposition struct {
	TierName string `json:"tier_name"`
	Count    int64  `json:"count"`
	Color    string `json:"color,omitempty"`
}
