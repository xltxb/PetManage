package pet

import (
	"testing"
	"time"

	"gorm.io/gorm"
)

type mockRepo struct {
	pets          map[int64]*Pet
	healthRecords map[int64][]HealthRecord
	weightRecords map[int64][]WeightRecord
	consumption   map[int64][]ConsumptionRecord
	nextPetID     int64
	nextHealthID  int64
	nextWeightID  int64
	findErr       error
	createErr     error
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		pets:          make(map[int64]*Pet),
		healthRecords: make(map[int64][]HealthRecord),
		weightRecords: make(map[int64][]WeightRecord),
		consumption:   make(map[int64][]ConsumptionRecord),
		nextPetID:     1, nextHealthID: 1, nextWeightID: 1,
	}
}

func (m *mockRepo) Create(p *Pet) error {
	p.ID = m.nextPetID
	m.nextPetID++
	m.pets[p.ID] = p
	return m.createErr
}
func (m *mockRepo) FindByID(id int64) (*Pet, error) {
	p, ok := m.pets[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return p, m.findErr
}
func (m *mockRepo) Update(p *Pet) error { m.pets[p.ID] = p; return nil }
func (m *mockRepo) ListByCustomer(customerID int64) ([]Pet, error) {
	var list []Pet
	for _, p := range m.pets {
		if p.CustomerID == customerID {
			list = append(list, *p)
		}
	}
	return list, nil
}
func (m *mockRepo) CreateHealthRecord(r *HealthRecord) error {
	r.ID = m.nextHealthID
	m.nextHealthID++
	m.healthRecords[r.PetID] = append(m.healthRecords[r.PetID], *r)
	return nil
}
func (m *mockRepo) FindHealthRecords(petID int64) ([]HealthRecord, error) {
	return m.healthRecords[petID], nil
}
func (m *mockRepo) CreateWeightRecord(r *WeightRecord) error {
	r.ID = m.nextWeightID
	m.nextWeightID++
	m.weightRecords[r.PetID] = append(m.weightRecords[r.PetID], *r)
	return nil
}
func (m *mockRepo) FindWeightRecords(petID int64) ([]WeightRecord, error) {
	return m.weightRecords[petID], nil
}
func (m *mockRepo) FindConsumptionRecords(petID int64) ([]ConsumptionRecord, error) {
	return m.consumption[petID], nil
}

func TestCalculateAge(t *testing.T) {
	birthday := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	years, months := CalculateAge(birthday, now)
	if years != 2 {
		t.Errorf("years = %d, want 2", years)
	}
	if months < 4 || months > 5 {
		t.Errorf("months = %d, want 4-5", months)
	}
}

func TestCalculateAgeNoBirthday(t *testing.T) {
	years, months := CalculateAge(time.Time{}, time.Now())
	if years != 0 {
		t.Errorf("years = %d, want 0 for no birthday", years)
	}
	_ = months
}

func TestCreatePet(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	birthday := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	p, err := svc.Create(CreatePetRequest{
		CustomerID: 1, Name: "布丁", Species: 1, Breed: "比熊犬",
		Gender: 1, Birthday: &birthday, WeightG: 5200, ChipNo: "15600218843",
	})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if p.Name != "布丁" {
		t.Errorf("Name = %q", p.Name)
	}
	if p.Status != 1 {
		t.Errorf("Status = %d, want 1", p.Status)
	}
}

func TestAddHealthRecord(t *testing.T) {
	repo := newMockRepo()
	repo.pets[1] = &Pet{ID: 1, Name: "布丁"}
	svc := NewService(repo)

	err := svc.AddHealthRecord(1, HealthRecordRequest{
		Type: "vaccine", Title: "狂犬+八联",
		PerformedAt: "2026-03-12", NextDueAt: "2026-09-12",
	})
	if err != nil {
		t.Fatalf("AddHealthRecord() error: %v", err)
	}
	records := repo.healthRecords[1]
	if len(records) != 1 {
		t.Fatalf("records count = %d, want 1", len(records))
	}
	if records[0].Type != "vaccine" {
		t.Errorf("type = %q", records[0].Type)
	}
}

func TestAddWeightRecord(t *testing.T) {
	repo := newMockRepo()
	repo.pets[1] = &Pet{ID: 1, Name: "布丁", WeightG: 5000}
	svc := NewService(repo)

	err := svc.AddWeightRecord(1, 5200)
	if err != nil {
		t.Fatalf("AddWeightRecord() error: %v", err)
	}
	if repo.pets[1].WeightG != 5200 {
		t.Errorf("WeightG = %d, want 5200 (updated)", repo.pets[1].WeightG)
	}
}

func TestGetConsumptionHistory(t *testing.T) {
	repo := newMockRepo()
	repo.pets[1] = &Pet{ID: 1, Name: "布丁"}
	repo.consumption[1] = []ConsumptionRecord{
		{
			Type:       "appointment",
			SourceID:   10,
			OccurredAt: time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
			Title:      "全套SPA",
			Amount:     26800,
			Status:     "completed",
		},
		{
			Type:       "boarding",
			SourceID:   20,
			OccurredAt: time.Date(2026, 6, 3, 18, 0, 0, 0, time.UTC),
			Title:      "寄养服务",
			Amount:     50400,
			Status:     "checked_out",
		},
	}
	svc := NewService(repo)

	rows, err := svc.GetConsumptionHistory(1)
	if err != nil {
		t.Fatalf("GetConsumptionHistory() error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].Type != "boarding" || rows[0].SourceID != 20 {
		t.Fatalf("first row = %#v, want latest boarding row", rows[0])
	}
	if rows[1].Type != "appointment" || rows[1].SourceID != 10 {
		t.Fatalf("second row = %#v, want older appointment row", rows[1])
	}
}
