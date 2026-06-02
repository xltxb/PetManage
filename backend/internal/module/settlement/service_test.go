package settlement

import (
	"testing"

	"gorm.io/gorm"
)

type mockRepo struct {
	settlements    map[int64]*Settlement
	nextID         int64
	findErr        error
	createErr      error
	updateErr      error
}

func newMockRepo() *mockRepo {
	return &mockRepo{settlements: make(map[int64]*Settlement), nextID: 100}
}

func (m *mockRepo) FindByID(id int64) (*Settlement, error) {
	s, ok := m.settlements[id]
	if !ok { return nil, gorm.ErrRecordNotFound }
	return s, m.findErr
}
func (m *mockRepo) Create(s *Settlement) error { s.ID = m.nextID; m.nextID++; m.settlements[s.ID] = s; return m.createErr }
func (m *mockRepo) Update(s *Settlement) error { m.settlements[s.ID] = s; return m.updateErr }
func (m *mockRepo) CreateItems(items []SettlementItem) error { return nil }
func (m *mockRepo) CreatePayment(p *Payment) error { return nil }
func (m *mockRepo) ListByStore(storeID int64, status string, page, pageSize int) ([]Settlement, int64, error) { return nil, 0, nil }

func TestCreateSettlement(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	req := CreateSettlementRequest{
		StoreID:    1,
		CustomerID: 1,
		BizType:    "service",
		Items: []SettlementItemRequest{
			{SourceType: "appointment", SourceID: 1, Name: "全套SPA", UnitPrice: 26800, Quantity: 1},
		},
	}
	s, err := svc.Create(req)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if s.Status != StatusUnpaid {
		t.Errorf("Status = %q, want unpaid", s.Status)
	}
	if s.TotalAmount != 26800 {
		t.Errorf("TotalAmount = %d, want 26800", s.TotalAmount)
	}
}

func TestPaySettlement(t *testing.T) {
	repo := newMockRepo()
	repo.settlements[1] = &Settlement{ID: 1, StoreID: 1, Status: StatusUnpaid, TotalAmount: 26800}
	svc := NewService(repo)

	err := svc.Pay(1, 26800, "cash", 3)
	if err != nil {
		t.Fatalf("Pay() error: %v", err)
	}
	if repo.settlements[1].Status != StatusPaid {
		t.Errorf("Status = %q, want paid", repo.settlements[1].Status)
	}
}

func TestPaySettlementAlreadyPaid(t *testing.T) {
	repo := newMockRepo()
	repo.settlements[1] = &Settlement{ID: 1, Status: StatusPaid, TotalAmount: 26800}
	svc := NewService(repo)

	err := svc.Pay(1, 26800, "cash", 3)
	if err == nil {
		t.Fatal("expected error for already paid settlement")
	}
}

func TestRefundSettlement(t *testing.T) {
	repo := newMockRepo()
	repo.settlements[1] = &Settlement{ID: 1, Status: StatusPaid, TotalAmount: 26800, PaidAmount: 26800}
	svc := NewService(repo)

	err := svc.Refund(1, 3, "顾客要求退款")
	if err != nil {
		t.Fatalf("Refund() error: %v", err)
	}
	// Original should be refunded
	if repo.settlements[1].Status != StatusRefunded {
		t.Errorf("Status = %q, want refunded", repo.settlements[1].Status)
	}
	// A red-ink reversal settlement should be created (ID = mock nextID started at 100)
	redInk, ok := repo.settlements[100]
	if !ok {
		t.Fatal("expected red-ink settlement created at ID 100")
	}
	if redInk.PaidAmount != -26800 {
		t.Errorf("red-ink PaidAmount = %d, want -26800", redInk.PaidAmount)
	}
}

func TestVoidSettlement(t *testing.T) {
	repo := newMockRepo()
	repo.settlements[1] = &Settlement{ID: 1, Status: StatusUnpaid, TotalAmount: 26800}
	svc := NewService(repo)

	err := svc.Void(1)
	if err != nil {
		t.Fatalf("Void() error: %v", err)
	}
	if repo.settlements[1].Status != StatusVoid {
		t.Errorf("Status = %q, want void", repo.settlements[1].Status)
	}
}

func TestVoidPaidSettlement(t *testing.T) {
	repo := newMockRepo()
	repo.settlements[1] = &Settlement{ID: 1, Status: StatusPaid, TotalAmount: 26800}
	svc := NewService(repo)

	err := svc.Void(1)
	if err == nil {
		t.Fatal("expected error for voiding paid settlement")
	}
}

func TestPaymentNotEnabled(t *testing.T) {
	repo := newMockRepo()
	repo.settlements[1] = &Settlement{ID: 1, Status: StatusUnpaid, TotalAmount: 26800}
	svc := NewService(repo)

	err := svc.Pay(1, 26800, "wechat", 3)
	if err == nil {
		t.Fatal("expected payment not enabled for wechat")
	}
}
