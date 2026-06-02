package analytics

import (
	"testing"
	"time"
)

type mockRepo struct {
	revenueTrend   []RevenueTrendPoint
	serviceBreakdown []ServiceBreakdown
	peakHours      []PeakHourPoint
	retentionFunnel []RetentionBucket
}

func (m *mockRepo) GetRevenueTrend(storeID int64, months int) ([]RevenueTrendPoint, error) { return m.revenueTrend, nil }
func (m *mockRepo) GetServiceBreakdown(storeID int64, start, end time.Time) ([]ServiceBreakdown, error) { return m.serviceBreakdown, nil }
func (m *mockRepo) GetPeakHours(storeID int64, start, end time.Time) ([]PeakHourPoint, error) { return m.peakHours, nil }
func (m *mockRepo) GetRetentionFunnel(storeID int64) ([]RetentionBucket, error) { return m.retentionFunnel, nil }

func TestGetRevenueTrend(t *testing.T) {
	repo := &mockRepo{
		revenueTrend: []RevenueTrendPoint{
			{Month: "2026-01", Amount: 500000},
			{Month: "2026-02", Amount: 480000},
		},
	}
	svc := NewService(repo)
	result, err := svc.GetRevenueTrend(1, 12)
	if err != nil {
		t.Fatalf("GetRevenueTrend() error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("length = %d, want 2", len(result))
	}
}

func TestGetServiceBreakdown(t *testing.T) {
	repo := &mockRepo{
		serviceBreakdown: []ServiceBreakdown{
			{CategoryName: "美容", Amount: 300000, Percentage: 45.5},
			{CategoryName: "洗护", Amount: 200000, Percentage: 30.3},
		},
	}
	svc := NewService(repo)
	result, err := svc.GetServiceBreakdown(1, time.Now().AddDate(-1, 0, 0), time.Now())
	if err != nil {
		t.Fatalf("GetServiceBreakdown() error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("length = %d, want 2", len(result))
	}
}

func TestGetRetentionFunnel(t *testing.T) {
	repo := &mockRepo{
		retentionFunnel: []RetentionBucket{
			{Bucket: "1次", Count: 30},
			{Bucket: "2-3次", Count: 20},
			{Bucket: "4-6次", Count: 10},
			{Bucket: "7次+", Count: 5},
		},
	}
	svc := NewService(repo)
	result, err := svc.GetRetentionFunnel(1)
	if err != nil {
		t.Fatalf("GetRetentionFunnel() error: %v", err)
	}
	if len(result) != 4 {
		t.Errorf("length = %d, want 4", len(result))
	}
}
