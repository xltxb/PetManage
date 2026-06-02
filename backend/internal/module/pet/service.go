package pet

import (
	"time"

	"gorm.io/gorm"

	"pawprint/backend/internal/pkg/apperr"
)

// Service handles pet business logic.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CalculateAge computes age in years and months from a birthday relative to a reference date.
func CalculateAge(birthday, reference time.Time) (years int, months int) {
	if birthday.IsZero() {
		return 0, 0
	}
	y, m, d := birthday.Date()
	ry, rm, rd := reference.Date()
	years = ry - y
	months = int(rm) - int(m)
	if rd < d {
		months--
	}
	if months < 0 {
		years--
		months += 12
	}
	if years < 0 {
		years = 0
		months = 0
	}
	return
}

// Create creates a new pet.
func (s *Service) Create(req CreatePetRequest) (*Pet, error) {
	p := &Pet{
		CustomerID: req.CustomerID,
		Name:       req.Name,
		Species:    req.Species,
		Breed:      req.Breed,
		Gender:     req.Gender,
		Neutered:   req.Neutered,
		Birthday:   req.Birthday,
		WeightG:    req.WeightG,
		Color:      req.Color,
		ChipNo:     req.ChipNo,
		BloodType:  req.BloodType,
		Note:       req.Note,
		Status:     1,
	}
	if p.AvatarText == "" && len(req.Name) > 0 {
		p.AvatarText = string([]rune(req.Name)[0])
	}
	if err := s.repo.Create(p); err != nil {
		return nil, apperr.Internal(err)
	}
	return p, nil
}

// GetDetail returns a pet with age, health records, and weight records.
func (s *Service) GetDetail(id int64) (*PetDetailResponse, error) {
	p, err := s.repo.FindByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperr.NotFound("宠物不存在")
		}
		return nil, apperr.Internal(err)
	}

	var birthday time.Time
	if p.Birthday != nil {
		birthday = *p.Birthday
	}
	years, months := CalculateAge(birthday, time.Now())

	health, _ := s.repo.FindHealthRecords(id)
	weights, _ := s.repo.FindWeightRecords(id)
	if health == nil { health = []HealthRecord{} }
	if weights == nil { weights = []WeightRecord{} }

	return &PetDetailResponse{
		Pet:           p,
		AgeYears:      years,
		AgeMonths:     months,
		HealthRecords: health,
		WeightRecords: weights,
	}, nil
}

// AddHealthRecord adds a health record to a pet.
func (s *Service) AddHealthRecord(petID int64, req HealthRecordRequest) error {
	_, err := s.repo.FindByID(petID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return apperr.NotFound("宠物不存在")
		}
		return apperr.Internal(err)
	}

	hr := &HealthRecord{PetID: petID, Type: req.Type, Title: req.Title, Detail: req.Detail}

	if req.PerformedAt != "" {
		t, err := time.Parse("2006-01-02", req.PerformedAt)
		if err == nil { hr.PerformedAt = &t }
	}
	if req.NextDueAt != "" {
		t, err := time.Parse("2006-01-02", req.NextDueAt)
		if err == nil { hr.NextDueAt = &t }
	}
	if req.OperatorID > 0 {
		opID := req.OperatorID
		hr.OperatorID = &opID
	}

	return s.repo.CreateHealthRecord(hr)
}

// AddWeightRecord records a new weight measurement and updates the pet's latest weight.
func (s *Service) AddWeightRecord(petID int64, weightG int) error {
	p, err := s.repo.FindByID(petID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return apperr.NotFound("宠物不存在")
		}
		return apperr.Internal(err)
	}

	wr := &WeightRecord{PetID: petID, WeightG: weightG, RecordedAt: time.Now()}
	if err := s.repo.CreateWeightRecord(wr); err != nil {
		return apperr.Internal(err)
	}

	p.WeightG = weightG
	return s.repo.Update(p)
}

// ListByCustomer returns all pets for a customer.
func (s *Service) ListByCustomer(customerID int64) ([]Pet, error) {
	return s.repo.ListByCustomer(customerID)
}
