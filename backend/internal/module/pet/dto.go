package pet

import "time"

// CreatePetRequest is the POST /pets body.
type CreatePetRequest struct {
	CustomerID int64      `json:"customer_id"`
	Name       string     `json:"name" binding:"required"`
	Species    int16      `json:"species"`
	Breed      string     `json:"breed"`
	Gender     int16      `json:"gender"`
	Neutered   bool       `json:"neutered"`
	Birthday   *time.Time `json:"birthday" time_format:"2006-01-02"`
	WeightG    int        `json:"weight_g"`
	Color      string     `json:"color"`
	ChipNo     string     `json:"chip_no"`
	BloodType  string     `json:"blood_type"`
	Note       string     `json:"note"`
}

// HealthRecordRequest is the POST /pets/:id/health body.
type HealthRecordRequest struct {
	Type        string `json:"type" binding:"required"`
	Title       string `json:"title" binding:"required"`
	PerformedAt string `json:"performed_at"`
	NextDueAt   string `json:"next_due_at"`
	Detail      string `json:"detail"`
	OperatorID  int64  `json:"operator_id"`
}

// WeightRecordRequest is the POST /pets/:id/weights body.
type WeightRecordRequest struct {
	WeightG    int    `json:"weight_g" binding:"required"`
	RecordedAt string `json:"recorded_at"`
}

// PetDetailResponse is the GET /pets/:id response with age and history.
type PetDetailResponse struct {
	Pet           *Pet           `json:"pet"`
	AgeYears      int            `json:"age_years"`
	AgeMonths     int            `json:"age_months"`
	HealthRecords []HealthRecord `json:"health_records"`
	WeightRecords []WeightRecord `json:"weight_records"`
}

// ConsumptionRecord is one pet-related consumption event.
type ConsumptionRecord struct {
	Type       string    `json:"type"`
	SourceID   int64     `json:"source_id"`
	OccurredAt time.Time `json:"occurred_at"`
	Title      string    `json:"title"`
	Amount     int64     `json:"amount"`
	Status     string    `json:"status"`
}
