package boarding

import (
	"errors"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"

	"pawprint/backend/internal/module/settlement"
	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/errcode"
)

// Service handles boarding business logic.
type Service struct {
	repo        Repository
	settlements SettlementCreator
}

type SettlementCreator interface {
	Create(settlement.CreateSettlementRequest) (*settlement.Settlement, error)
}

type Option func(*Service)

func WithSettlementCreator(c SettlementCreator) Option {
	return func(s *Service) {
		s.settlements = c
	}
}

func NewService(repo Repository, opts ...Option) *Service {
	s := &Service{repo: repo}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// CalculateNights computes the number of billable nights.
// Formula: ceil((checkOut - checkIn) / 24h), minimum 1 night.
func CalculateNights(checkIn, checkOut time.Time) int {
	duration := checkOut.Sub(checkIn)
	if duration <= 0 {
		return 1
	}
	hours := duration.Hours()
	nights := int(math.Ceil(hours / 24.0))
	if nights < 1 {
		nights = 1
	}
	return nights
}

// CheckIn creates a boarding order and marks the room as occupied.
func (s *Service) CheckIn(req CheckInRequest) (*BoardingOrder, error) {
	room, err := s.repo.FindRoomByID(req.RoomID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("笼位不存在")
		}
		return nil, apperr.Internal(err)
	}

	if room.Status != RoomStatusFree {
		return nil, apperr.New(errcode.ResourceConflict, "该笼位不可用（当前状态: "+roomStatusLabel(room.Status)+"）")
	}

	now := time.Now().UTC()
	source := req.Source
	if source == 0 {
		source = 1
	}

	order := &BoardingOrder{
		StoreID:          req.StoreID,
		CustomerID:       req.CustomerID,
		PetID:            req.PetID,
		RoomID:           &req.RoomID,
		RoomTypeSnapshot: req.RoomTypeCode,
		PricePerNight:    req.PricePerNight,
		Status:           StatusCheckedIn,
		Source:           source,
		PlannedCheckIn:   req.PlannedCheckIn,
		PlannedCheckOut:  req.PlannedCheckOut,
		ActualCheckIn:    &now,
		Remark:           req.Remark,
	}

	if err := s.repo.CreateOrder(order); err != nil {
		return nil, apperr.Internal(err)
	}

	// Mark room as occupied
	room.Status = RoomStatusOccupied
	if err := s.repo.UpdateRoom(room); err != nil {
		return nil, apperr.Internal(err)
	}

	return order, nil
}

// CheckOut completes a boarding order and calculates billing.
func (s *Service) CheckOut(id, storeID int64) (*CheckOutResponse, error) {
	var response CheckOutResponse
	err := s.repo.WithTx(func(txRepo Repository) error {
		order, err := txRepo.FindOrderByID(id, storeID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperr.NotFound("寄养订单不存在")
			}
			return apperr.Internal(err)
		}

		if order.Status != StatusCheckedIn {
			return apperr.New(errcode.StateTransitionInvalid, "仅可在住状态的订单办理退房")
		}

		now := time.Now().UTC()
		nights := CalculateNights(*order.ActualCheckIn, now)
		totalAmount := int64(nights) * order.PricePerNight

		order.Status = StatusCheckedOut
		order.ActualCheckOut = &now
		order.Nights = &nights
		order.TotalAmount = &totalAmount

		if s.settlements != nil {
			created, err := s.settlements.Create(settlement.CreateSettlementRequest{
				StoreID:    order.StoreID,
				CustomerID: order.CustomerID,
				BizType:    settlement.BizBoarding,
				Items: []settlement.SettlementItemRequest{{
					SourceType: "boarding",
					SourceID:   order.ID,
					Name:       "寄养服务",
					UnitPrice:  order.PricePerNight,
					Quantity:   nights,
				}},
			})
			if err != nil {
				return err
			}
			order.SettlementID = &created.ID
		}

		if err := txRepo.UpdateOrder(order); err != nil {
			return apperr.Internal(err)
		}

		// Mark room as cleaning
		if order.RoomID != nil {
			room, err := txRepo.FindRoomByID(*order.RoomID)
			if err == nil {
				room.Status = RoomStatusCleaning
				if err := txRepo.UpdateRoom(room); err != nil {
					return apperr.Internal(err)
				}
			}
		}

		response = CheckOutResponse{
			Order:       order,
			Nights:      nights,
			TotalAmount: totalAmount,
		}
		return nil
	})
	if err != nil {
		if appErr, ok := err.(*apperr.AppError); ok {
			return nil, appErr
		}
		return nil, apperr.Internal(err)
	}
	return &response, nil
}

