package dashboard

import (
	"testing"
	"time"
)

// mockRepo implements Repository for testing
type mockRepo struct {
	revenueToday      int64
	appointmentCount  int64
	petsInStore       int64
	newMembersCount   int64
	revenueTrend      []RevenuePoint
	todayAppointments []AppointmentTimelineItem
	popularServices   []ServiceRank
	inventoryAlerts   []InventoryAlert
	memberComposition []TierComposition
	summaryErr        error
}

func (m *mockRepo) GetRevenueToday(storeID int64, tz string) (int64, error) {
	return m.revenueToday, m.summaryErr
}
func (m *mockRepo) GetAppointmentCountToday(storeID int64, tz string) (int64, error) {
	return m.appointmentCount, m.summaryErr
}
func (m *mockRepo) GetPetsInStore(storeID int64, tz string) (int64, error) {
	return m.petsInStore, m.summaryErr
}
func (m *mockRepo) GetNewMembersToday(storeID int64, tz string) (int64, error) {
	return m.newMembersCount, m.summaryErr
}
func (m *mockRepo) GetRevenueTrend(storeID int64, days int, tz string) ([]RevenuePoint, error) {
	return m.revenueTrend, m.summaryErr
}
func (m *mockRepo) GetTodayAppointments(storeID int64, tz string) ([]AppointmentTimelineItem, error) {
	return m.todayAppointments, m.summaryErr
}
func (m *mockRepo) GetPopularServices(storeID int64, month time.Time) ([]ServiceRank, error) {
	return m.popularServices, m.summaryErr
}
func (m *mockRepo) GetInventoryAlerts(storeID int64) ([]InventoryAlert, error) {
	return m.inventoryAlerts, m.summaryErr
}
func (m *mockRepo) GetMemberComposition(storeID int64) ([]TierComposition, error) {
	return m.memberComposition, m.summaryErr
}

func TestGetSummary(t *testing.T) {
	repo := &mockRepo{
		revenueToday:      100000,
		appointmentCount:  8,
		petsInStore:       12,
		newMembersCount:   3,
		revenueTrend: []RevenuePoint{
			{Date: "2026-05-20", Amount: 50000},
			{Date: "2026-05-21", Amount: 80000},
		},
		todayAppointments: []AppointmentTimelineItem{
			{ID: 1, PetName: "布丁", ServiceName: "全套SPA", ScheduledStart: time.Now(), Status: "in_progress"},
		},
		popularServices: []ServiceRank{
			{ServiceName: "全套SPA·小型犬", Count: 15},
		},
		inventoryAlerts: []InventoryAlert{
			{ProductName: "皇家幼犬粮 2kg", Quantity: 6, SafetyStock: 8},
		},
		memberComposition: []TierComposition{
			{TierName: "普通会员", Count: 50},
			{TierName: "银卡会员", Count: 20},
		},
	}

	svc := NewService(repo, "Asia/Shanghai")
	summary, err := svc.GetSummary(1)
	if err != nil {
		t.Fatalf("GetSummary() error: %v", err)
	}

	if summary.RevenueToday != 100000 {
		t.Errorf("RevenueToday = %d, want 100000", summary.RevenueToday)
	}
	if summary.AppointmentCount != 8 {
		t.Errorf("AppointmentCount = %d, want 8", summary.AppointmentCount)
	}
	if summary.PetsInStore != 12 {
		t.Errorf("PetsInStore = %d, want 12", summary.PetsInStore)
	}
	if summary.NewMembersCount != 3 {
		t.Errorf("NewMembersCount = %d, want 3", summary.NewMembersCount)
	}
	if len(summary.RevenueTrend) != 2 {
		t.Errorf("RevenueTrend length = %d, want 2", len(summary.RevenueTrend))
	}
	if len(summary.TodayAppointments) != 1 {
		t.Errorf("TodayAppointments length = %d, want 1", len(summary.TodayAppointments))
	}
	if len(summary.PopularServices) != 1 {
		t.Errorf("PopularServices length = %d, want 1", len(summary.PopularServices))
	}
	if len(summary.InventoryAlerts) != 1 {
		t.Errorf("InventoryAlerts length = %d, want 1", len(summary.InventoryAlerts))
	}
	if len(summary.MemberComposition) != 2 {
		t.Errorf("MemberComposition length = %d, want 2", len(summary.MemberComposition))
	}
}

func TestGetSummaryRequiresStoreID(t *testing.T) {
	repo := &mockRepo{}
	svc := NewService(repo, "Asia/Shanghai")
	_, err := svc.GetSummary(0)
	if err == nil {
		t.Fatal("expected error for storeID=0")
	}
}
