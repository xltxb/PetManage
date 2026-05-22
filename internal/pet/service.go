package pet

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// VaccineRecord represents a single vaccine record.
type VaccineRecord struct {
	VaccineName string `json:"vaccine_name"`
	Date        string `json:"date"`
	NextDate    string `json:"next_date,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

// DewormingRecord represents a single deworming record.
type DewormingRecord struct {
	MedicineName string `json:"medicine_name"`
	Date         string `json:"date"`
	NextDate     string `json:"next_date,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

// Pet represents a pet record.
type Pet struct {
	ID               int64            `json:"id"`
	MerchantID       int64            `json:"merchant_id"`
	MemberID         int64            `json:"member_id"`
	Name             string           `json:"name"`
	Breed            string           `json:"breed"`
	Gender           string           `json:"gender"`
	Age              int              `json:"age"`
	Weight           string           `json:"weight"`
	VaccineRecords   []VaccineRecord  `json:"vaccine_records"`
	DewormingRecords []DewormingRecord `json:"deworming_records"`
	AllergyHistory   string           `json:"allergy_history"`
	Notes            string           `json:"notes"`
	Status           string           `json:"status"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

// CreatePetRequest is the request body for creating a pet.
type CreatePetRequest struct {
	Name           string           `json:"name"`
	Breed          string           `json:"breed"`
	Gender         string           `json:"gender"`
	Age            int              `json:"age"`
	Weight         string           `json:"weight"`
	VaccineRecords   []VaccineRecord  `json:"vaccine_records"`
	DewormingRecords []DewormingRecord `json:"deworming_records"`
	AllergyHistory string           `json:"allergy_history"`
	Notes          string           `json:"notes"`
}

// UpdatePetRequest is the request body for updating a pet (partial).
type UpdatePetRequest struct {
	Name             *string           `json:"name"`
	Breed            *string           `json:"breed"`
	Gender           *string           `json:"gender"`
	Age              *int              `json:"age"`
	Weight           *string           `json:"weight"`
	VaccineRecords   *[]VaccineRecord  `json:"vaccine_records"`
	DewormingRecords *[]DewormingRecord `json:"deworming_records"`
	AllergyHistory   *string           `json:"allergy_history"`
	Notes            *string           `json:"notes"`
}

// Service provides pet management operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new pet Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

const petColumns = `id, merchant_id, member_id, name, breed, gender, age, weight, vaccine_records, deworming_records, allergy_history, notes, status, created_at, updated_at`

func scanPet(row interface{ Scan(...interface{}) error }) (*Pet, error) {
	p := &Pet{}
	var vJSON, dJSON []byte
	err := row.Scan(
		&p.ID, &p.MerchantID, &p.MemberID, &p.Name, &p.Breed, &p.Gender,
		&p.Age, &p.Weight, &vJSON, &dJSON, &p.AllergyHistory,
		&p.Notes, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if vJSON != nil {
		json.Unmarshal(vJSON, &p.VaccineRecords)
	}
	if p.VaccineRecords == nil {
		p.VaccineRecords = make([]VaccineRecord, 0)
	}
	if dJSON != nil {
		json.Unmarshal(dJSON, &p.DewormingRecords)
	}
	if p.DewormingRecords == nil {
		p.DewormingRecords = make([]DewormingRecord, 0)
	}
	return p, nil
}

// Create creates a new pet for a member.
func (s *Service) Create(ctx context.Context, merchantID, memberID int64, req CreatePetRequest) (*Pet, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperrors.NewValidationError("pet name is required")
	}
	if req.Gender != "" && req.Gender != "M" && req.Gender != "F" {
		return nil, apperrors.NewValidationError("gender must be M or F")
	}

	vJSON, _ := json.Marshal(coalesceVaccineRecords(req.VaccineRecords))
	dJSON, _ := json.Marshal(coalesceDewormingRecords(req.DewormingRecords))

	p, err := scanPet(s.db.QueryRowContext(ctx,
		`INSERT INTO pets (merchant_id, member_id, name, breed, gender, age, weight, vaccine_records, deworming_records, allergy_history, notes)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING `+petColumns,
		merchantID, memberID, strings.TrimSpace(req.Name), req.Breed, req.Gender,
		req.Age, req.Weight, vJSON, dJSON, req.AllergyHistory, req.Notes,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create pet", err)
	}
	return p, nil
}

// ListByMember returns all pets for a given member, scoped to a merchant.
func (s *Service) ListByMember(ctx context.Context, merchantID, memberID int64) ([]Pet, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+petColumns+` FROM pets
		 WHERE merchant_id = $1 AND member_id = $2 AND deleted_at IS NULL
		 ORDER BY created_at DESC`,
		merchantID, memberID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list pets", err)
	}
	defer rows.Close()

	pets := make([]Pet, 0)
	for rows.Next() {
		p, err := scanPet(rows)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to scan pet", err)
		}
		pets = append(pets, *p)
	}
	if pets == nil {
		pets = make([]Pet, 0)
	}
	return pets, nil
}

// GetByID returns a single pet, scoped to merchant and member.
func (s *Service) GetByID(ctx context.Context, petID, memberID, merchantID int64) (*Pet, error) {
	p, err := scanPet(s.db.QueryRowContext(ctx,
		`SELECT `+petColumns+` FROM pets
		 WHERE id = $1 AND member_id = $2 AND merchant_id = $3 AND deleted_at IS NULL`,
		petID, memberID, merchantID,
	))
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("pet not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get pet", err)
	}
	return p, nil
}

// Update updates pet fields. Only non-nil fields in the request are applied.
func (s *Service) Update(ctx context.Context, petID, memberID, merchantID int64, req UpdatePetRequest) (*Pet, error) {
	existing, err := s.GetByID(ctx, petID, memberID, merchantID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			return nil, apperrors.NewValidationError("pet name is required")
		}
		existing.Name = strings.TrimSpace(*req.Name)
	}
	if req.Breed != nil {
		existing.Breed = *req.Breed
	}
	if req.Gender != nil {
		if *req.Gender != "" && *req.Gender != "M" && *req.Gender != "F" {
			return nil, apperrors.NewValidationError("gender must be M or F")
		}
		existing.Gender = *req.Gender
	}
	if req.Age != nil {
		existing.Age = *req.Age
	}
	if req.Weight != nil {
		existing.Weight = *req.Weight
	}
	if req.VaccineRecords != nil {
		existing.VaccineRecords = coalesceVaccineRecords(*req.VaccineRecords)
	}
	if req.DewormingRecords != nil {
		existing.DewormingRecords = coalesceDewormingRecords(*req.DewormingRecords)
	}
	if req.AllergyHistory != nil {
		existing.AllergyHistory = *req.AllergyHistory
	}
	if req.Notes != nil {
		existing.Notes = *req.Notes
	}

	vJSON, _ := json.Marshal(existing.VaccineRecords)
	dJSON, _ := json.Marshal(existing.DewormingRecords)

	p, err := scanPet(s.db.QueryRowContext(ctx,
		`UPDATE pets SET name=$1, breed=$2, gender=$3, age=$4, weight=$5, vaccine_records=$6, deworming_records=$7, allergy_history=$8, notes=$9, updated_at=NOW()
		 WHERE id=$10 AND member_id=$11 AND merchant_id=$12 AND deleted_at IS NULL
		 RETURNING `+petColumns,
		existing.Name, existing.Breed, existing.Gender, existing.Age, existing.Weight,
		vJSON, dJSON, existing.AllergyHistory, existing.Notes,
		petID, memberID, merchantID,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update pet", err)
	}
	return p, nil
}

// Delete soft-deletes a pet.
func (s *Service) Delete(ctx context.Context, petID, memberID, merchantID int64) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE pets SET deleted_at = NOW() WHERE id = $1 AND member_id = $2 AND merchant_id = $3 AND deleted_at IS NULL`,
		petID, memberID, merchantID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to delete pet", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return apperrors.NewNotFoundError("pet not found")
	}
	return nil
}

func coalesceVaccineRecords(records []VaccineRecord) []VaccineRecord {
	if records == nil {
		return make([]VaccineRecord, 0)
	}
	return records
}

func coalesceDewormingRecords(records []DewormingRecord) []DewormingRecord {
	if records == nil {
		return make([]DewormingRecord, 0)
	}
	return records
}
