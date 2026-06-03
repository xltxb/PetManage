package settlement

import (
	"errors"
	"testing"

	"gorm.io/gorm"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/errcode"
)

type mockRepo struct {
	settlements map[int64]*Settlement
	items       map[int64][]SettlementItem
	payments    []*Payment
	nextID      int64
	findErr     error
	createErr   error
	updateErr   error
}

func newMockRepo() *mockRepo {
	return &mockRepo{settlements: make(map[int64]*Settlement), items: make(map[int64][]SettlementItem), nextID: 100}
}

func (m *mockRepo) FindByID(id int64) (*Settlement, error) {
	s, ok := m.settlements[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return s, m.findErr
}
func (m *mockRepo) Create(s *Settlement) error {
	s.ID = m.nextID
	m.nextID++
	m.settlements[s.ID] = s
	return m.createErr
}
func (m *mockRepo) Update(s *Settlement) error { m.settlements[s.ID] = s; return m.updateErr }
func (m *mockRepo) CreateItems(items []SettlementItem) error {
	if len(items) > 0 {
		m.items[items[0].SettlementID] = append(m.items[items[0].SettlementID], items...)
	}
	return nil
}
func (m *mockRepo) FindItems(settlementID int64) ([]SettlementItem, error) {
	return m.items[settlementID], nil
}
func (m *mockRepo) CreatePayment(p *Payment) error {
	m.payments = append(m.payments, p)
	return nil
}
func (m *mockRepo) CreateReceipt(storeID, settlementID, operatorID int64, content map[string]interface{}) error {
	return nil
}
func (m *mockRepo) ListByStore(storeID int64, status string, page, pageSize int) ([]Settlement, int64, error) {
	return nil, 0, nil
}

type fakeMemberEffects struct {
	walletAmount  int64
	pointsAmount  int64
	spendAmount   int64
	reverseAmount int64
}

func (f *fakeMemberEffects) WalletConsume(customerID, amount, storeID, operatorID int64, remark string) error {
	f.walletAmount = amount
	return nil
}

func (f *fakeMemberEffects) EarnPoints(customerID, amountPaid, storeID, operatorID int64, refType string, refID int64) (int64, error) {
	f.pointsAmount = amountPaid
	return 268, nil
}

func (f *fakeMemberEffects) ApplyPaidSpend(customerID, amountPaid, storeID, operatorID int64, refType string, refID int64) error {
	f.spendAmount = amountPaid
	return nil
}

func (f *fakeMemberEffects) ReverseSettlement(customerID, amountPaid, storeID, operatorID int64, refID int64) error {
	f.reverseAmount = amountPaid
	return nil
}

type fakeInventoryEffects struct {
	productID         int64
	quantity          int
	reversedProductID int64
	reversedQuantity  int
}

func (f *fakeInventoryEffects) SaleOut(storeID, productID int64, quantity int, operatorID int64, refType string, refID int64) error {
	f.productID = productID
	f.quantity = quantity
	return nil
}

func (f *fakeInventoryEffects) ReverseSaleOut(storeID, productID int64, quantity int, operatorID int64, refType string, refID int64) error {
	f.reversedProductID = productID
	f.reversedQuantity = quantity
	return nil
}

type fakePrintJobs struct {
	jobs []fakePrintJob
}

type fakePrintJob struct {
	StoreID    int64
	RefID      int64
	OperatorID int64
	Content    map[string]interface{}
}

func (f *fakePrintJobs) CreateReceipt(storeID, settlementID, operatorID int64, content map[string]interface{}) error {
	f.jobs = append(f.jobs, fakePrintJob{StoreID: storeID, RefID: settlementID, OperatorID: operatorID, Content: content})
	return nil
}

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

func TestGenerateCodeDoesNotCollideInBurst(t *testing.T) {
	seen := make(map[string]bool, 2000)
	for i := 0; i < 2000; i++ {
		code := GenerateCode()
		if seen[code] {
			t.Fatalf("duplicate settlement code generated: %s", code)
		}
		seen[code] = true
	}
}

func TestPrintJobOperatorIDValueOmitsZero(t *testing.T) {
	if got := printJobOperatorIDValue(0); got != nil {
		t.Fatalf("operator id value for 0 = %#v, want nil", got)
	}
	value := printJobOperatorIDValue(9)
	ptr, ok := value.(*int64)
	if !ok || ptr == nil || *ptr != 9 {
		t.Fatalf("operator id value for 9 = %#v, want *int64(9)", value)
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

func TestPayWalletSettlementDeductsWalletInventoryAndAwardsPoints(t *testing.T) {
	customerID := int64(100)
	repo := newMockRepo()
	repo.settlements[1] = &Settlement{ID: 1, StoreID: 1, CustomerID: &customerID, Status: StatusUnpaid, TotalAmount: 26800}
	repo.items[1] = []SettlementItem{{SettlementID: 1, SourceType: "product", SourceID: 10, Name: "犬粮", UnitPrice: 26800, Quantity: 1, Amount: 26800}}
	memberSvc := &fakeMemberEffects{}
	inventorySvc := &fakeInventoryEffects{}
	prints := &fakePrintJobs{}
	svc := NewService(repo, WithMemberEffects(memberSvc), WithInventoryEffects(inventorySvc), WithPrintJobs(prints))

	err := svc.Pay(1, 26800, PayWallet, 9)
	if err != nil {
		t.Fatalf("Pay error = %v", err)
	}
	if memberSvc.walletAmount != 26800 || memberSvc.pointsAmount != 26800 || memberSvc.spendAmount != 26800 {
		t.Fatalf("member effects = %#v", memberSvc)
	}
	if inventorySvc.productID != 10 || inventorySvc.quantity != 1 {
		t.Fatalf("inventory effects = %#v", inventorySvc)
	}
	if len(prints.jobs) != 1 || prints.jobs[0].RefID != 1 {
		t.Fatalf("print jobs = %#v", prints.jobs)
	}
}

func TestPayWechatReturnsPaymentNotEnabledCode(t *testing.T) {
	repo := newMockRepo()
	repo.settlements[1] = &Settlement{ID: 1, StoreID: 1, Status: StatusUnpaid, TotalAmount: 26800}
	svc := NewService(repo)

	err := svc.Pay(1, 26800, PayWechat, 9)
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != errcode.PaymentNotEnabled {
		t.Fatalf("err = %v, want payment not enabled", err)
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

func TestRefundSettlementReversesMemberAndProductEffects(t *testing.T) {
	customerID := int64(100)
	repo := newMockRepo()
	repo.settlements[1] = &Settlement{
		ID: 1, StoreID: 1, CustomerID: &customerID,
		Status: StatusPaid, TotalAmount: 26800, PaidAmount: 26800,
	}
	repo.items[1] = []SettlementItem{{
		SettlementID: 1,
		SourceType:   "product",
		SourceID:     10,
		Name:         "犬粮",
		UnitPrice:    26800,
		Quantity:     2,
		Amount:       53600,
	}}
	memberSvc := &fakeMemberEffects{}
	inventorySvc := &fakeInventoryEffects{}
	svc := NewService(repo, WithMemberEffects(memberSvc), WithInventoryEffects(inventorySvc))

	err := svc.Refund(1, 9, "顾客要求退款")
	if err != nil {
		t.Fatalf("Refund error = %v", err)
	}
	if memberSvc.reverseAmount != 26800 {
		t.Fatalf("member reverse amount = %d, want 26800", memberSvc.reverseAmount)
	}
	if inventorySvc.reversedProductID != 10 || inventorySvc.reversedQuantity != 2 {
		t.Fatalf("inventory reverse = product %d quantity %d, want product 10 quantity 2",
			inventorySvc.reversedProductID, inventorySvc.reversedQuantity)
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
