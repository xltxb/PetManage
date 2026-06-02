package wx

import "time"

type LoginRequest struct {
	Code    string `json:"code" binding:"required"`
	StoreID int64  `json:"store_id" binding:"required"`
}

type LoginResponse struct {
	CustomerID int64  `json:"customer_id"`
	Token      string `json:"token"`
}

type ServiceOffering struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Price       int64  `json:"price"`
	DurationMin int    `json:"duration_min"`
}

type CreateAppointmentRequest struct {
	StoreID           int64     `json:"store_id" binding:"required"`
	PetID             int64     `json:"pet_id" binding:"required"`
	ServiceOfferingID int64     `json:"service_offering_id" binding:"required"`
	ScheduledStart    time.Time `json:"scheduled_start" binding:"required"`
}
