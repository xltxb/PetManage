package pet

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
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
	Name             string             `json:"name"`
	Breed            string             `json:"breed"`
	Gender           string             `json:"gender"`
	Age              int                `json:"age"`
	Weight           string             `json:"weight"`
	VaccineRecords   []VaccineRecord    `json:"vaccine_records"`
	DewormingRecords []DewormingRecord  `json:"deworming_records"`
	AllergyHistory   string             `json:"allergy_history"`
	Notes            string             `json:"notes"`
}

// HealthReminder represents a vaccine or deworming due reminder.
type HealthReminder struct {
	PetID        int64  `json:"pet_id"`
	PetName      string `json:"pet_name"`
	MemberID     int64  `json:"member_id"`
	MemberName   string `json:"member_name"`
	CardNo       string `json:"card_no"`
	ReminderType string `json:"reminder_type"`
	ItemName     string `json:"item_name"`
	LastDate     string `json:"last_date"`
	NextDate     string `json:"next_date"`
	DaysLeft     int    `json:"days_left"`
	Notes        string `json:"notes,omitempty"`
}

// HealthReminderCounts holds counts for different reminder types.
type HealthReminderCounts struct {
	VaccineCount   int `json:"vaccine_count"`
	DewormingCount int `json:"deworming_count"`
}

// HealthReminderParams holds query parameters.
type HealthReminderParams struct {
	Type       string
	Days       int
	Page       int
	PageSize   int
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

	vJSON, _ := json.Marshal(normalizeVaccineRecords(req.VaccineRecords))
	dJSON, _ := json.Marshal(normalizeDewormingRecords(req.DewormingRecords))

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
		existing.VaccineRecords = normalizeVaccineRecords(*req.VaccineRecords)
	}
	if req.DewormingRecords != nil {
		existing.DewormingRecords = normalizeDewormingRecords(*req.DewormingRecords)
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

const dateLayout = "2006-01-02"

// normalizeVaccineRecords ensures each record has a next_date.
// If next_date is empty, defaults to date + 1 year.
func normalizeVaccineRecords(records []VaccineRecord) []VaccineRecord {
	if records == nil {
		return make([]VaccineRecord, 0)
	}
	for i := range records {
		if records[i].NextDate == "" && records[i].Date != "" {
			if t, err := time.Parse(dateLayout, records[i].Date); err == nil {
				records[i].NextDate = t.AddDate(1, 0, 0).Format(dateLayout)
			}
		}
	}
	return records
}


// normalizeDewormingRecords ensures each record has a next_date.
// If next_date is empty, defaults to date + 3 months.
func normalizeDewormingRecords(records []DewormingRecord) []DewormingRecord {
	if records == nil {
		return make([]DewormingRecord, 0)
	}
	for i := range records {
		if records[i].NextDate == "" && records[i].Date != "" {
			if t, err := time.Parse(dateLayout, records[i].Date); err == nil {
				records[i].NextDate = t.AddDate(0, 3, 0).Format(dateLayout)
			}
		}
	}
	return records
}

// GetHealthReminders returns vaccine and/or deworming reminders for a merchant.
func (s *Service) GetHealthReminders(ctx context.Context, merchantID int64, params HealthReminderParams) ([]HealthReminder, int, error) {
	if params.Days <= 0 {
		params.Days = 7
	}
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}

	const baseQuery = `
		SELECT p.id AS pet_id, p.name AS pet_name, p.member_id,
		       m.name AS member_name, m.card_no,
		       '%s' AS reminder_type,
		       %s AS item_name,
		       %s AS last_date,
		       %s AS next_date,
		       (%s)::date - CURRENT_DATE AS days_left,
		       COALESCE(%s, '') AS notes
		FROM pets p
		JOIN members m ON p.member_id = m.id AND m.deleted_at IS NULL
		CROSS JOIN LATERAL jsonb_array_elements(%s) AS rec(record)
		WHERE p.merchant_id = $1 AND p.deleted_at IS NULL AND m.status = 'active'
		  AND %s IS NOT NULL AND %s != ''
		  AND (%s)::date BETWEEN CURRENT_DATE AND (CURRENT_DATE + ($2 || ' days')::interval)`

	var allReminders []HealthReminder

	if params.Type == "" || params.Type == "all" || params.Type == "vaccine" {
		r, err := s.queryReminders(ctx, baseQuery, "vaccine",
			"rec.record->>'vaccine_name'",
			"rec.record->>'date'",
			"rec.record->>'next_date'",
			"rec.record->>'next_date'",
			"rec.record->>'notes'",
			"p.vaccine_records",
			"rec.record->>'next_date'",
			"rec.record->>'next_date'",
			"rec.record->>'next_date'",
			merchantID, params.Days)
		if err != nil {
			return nil, 0, err
		}
		allReminders = append(allReminders, r...)
	}

	if params.Type == "" || params.Type == "all" || params.Type == "deworming" {
		r, err := s.queryReminders(ctx, baseQuery, "deworming",
			"rec.record->>'medicine_name'",
			"rec.record->>'date'",
			"rec.record->>'next_date'",
			"rec.record->>'next_date'",
			"rec.record->>'notes'",
			"p.deworming_records",
			"rec.record->>'next_date'",
			"rec.record->>'next_date'",
			"rec.record->>'next_date'",
			merchantID, params.Days)
		if err != nil {
			return nil, 0, err
		}
		allReminders = append(allReminders, r...)
	}

	sort.Slice(allReminders, func(i, j int) bool {
		if allReminders[i].DaysLeft != allReminders[j].DaysLeft {
			return allReminders[i].DaysLeft < allReminders[j].DaysLeft
		}
		return allReminders[i].PetName < allReminders[j].PetName
	})

	total := len(allReminders)
	offset := (params.Page - 1) * params.PageSize
	if offset >= total {
		return []HealthReminder{}, total, nil
	}
	end := offset + params.PageSize
	if end > total {
		end = total
	}
	return allReminders[offset:end], total, nil
}

