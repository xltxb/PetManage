package analytics

// RevenueTrendPoint is a single month's revenue.
type RevenueTrendPoint struct {
	Month  string `json:"month"`
	Amount int64  `json:"amount"`
}

// ServiceBreakdown shows revenue share by service category.
type ServiceBreakdown struct {
	CategoryName string  `json:"category_name"`
	Amount       int64   `json:"amount"`
	Percentage   float64 `json:"percentage"`
	Color        string  `json:"color,omitempty"`
}

// PeakHourPoint is a count of settlements in a given hour.
type PeakHourPoint struct {
	Hour  int   `json:"hour"`
	Count int64 `json:"count"`
}

// RetentionBucket is a visit-frequency group.
type RetentionBucket struct {
	Bucket string `json:"bucket"`
	Count  int64  `json:"count"`
}

// AnalyticsReport is the combined analytics response.
type AnalyticsReport struct {
	RevenueTrend     []RevenueTrendPoint  `json:"revenue_trend"`
	ServiceBreakdown []ServiceBreakdown   `json:"service_breakdown"`
	PeakHours        []PeakHourPoint       `json:"peak_hours"`
	RetentionFunnel  []RetentionBucket     `json:"retention_funnel"`
}
