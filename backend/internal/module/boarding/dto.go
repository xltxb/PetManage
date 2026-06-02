package boarding

import "time"

// CheckInRequest is the POST /boarding-orders/check-in body.
type CheckInRequest struct {
	StoreID         int64     `json:"store_id"`
	CustomerID      int64     `json:"customer_id"`
	PetID           int64     `json:"pet_id"`
	RoomID          int64     `json:"room_id"`
	RoomTypeCode    string    `json:"room_type_code"`
	PricePerNight   int64     `json:"price_per_night"`
	PlannedCheckIn  time.Time `json:"planned_check_in"`
	PlannedCheckOut time.Time `json:"planned_check_out"`
	Source          int16     `json:"source"`
	Remark          string    `json:"remark"`
}

// CheckOutResponse is returned after check-out with billing info.
type CheckOutResponse struct {
	Order       *BoardingOrder `json:"order"`
	Nights      int            `json:"nights"`
	TotalAmount int64          `json:"total_amount"`
}

// CareLogRequest is the POST /care-logs body.
type CareLogRequest struct {
	Task       string `json:"task" binding:"required"`
	Status     string `json:"status" binding:"required"`
	Note       string `json:"note"`
	PhotoURL   string `json:"photo_url"`
	OperatorID int64  `json:"operator_id"`
}
