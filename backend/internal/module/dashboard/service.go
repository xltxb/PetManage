package dashboard

import (
	"time"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/errcode"
)

// Service handles dashboard business logic.
type Service struct {
	repo Repository
	tz   string
}

func NewService(repo Repository, timezone string) *Service {
	return &Service{repo: repo, tz: timezone}
}

// GetSummary returns the complete dashboard summary for a store.
func (s *Service) GetSummary(storeID int64) (*DashboardSummary, error) {
	if storeID <= 0 {
		return nil, apperr.BadRequest("缺少门店ID")
	}

	var err error
	summary := &DashboardSummary{}

	summary.RevenueToday, err = s.repo.GetRevenueToday(storeID, s.tz)
	if err != nil {
		return nil, apperr.Wrap(err, errcode.InternalError)
	}
	summary.AppointmentCount, err = s.repo.GetAppointmentCountToday(storeID, s.tz)
	if err != nil {
		return nil, apperr.Wrap(err, errcode.InternalError)
	}
	summary.PetsInStore, err = s.repo.GetPetsInStore(storeID, s.tz)
	if err != nil {
		return nil, apperr.Wrap(err, errcode.InternalError)
	}
	summary.NewMembersCount, err = s.repo.GetNewMembersToday(storeID, s.tz)
	if err != nil {
		return nil, apperr.Wrap(err, errcode.InternalError)
	}
	summary.RevenueTrend, err = s.repo.GetRevenueTrend(storeID, 14, s.tz)
	if err != nil {
		return nil, apperr.Wrap(err, errcode.InternalError)
	}
	summary.TodayAppointments, err = s.repo.GetTodayAppointments(storeID, s.tz)
	if err != nil {
		return nil, apperr.Wrap(err, errcode.InternalError)
	}
	summary.PopularServices, err = s.repo.GetPopularServices(storeID, time.Now())
	if err != nil {
		return nil, apperr.Wrap(err, errcode.InternalError)
	}
	summary.InventoryAlerts, err = s.repo.GetInventoryAlerts(storeID)
	if err != nil {
		return nil, apperr.Wrap(err, errcode.InternalError)
	}
	summary.MemberComposition, err = s.repo.GetMemberComposition(storeID)
	if err != nil {
		return nil, apperr.Wrap(err, errcode.InternalError)
	}

	// Ensure non-nil slices for JSON output
	if summary.RevenueTrend == nil {
		summary.RevenueTrend = []RevenuePoint{}
	}
	if summary.TodayAppointments == nil {
		summary.TodayAppointments = []AppointmentTimelineItem{}
	}
	if summary.PopularServices == nil {
		summary.PopularServices = []ServiceRank{}
	}
	if summary.InventoryAlerts == nil {
		summary.InventoryAlerts = []InventoryAlert{}
	}
	if summary.MemberComposition == nil {
		summary.MemberComposition = []TierComposition{}
	}

	return summary, nil
}
