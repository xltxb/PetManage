package inventory

import (
	"testing"

	"gorm.io/gorm"

	"pawprint/backend/internal/module/notification"
)

type mockRepo struct {
	inventory    map[int64]*InventoryItem
	transactions []StockTransaction
	nextTxID     int64
	findErr      error
	updateErr    error
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		inventory: make(map[int64]*InventoryItem),
		nextTxID:  1,
	}
}

func (m *mockRepo) GetInventory(storeID, productID int64) (*InventoryItem, error) {
	key := productID
	inv, ok := m.inventory[key]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return inv, m.findErr
}
func (m *mockRepo) GetInventoryForUpdate(storeID, productID int64) (*InventoryItem, error) {
	return m.GetInventory(storeID, productID)
}
func (m *mockRepo) UpdateInventory(inv *InventoryItem) error {
	m.inventory[inv.ProductID] = inv
	return m.updateErr
}
func (m *mockRepo) CreateTransaction(tx *StockTransaction) error {
	tx.ID = m.nextTxID
	m.nextTxID++
	m.transactions = append(m.transactions, *tx)
	return nil
}
func (m *mockRepo) CheckSafetyStock(storeID int64) ([]InventoryAlert, error) { return nil, nil }
func (m *mockRepo) WithTx(fn func(Repository) error) error                   { return fn(m) }

type fakeNotifier struct {
	sent []notification.SendRequest
}

func (f *fakeNotifier) Send(req notification.SendRequest) error {
	f.sent = append(f.sent, req)
	return nil
}

type fakeSettingsProvider struct {
	settings map[string]interface{}
	err      error
}

func (f fakeSettingsProvider) GetAll(storeID int64) (map[string]interface{}, error) {
	return f.settings, f.err
}

func TestSaleOut(t *testing.T) {
	repo := newMockRepo()
	repo.inventory[1] = &InventoryItem{StoreID: 1, ProductID: 1, Quantity: 6, SafetyStock: 8}
	svc := NewService(repo)

	err := svc.SaleOut(1, 1, 2, 3, "sale", 100)
	if err != nil {
		t.Fatalf("SaleOut() error: %v", err)
	}
	if repo.inventory[1].Quantity != 4 {
		t.Errorf("Quantity = %d, want 4", repo.inventory[1].Quantity)
	}
	// Check transaction created
	if len(repo.transactions) != 1 {
		t.Fatalf("transactions count = %d, want 1", len(repo.transactions))
	}
	tx := repo.transactions[0]
	if tx.Type != "sale_out" || tx.Quantity != -2 || tx.BalanceAfter != 4 {
		t.Errorf("tx: type=%s qty=%d bal=%d", tx.Type, tx.Quantity, tx.BalanceAfter)
	}
}

func TestSaleOutInsufficientStock(t *testing.T) {
	repo := newMockRepo()
	repo.inventory[1] = &InventoryItem{StoreID: 1, ProductID: 1, Quantity: 4, SafetyStock: 8}
	svc := NewService(repo)

	err := svc.SaleOut(1, 1, 10, 3, "sale", 100)
	if err == nil {
		t.Fatal("expected insufficient stock error")
	}
	if repo.inventory[1].Quantity != 4 {
		t.Errorf("Quantity should not change on error, got %d", repo.inventory[1].Quantity)
	}
}

func TestSaleOutAllowsNegativeStockWhenConfigured(t *testing.T) {
	repo := newMockRepo()
	repo.inventory[1] = &InventoryItem{StoreID: 1, ProductID: 1, Quantity: 4, SafetyStock: 8}
	svc := NewService(repo, WithSettings(fakeSettingsProvider{
		settings: map[string]interface{}{"inventory.allow_negative": true},
	}))

	err := svc.SaleOut(1, 1, 10, 3, "sale", 100)
	if err != nil {
		t.Fatalf("SaleOut() error: %v", err)
	}
	if repo.inventory[1].Quantity != -6 {
		t.Fatalf("Quantity = %d, want -6", repo.inventory[1].Quantity)
	}
	if len(repo.transactions) != 1 {
		t.Fatalf("transactions count = %d, want 1", len(repo.transactions))
	}
	if repo.transactions[0].BalanceAfter != -6 {
		t.Fatalf("BalanceAfter = %d, want -6", repo.transactions[0].BalanceAfter)
	}
}

func TestSaleOutTriggersSafetyAlert(t *testing.T) {
	repo := newMockRepo()
	repo.inventory[1] = &InventoryItem{StoreID: 1, ProductID: 1, Quantity: 8, SafetyStock: 10}
	svc := NewService(repo)

	err := svc.SaleOut(1, 1, 2, 3, "sale", 100)
	if err != nil {
		t.Fatalf("SaleOut() error: %v", err)
	}
	if !repo.inventory[1].HasAlert {
		t.Error("expected safety stock alert triggered (6 <= 10)")
	}
}

func TestSaleOutCreatesStockLowNotification(t *testing.T) {
	repo := newMockRepo()
	repo.inventory[1] = &InventoryItem{ID: 1, StoreID: 1, ProductID: 1, Quantity: 6, SafetyStock: 8}
	notifier := &fakeNotifier{}
	svc := NewService(repo, WithNotifier(notifier))

	err := svc.SaleOut(1, 1, 2, 3, "sale", 100)
	if err != nil {
		t.Fatalf("SaleOut() error: %v", err)
	}
	if len(notifier.sent) != 1 {
		t.Fatalf("notifications count = %d, want 1", len(notifier.sent))
	}
	if notifier.sent[0].TemplateCode != "stock_low" || notifier.sent[0].Channel != notification.ChannelInApp {
		t.Fatalf("notification = %#v", notifier.sent[0])
	}
}

func TestPurchaseIn(t *testing.T) {
	repo := newMockRepo()
	repo.inventory[1] = &InventoryItem{StoreID: 1, ProductID: 1, Quantity: 4, SafetyStock: 8}
	svc := NewService(repo)

	err := svc.PurchaseIn(1, 1, 20, 8, "purchase", 200)
	if err != nil {
		t.Fatalf("PurchaseIn() error: %v", err)
	}
	if repo.inventory[1].Quantity != 24 {
		t.Errorf("Quantity = %d, want 24", repo.inventory[1].Quantity)
	}
	tx := repo.transactions[0]
	if tx.Type != "purchase_in" || tx.Quantity != 20 {
		t.Errorf("tx type=%s qty=%d", tx.Type, tx.Quantity)
	}
}

func TestAdjustInventory(t *testing.T) {
	repo := newMockRepo()
	repo.inventory[1] = &InventoryItem{StoreID: 1, ProductID: 1, Quantity: 10, SafetyStock: 5}
	svc := NewService(repo)

	err := svc.Adjust(1, 1, -3, 3, "盘点调整")
	if err != nil {
		t.Fatalf("Adjust() error: %v", err)
	}
	if repo.inventory[1].Quantity != 7 {
		t.Errorf("Quantity = %d, want 7", repo.inventory[1].Quantity)
	}
}

func TestAdjustInventoryRequiresReason(t *testing.T) {
	repo := newMockRepo()
	repo.inventory[1] = &InventoryItem{StoreID: 1, ProductID: 1, Quantity: 10}
	svc := NewService(repo)

	err := svc.Adjust(1, 1, -3, 3, "")
	if err == nil {
		t.Fatal("expected error for adjust without reason")
	}
}
