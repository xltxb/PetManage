package analytics

import (
	"time"

	"pawprint/backend/internal/pkg/apperr"
)

// Service handles analytics business logic.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// GetRevenueTrend returns monthly revenue for the last N months.
func (s *Service) GetRevenueTrend(storeID int64, months int) ([]RevenueTrendPoint, error) {
	if months <= 0 {
		months = 12
	}
	result, err := s.repo.GetRevenueTrend(storeID, months)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if result == nil {
		result = []RevenueTrendPoint{}
	}
	return result, nil
}

// GetServiceBreakdown returns service category revenue shares.
func (s *Service) GetServiceBreakdown(storeID int64, start, end time.Time) ([]ServiceBreakdown, error) {
	result, err := s.repo.GetServiceBreakdown(storeID, start, end)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if result == nil {
		result = []ServiceBreakdown{}
	}
	return result, nil
}

// GetPeakHours returns hourly booking/settlement distribution.
func (s *Service) GetPeakHours(storeID int64, start, end time.Time) ([]PeakHourPoint, error) {
	result, err := s.repo.GetPeakHours(storeID, start, end)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if result == nil {
		result = []PeakHourPoint{}
	}
	return result, nil
}

// GetRetentionFunnel returns customer visit frequency distribution.
func (s *Service) GetRetentionFunnel(storeID int64) ([]RetentionBucket, error) {
	result, err := s.repo.GetRetentionFunnel(storeID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if result == nil {
		result = []RetentionBucket{}
	}
	return result, nil
}

// GetReport returns the full analytics report.
func (s *Service) GetReport(storeID int64, start, end time.Time) (*AnalyticsReport, error) {
	trend, _ := s.GetRevenueTrend(storeID, 12)
	breakdown, _ := s.GetServiceBreakdown(storeID, start, end)
	peaks, _ := s.GetPeakHours(storeID, start, end)
	funnel, _ := s.GetRetentionFunnel(storeID)

	return &AnalyticsReport{
		RevenueTrend:     trend,
		ServiceBreakdown: breakdown,
		PeakHours:        peaks,
		RetentionFunnel:  funnel,
	}, nil
}
