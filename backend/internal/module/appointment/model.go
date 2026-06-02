package appointment

import "time"

// Appointment mirrors the appointments table.
type Appointment struct {
	ID              int64     `gorm:"primaryKey" json:"id"`
	StoreID         int64     `json:"store_id"`
	CustomerID      *int64    `json:"customer_id"`
	PetID           *int64    `json:"pet_id"`
	Source          int16     `json:"source"`
	Status          string    `gorm:"size:16;default:pending" json:"status"`
	ScheduledStart  time.Time `json:"scheduled_start"`
	ScheduledEnd    time.Time `json:"scheduled_end"`
	StationID       *int64    `json:"station_id"`
	StaffUserID     *int64    `json:"staff_user_id"`
	ContactName     string    `gorm:"size:64" json:"contact_name"`
	ContactPhone    string    `gorm:"size:20" json:"contact_phone"`
	TotalAmount     int64     `json:"total_amount"`
	Remark          string    `gorm:"size:255" json:"remark"`
	CancelledReason string    `gorm:"size:255" json:"cancelled_reason"`
	CreatedBy       *int64    `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	DeletedAt       *time.Time `gorm:"index" json:"-"`
}

func (Appointment) TableName() string { return "appointments" }

// AppointmentItem mirrors the appointment_items table.
type AppointmentItem struct {
	ID                 int64  `gorm:"primaryKey" json:"id"`
	AppointmentID      int64  `json:"appointment_id"`
	ServiceOfferingID  int64  `json:"service_offering_id"`
	ServiceName        string `gorm:"size:64" json:"service_name"`
	Price              int64  `json:"price"`
	DurationMin        int    `json:"duration_min"`
	StationID          *int64 `json:"station_id"`
}

func (AppointmentItem) TableName() string { return "appointment_items" }

// Valid statuses
const (
	StatusPending     = "pending"
	StatusArrived     = "arrived"
	StatusInProgress  = "in_progress"
	StatusCompleted   = "completed"
	StatusCancelled   = "cancelled"
	StatusNoShow      = "no_show"
)

// Transition actions
const (
	ActionArrive    = "arrive"
	ActionStart     = "start"
	ActionComplete  = "complete"
	ActionCancel    = "cancel"
	ActionNoShow    = "no_show"
)

// State machine: from status → allowed next actions
var appointmentTransitions = map[string][]string{
	StatusPending:    {ActionArrive, ActionCancel, ActionNoShow},
	StatusArrived:    {ActionStart, ActionCancel},
	StatusInProgress: {ActionComplete},
	StatusCompleted:  {},
	StatusCancelled:  {},
	StatusNoShow:     {},
}

// IsValidTransition checks if a status transition is allowed.
func IsValidTransition(from, action string) bool {
	allowed, ok := appointmentTransitions[from]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == action {
			return true
		}
	}
	return false
}

// targetStatus maps an action to the resulting status.
var actionToStatus = map[string]string{
	ActionArrive:   StatusArrived,
	ActionStart:    StatusInProgress,
	ActionComplete: StatusCompleted,
	ActionCancel:   StatusCancelled,
	ActionNoShow:   StatusNoShow,
}

func targetStatus(action string) string {
	if s, ok := actionToStatus[action]; ok {
		return s
	}
	return ""
}