func (s *Service) Cancel(id, storeID, operatorID int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return apperr.BadRequest("异常取消必须填写原因")
	}

	err := s.repo.WithTx(func(txRepo Repository) error {
		order, err := txRepo.FindOrderByID(id, storeID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperr.NotFound("寄养订单不存在")
			}
			return apperr.Internal(err)
		}

		if order.Status != StatusCheckedIn {
			return apperr.New(errcode.StateTransitionInvalid, "仅可取消在住状态的寄养订单")
		}

		order.Status = StatusCancelled
		if order.Remark == "" {
			order.Remark = "异常取消: " + reason
		} else {
			order.Remark += "\n异常取消: " + reason
		}

		if err := txRepo.UpdateOrder(order); err != nil {
			return apperr.Internal(err)
		}

		if order.RoomID != nil {
			room, err := txRepo.FindRoomByID(*order.RoomID)
			if err == nil {
				room.Status = RoomStatusFree
				if err := txRepo.UpdateRoom(room); err != nil {
					return apperr.Internal(err)
				}
			}
		}

		return nil
	})
	if err != nil {
		if appErr, ok := err.(*apperr.AppError); ok {
			return appErr
		}
		return apperr.Internal(err)
	}
	return nil
}

// LogCare records a care task for a boarding order.
func (s *Service) LogCare(orderID, storeID int64, task, status, note string, operatorID int64) error {
	order, err := s.repo.FindOrderByID(orderID, storeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.NotFound("寄养订单不存在")
		}
		return apperr.Internal(err)
	}

	if order.Status != StatusCheckedIn {
		return apperr.New(errcode.StateTransitionInvalid, "仅可对在住订单登记照护记录")
	}

	var doneAt *time.Time
	if status == CareStatusDone {
		now := time.Now().UTC()
		doneAt = &now
	}

	var opID *int64
	if operatorID > 0 {
		opID = &operatorID
	}

	cl := &CareLog{
		BoardingOrderID: orderID,
		StoreID:         storeID,
		Task:            task,
		Status:          status,
		DoneAt:          doneAt,
		OperatorID:      opID,
		Note:            note,
		LogDate:         time.Now().UTC(),
	}

	return s.repo.CreateCareLog(cl)
}

// GetOrder returns a boarding order by ID with store validation.
func (s *Service) GetOrder(id, storeID int64) (*BoardingOrder, error) {
	return s.repo.FindOrderByID(id, storeID)
}

// ListOrders returns paginated boarding orders for a store.
func (s *Service) ListOrders(storeID int64, status string, page, pageSize int) ([]BoardingOrder, int64, error) {
	return s.repo.ListOrders(storeID, status, page, pageSize)
}

// GetCareLogs returns care logs for an order.
func (s *Service) GetCareLogs(orderID int64) ([]CareLog, error) {
	return s.repo.FindCareLogs(orderID, time.Now())
}

func roomStatusLabel(s string) string {
	switch s {
	case RoomStatusFree:
		return "空闲"
	case RoomStatusOccupied:
		return "已占用"
	case RoomStatusCleaning:
		return "清洁中"
	case RoomStatusMaintenance:
		return "维护中"
	default:
		return s
	}
}
