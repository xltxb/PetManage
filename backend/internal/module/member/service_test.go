package member

import (
	"testing"

	"gorm.io/gorm"
)

type mockRepo struct {
	customers  map[int64]*Customer
	tiers      []MembershipTier
	txLogs     []WalletTransaction
	ptsLogs    []PointsTransaction
	nextCustID int64
	findErr    error
	createErr  error
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		customers:  make(map[int64]*Customer),
		nextCustID: 1,
		tiers: []MembershipTier{
			{ID: 1, Code: "normal", Name: "普通会员", MinTotalSpend: 0, DiscountRate: 100, PointsRate: 1.0, Sort: 0},
			{ID: 2, Code: "silver", Name: "银卡会员", MinTotalSpend: 200000, DiscountRate: 98, PointsRate: 1.0, Sort: 1},
			{ID: 3, Code: "gold", Name: "金卡会员", MinTotalSpend: 800000, DiscountRate: 95, PointsRate: 1.5, Sort: 2},
			{ID: 4, Code: "diamond", Name: "黑钻会员", MinTotalSpend: 2000000, DiscountRate: 90, PointsRate: 2.0, Sort: 3},
		},
	}
}

func (m *mockRepo) FindCustomerByID(id int64) (*Customer, error) {
	c, ok := m.customers[id]
	if !ok { return nil, gorm.ErrRecordNotFound }
	return c, m.findErr
}
func (m *mockRepo) UpdateCustomer(c *Customer) error { m.customers[c.ID] = c; return nil }
func (m *mockRepo) CreateWalletTx(tx *WalletTransaction) error { m.txLogs = append(m.txLogs, *tx); return nil }
func (m *mockRepo) CreatePointsTx(tx *PointsTransaction) error { m.ptsLogs = append(m.ptsLogs, *tx); return nil }
func (m *mockRepo) GetTiers() ([]MembershipTier, error) { return m.tiers, nil }
func (m *mockRepo) ListCustomers(storeID int64, keyword string, page, pageSize int) ([]Customer, int64, error) {
	return nil, 0, nil
}

// --- Wallet Tests ---

func TestRecharge(t *testing.T) {
	repo := newMockRepo()
	repo.customers[1] = &Customer{ID: 1, Name: "刘思远", WalletBalance: 0, TierID: 2}
	svc := NewService(repo)

	err := svc.Recharge(1, 50000, 1, 3, "现金充值")
	if err != nil {
		t.Fatalf("Recharge() error: %v", err)
	}
	if repo.customers[1].WalletBalance != 50000 {
		t.Errorf("WalletBalance = %d, want 50000", repo.customers[1].WalletBalance)
	}
	if len(repo.txLogs) != 1 || repo.txLogs[0].Type != "recharge" {
		t.Errorf("wallet tx not created correctly")
	}
	if repo.txLogs[0].BalanceAfter != 50000 {
		t.Errorf("BalanceAfter = %d, want 50000", repo.txLogs[0].BalanceAfter)
	}
}

func TestWalletConsume(t *testing.T) {
	repo := newMockRepo()
	repo.customers[1] = &Customer{ID: 1, WalletBalance: 50000, TierID: 2}
	svc := NewService(repo)

	err := svc.WalletConsume(1, 26800, 1, 3, "消费")
	if err != nil {
		t.Fatalf("WalletConsume() error: %v", err)
	}
	if repo.customers[1].WalletBalance != 23200 {
		t.Errorf("WalletBalance = %d, want 23200", repo.customers[1].WalletBalance)
	}
}

func TestWalletConsumeInsufficient(t *testing.T) {
	repo := newMockRepo()
	repo.customers[1] = &Customer{ID: 1, WalletBalance: 10000, TierID: 2}
	svc := NewService(repo)

	err := svc.WalletConsume(1, 26800, 1, 3, "消费")
	if err == nil {
		t.Fatal("expected insufficient wallet error")
	}
}

func TestWalletAdjustRequiresReason(t *testing.T) {
	repo := newMockRepo()
	repo.customers[1] = &Customer{ID: 1, WalletBalance: 50000}
	svc := NewService(repo)

	err := svc.WalletAdjust(1, 10000, 1, 3, "")
	if err == nil {
		t.Fatal("expected error for adjust without reason")
	}
}

// --- Points Tests ---

func TestEarnPoints(t *testing.T) {
	repo := newMockRepo()
	// Gold tier: points_rate = 1.5
	repo.customers[1] = &Customer{ID: 1, PointsBalance: 0, TierID: 3, TotalSpend: 800000}
	svc := NewService(repo)

	pts, err := svc.EarnPoints(1, 20000, 1, 3, "settlement", 100)
	if err != nil {
		t.Fatalf("EarnPoints() error: %v", err)
	}
	// 20000/100 = 200 yuan * 1.5 = 300 points
	if pts != 300 {
		t.Errorf("points earned = %d, want 300", pts)
	}
	if repo.customers[1].PointsBalance != 300 {
		t.Errorf("PointsBalance = %d, want 300", repo.customers[1].PointsBalance)
	}
}

// --- Tier Upgrade Tests ---

func TestTierUpgrade(t *testing.T) {
	repo := newMockRepo()
	// total_spend close to gold threshold (800000)
	repo.customers[1] = &Customer{ID: 1, TierID: 2, TotalSpend: 790000, WalletBalance: 100000}
	svc := NewService(repo)

	// After adding 20000, total_spend = 810000 > 800000 → gold
	upgraded, newTier := svc.CheckTierUpgrade(1, 810000)
	if !upgraded {
		t.Fatal("expected tier upgrade")
	}
	if newTier != "gold" {
		t.Errorf("newTier = %q, want gold", newTier)
	}
}

func TestNoDowngrade(t *testing.T) {
	repo := newMockRepo()
	repo.customers[1] = &Customer{ID: 1, TierID: 3, TotalSpend: 900000} // already gold
	svc := NewService(repo)

	// Total spend stays below diamond threshold — stays gold
	upgraded, _ := svc.CheckTierUpgrade(1, 900000)
	if upgraded {
		t.Error("should not upgrade (already at highest for spend)")
	}
}
