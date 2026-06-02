package boarding

import "time"

// BoardingRoom mirrors the boarding_rooms table.
type BoardingRoom struct {
	ID         int64  `gorm:"primaryKey" json:"id"`
	StoreID    int64  `json:"store_id"`
	RoomTypeID int64  `json:"room_type_id"`
	Code       string `gorm:"size:16" json:"code"`
	Status     string `gorm:"size:16;default:free" json:"status"`
	Sort       int16  `json:"sort"`
}

func (BoardingRoom) TableName() string { return "boarding_rooms" }

// BoardingOrder mirrors the boarding_orders table.
type BoardingOrder struct {
	ID               int64      `gorm:"primaryKey" json:"id"`
	StoreID          int64      `json:"store_id"`
	CustomerID       int64      `json:"customer_id"`
	PetID            int64      `json:"pet_id"`
	RoomID           *int64     `json:"room_id"`
	RoomTypeSnapshot string     `gorm:"size:32" json:"room_type_snapshot"`
	PricePerNight    int64      `json:"price_per_night"`
	Status           string     `gorm:"size:16;default:booked" json:"status"`
	Source           int16      `json:"source"`
	PlannedCheckIn   time.Time  `json:"planned_check_in"`
	PlannedCheckOut  time.Time  `json:"planned_check_out"`
	ActualCheckIn    *time.Time `json:"actual_check_in"`
	ActualCheckOut   *time.Time `json:"actual_check_out"`
	Nights           *int       `json:"nights"`
	TotalAmount      *int64     `json:"total_amount"`
	SettlementID     *int64     `json:"settlement_id"`
	Remark           string     `gorm:"type:text" json:"remark"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `gorm:"index" json:"-"`
}

func (BoardingOrder) TableName() string { return "boarding_orders" }

// CareLog mirrors the boarding_care_logs table.
type CareLog struct {
	ID               int64      `gorm:"primaryKey" json:"id"`
	BoardingOrderID  int64      `json:"boarding_order_id"`
	StoreID          int64      `json:"store_id"`
	Task             string     `gorm:"size:16" json:"task"`
	Status           string     `gorm:"size:8;default:pending" json:"status"`
	DoneAt           *time.Time `json:"done_at"`
	OperatorID       *int64     `json:"operator_id"`
	Note             string     `gorm:"size:255" json:"note"`
	PhotoURL         string     `gorm:"size:255" json:"photo_url"`
	LogDate          time.Time  `json:"log_date"`
	CreatedAt        time.Time  `json:"created_at"`
}

func (CareLog) TableName() string { return "boarding_care_logs" }

// Order statuses
const (
	StatusBooked    = "booked"
	StatusCheckedIn = "checked_in"
	StatusCheckedOut = "checked_out"
	StatusCancelled = "cancelled"
)

// Room statuses
const (
	RoomStatusFree        = "free"
	RoomStatusOccupied    = "occupied"
	RoomStatusCleaning    = "cleaning"
	RoomStatusMaintenance = "maintenance"
)

// Care tasks
const (
	TaskFeeding    = "feeding"
	TaskWalking    = "walking"
	TaskMedication = "medication"
	TaskPhoto      = "photo"
)

// Care log statuses
const (
	CareStatusDone    = "done"
	CareStatusPending = "pending"
)

// Room state machine
var roomTransitions = map[string][]string{
	RoomStatusFree:        {RoomStatusOccupied, RoomStatusMaintenance},
	RoomStatusOccupied:    {RoomStatusCleaning},
	RoomStatusCleaning:    {RoomStatusFree, RoomStatusMaintenance},
	RoomStatusMaintenance: {RoomStatusFree},
}

// IsValidRoomTransition checks if a room status transition is allowed.
func IsValidRoomTransition(from, to string) bool {
	allowed, ok := roomTransitions[from]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == to {
			return true
		}
	}
	return false
}