func (s *Service) queryReminders(ctx context.Context, queryFmt string, rtype string,
	itemCol, lastCol, nextCol, nextCol2, notesCol, jsonCol, cond1, cond2, cond3 string,
	merchantID int64, days int) ([]HealthReminder, error) {

	q := fmt.Sprintf(queryFmt, rtype, itemCol, lastCol, nextCol, nextCol2, notesCol, jsonCol, cond1, cond2, cond3)
	rows, err := s.db.QueryContext(ctx, q, merchantID, days)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query health reminders", err)
	}
	defer rows.Close()

	var reminders []HealthReminder
	for rows.Next() {
		var r HealthReminder
		if err := rows.Scan(&r.PetID, &r.PetName, &r.MemberID, &r.MemberName, &r.CardNo,
			&r.ReminderType, &r.ItemName, &r.LastDate, &r.NextDate, &r.DaysLeft, &r.Notes); err != nil {
			return nil, apperrors.NewInternalError("failed to scan health reminder", err)
		}
		reminders = append(reminders, r)
	}
	if reminders == nil {
		reminders = make([]HealthReminder, 0)
	}
	return reminders, nil
}

// GetHealthReminderCounts returns vaccine and deworming reminder counts for a merchant.
func (s *Service) GetHealthReminderCounts(ctx context.Context, merchantID int64, withinDays int) (*HealthReminderCounts, error) {
	if withinDays <= 0 {
		withinDays = 7
	}
	counts := &HealthReminderCounts{}

	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM (
			SELECT 1 FROM pets p
			JOIN members m ON p.member_id = m.id AND m.deleted_at IS NULL
			CROSS JOIN LATERAL jsonb_array_elements(p.vaccine_records) AS v(record)
			WHERE p.merchant_id = $1 AND p.deleted_at IS NULL AND m.status = 'active'
			  AND v.record->>'next_date' IS NOT NULL AND v.record->>'next_date' != ''
			  AND (v.record->>'next_date')::date BETWEEN CURRENT_DATE AND (CURRENT_DATE + ($2 || ' days')::interval)
		) sub`,
		merchantID, withinDays,
	).Scan(&counts.VaccineCount)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count vaccine reminders", err)
	}

	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM (
			SELECT 1 FROM pets p
			JOIN members m ON p.member_id = m.id AND m.deleted_at IS NULL
			CROSS JOIN LATERAL jsonb_array_elements(p.deworming_records) AS d(record)
			WHERE p.merchant_id = $1 AND p.deleted_at IS NULL AND m.status = 'active'
			  AND d.record->>'next_date' IS NOT NULL AND d.record->>'next_date' != ''
			  AND (d.record->>'next_date')::date BETWEEN CURRENT_DATE AND (CURRENT_DATE + ($2 || ' days')::interval)
		) sub`,
		merchantID, withinDays,
	).Scan(&counts.DewormingCount)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count deworming reminders", err)
	}

	return counts, nil
}
