package appointment

import "time"

// CreateAppointmentRequest is the POST /appointments body.
type CreateAppointmentRequest struct {
	StoreID        int64                   `json:"store_id"`
	CustomerID     int64                   `json:"customer_id"`
	PetID          int64                   `json:"pet_id"`
	Source         int16                   `json:"source"`
	ScheduledStart time.Time               `json:"scheduled_start"`
	ScheduledEnd   time.Time               `json:"scheduled_end"`
	StationID      int64                   `json:"station_id"`
	StaffUserID    int64                   `json:"staff_user_id"`
	ContactName    string                  `json:"contact_name"`
	ContactPhone   string                  `json:"contact_phone"`
	TotalAmount    int64                   `json:"total_amount"`
	Remark         string                  `json:"remark"`
	Items          []CreateAppointmentItem `json:"items"`
	CreatedBy      int64                   `json:"created_by"`
}

// CreateAppointmentItem is an item in the create request.
type CreateAppointmentItem struct {
	ServiceOfferingID int64  `json:"service_offering_id"`
	ServiceName       string `json:"service_name"`
	Price             int64  `json:"price"`
	DurationMin       int    `json:"duration_min"`
	StationID         int64  `json:"station_id"`
}

// TransitionRequest is the POST /appointments/{id}/transitions body.
type TransitionRequest struct {
	Action string `json:"action" binding:"required"`
	Reason string `json:"reason"` // optional, for cancel
}

// ListRequest holds query parameters for listing appointments.
type ListRequest struct {
	Status   string    `form:"status"`
	DateFrom time.Time `form:"date_from" time_format:"2006-01-02"`
	DateTo   time.Time `form:"date_to" time_format:"2006-01-02"`
	Page     int       `form:"page"`
	PageSize int       `form:"page_size"`
}

type WeekScheduleResponse struct {
	StationID int64             `json:"station_id"`
	WeekStart string            `json:"week_start"`
	WeekEnd   string            `json:"week_end"`
	Days      []WeekScheduleDay `json:"days"`
}

type WeekScheduleDay struct {
	Date         string                    `json:"date"`
	Appointments []WeekScheduleAppointment `json:"appointments"`
}

type WeekScheduleAppointment struct {
	ID             int64     `json:"id"`
	Status         string    `json:"status"`
	ScheduledStart time.Time `json:"scheduled_start"`
	ScheduledEnd   time.Time `json:"scheduled_end"`
	CustomerID     *int64    `json:"customer_id"`
	PetID          *int64    `json:"pet_id"`
	ContactName    string    `json:"contact_name"`
	TotalAmount    int64     `json:"total_amount"`
}
