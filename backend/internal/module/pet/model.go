package pet

import "time"

// Pet mirrors the pets table.
type Pet struct {
	ID         int64      `gorm:"primaryKey" json:"id"`
	CustomerID int64      `json:"customer_id"`
	Name       string     `gorm:"size:64" json:"name"`
	Species    int16      `json:"species"`
	Breed      string     `gorm:"size:64" json:"breed"`
	Gender     int16      `json:"gender"`
	Neutered   bool       `json:"neutered"`
	Birthday   *time.Time `json:"birthday"`
	WeightG    int        `json:"weight_g"`
	Color      string     `gorm:"size:32" json:"color"`
	ChipNo     string     `gorm:"size:40" json:"chip_no"`
	BloodType  string     `gorm:"size:16" json:"blood_type"`
	AvatarText string     `gorm:"size:4" json:"avatar_text"`
	Status     int16      `json:"status"`
	Note       string     `gorm:"type:text" json:"note"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `gorm:"index" json:"-"`
}

func (Pet) TableName() string { return "pets" }

// HealthRecord mirrors pet_health_records.
type HealthRecord struct {
	ID          int64      `gorm:"primaryKey" json:"id"`
	PetID       int64      `json:"pet_id"`
	Type        string     `gorm:"size:16" json:"type"`
	Title       string     `gorm:"size:128" json:"title"`
	PerformedAt *time.Time `json:"performed_at"`
	NextDueAt   *time.Time `json:"next_due_at"`
	OperatorID  *int64     `json:"operator_id"`
	Detail      string     `gorm:"type:text" json:"detail"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (HealthRecord) TableName() string { return "pet_health_records" }

// WeightRecord mirrors pet_weight_records.
type WeightRecord struct {
	ID         int64     `gorm:"primaryKey" json:"id"`
	PetID      int64     `json:"pet_id"`
	WeightG    int       `json:"weight_g"`
	RecordedAt time.Time `json:"recorded_at"`
}

func (WeightRecord) TableName() string { return "pet_weight_records" }

// Species constants
const (
	SpeciesDog = 1
	SpeciesCat = 2
	SpeciesOther = 9
)

// Health record types
const (
	HealthVaccine = "vaccine"
	HealthDeworm  = "deworm"
	HealthExam    = "exam"
	HealthAllergy = "allergy"
	HealthOther   = "other"
)
