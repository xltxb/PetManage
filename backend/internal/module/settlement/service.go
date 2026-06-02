package settlement

import (
	"time"

	"gorm.io/gorm"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/errcode"
)

// Service handles settlement business logic.
type Service struct {
	repo      Repository
	members   MemberEffects
	inventory InventoryEffects
	prints    PrintJobs
}

type MemberEffects interface {
	WalletConsume(customerID, amount, storeID, operatorID int64, remark string) error
	EarnPoints(customerID, amountPaid, storeID, operatorID int64, refType string, refID int64) (int64, error)
	ApplyPaidSpend(customerID, amountPaid, storeID, operatorID int64, refType string, refID int64) error
	ReverseSettlement(customerID, amountPaid, storeID, operatorID int64, refID int64) error
}

type InventoryEffects interface {
	SaleOut(storeID, productID int64, quantity int, operatorID int64, refType string, refID int64) error
	ReverseSaleOut(storeID, productID int64, quantity int, operatorID int64, refType string, refID int64) error
}

type PrintJobs interface {
	CreateReceipt(storeID, settlementID, operatorID int64, content map[string]interface{}) error
}

type Option func(*Service)

func WithMemberEffects(e MemberEffects) Option {
	return func(s *Service) { s.members = e }
}

func WithInventoryEffects(e InventoryEffects) Option {
	return func(s *Service) { s.inventory = e }
}

func WithPrintJobs(p PrintJobs) Option {
	return func(s *Service) { s.prints = p }
}

func NewService(repo Repository, opts ...Option) *Service {
	s := &Service{repo: repo}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Create creates a new settlement with items.
func (s *Service) Create(req CreateSettlementRequest) (*Settlement, error) {
	totalAmount := int64(0)
	for _, item := range req.Items {
		qty := item.Quantity
		if qty == 0 {
			qty = 1
		}
		totalAmount += item.UnitPrice * int64(qty)
	}

	var customerID *int64
	if req.CustomerID > 0 {
		customerID = &req.CustomerID
	}

	settlement := &Settlement{
		StoreID:     req.StoreID,
		Code:        GenerateCode(),
		CustomerID:  customerID,
		BizType:     req.BizType,
		Status:      StatusUnpaid,
		TotalAmount: totalAmount,
		Remark:      req.Remark,
	}

	if err := s.repo.Create(settlement); err != nil {
		return nil, apperr.Internal(err)
	}

	items := make([]SettlementItem, len(req.Items))
	for i, item := range req.Items {
		qty := item.Quantity
		if qty == 0 {
			qty = 1
		}
		items[i] = SettlementItem{
			SettlementID: settlement.ID,
			SourceType:   item.SourceType,
			SourceID:     item.SourceID,
			Name:         item.Name,
			UnitPrice:    item.UnitPrice,
			Quantity:     qty,
			Amount:       item.UnitPrice * int64(qty),
		}
	}
	if err := s.repo.CreateItems(items); err != nil {
		return nil, apperr.Internal(err)
	}

	return settlement, nil
}

// Pay processes payment for a settlement.
func (s *Service) Pay(id int64, amount int64, method string, operatorID int64) error {
	settlement, err := s.repo.FindByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return apperr.NotFound("结算单不存在")
		}
		return apperr.Internal(err)
	}

	if settlement.Status != StatusUnpaid {
		return apperr.New(errcode.StateTransitionInvalid, "仅可对未支付结算单进行收款")
	}

	// Online payment methods return not-enabled error (dev doc §11)
	if method == PayWechat || method == PayAlipay {
		return apperr.New(errcode.PaymentNotEnabled, "线上支付未开通，请选择其他方式")
	}

	if method == PayWallet && settlement.CustomerID != nil && s.members != nil {
		if err := s.members.WalletConsume(*settlement.CustomerID, amount, settlement.StoreID, operatorID, "结算消费 "+settlement.Code); err != nil {
			return err
		}
	}

	items, err := s.repo.FindItems(id)
	if err != nil {
		return apperr.Internal(err)
	}
	for _, item := range items {
		if item.SourceType == "product" && s.inventory != nil {
			if err := s.inventory.SaleOut(settlement.StoreID, item.SourceID, item.Quantity, operatorID, "settlement", settlement.ID); err != nil {
				return err
			}
		}
	}

	now := time.Now().UTC()
	settlement.Status = StatusPaid
	settlement.PaidAmount = amount
	settlement.PaidAt = &now

	if err := s.repo.Update(settlement); err != nil {
		return apperr.Internal(err)
	}

	payment := &Payment{
		SettlementID: id,
		Method:       method,
		Amount:       amount,
		Status:       "success",
		PaidAt:       &now,
	}
	if err := s.repo.CreatePayment(payment); err != nil {
		return apperr.Internal(err)
	}

	if settlement.CustomerID != nil && s.members != nil {
		if err := s.members.ApplyPaidSpend(*settlement.CustomerID, amount, settlement.StoreID, operatorID, "settlement", settlement.ID); err != nil {
			return err
		}
		if _, err := s.members.EarnPoints(*settlement.CustomerID, amount, settlement.StoreID, operatorID, "settlement", settlement.ID); err != nil {
			return err
		}
	}

	if s.prints != nil {
		return s.prints.CreateReceipt(settlement.StoreID, settlement.ID, operatorID, map[string]interface{}{
			"code":        settlement.Code,
			"paid_amount": amount,
			"method":      method,
		})
	}
	return nil
}

// Refund refunds a paid settlement and creates a red-ink reversal.
func (s *Service) Refund(id int64, operatorID int64, reason string) error {
	settlement, err := s.repo.FindByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return apperr.NotFound("结算单不存在")
		}
		return apperr.Internal(err)
	}

	if settlement.Status != StatusPaid {
		return apperr.New(errcode.StateTransitionInvalid, "仅可对已支付结算单进行退款")
	}

	settlement.Status = StatusRefunded
	if err := s.repo.Update(settlement); err != nil {
		return apperr.Internal(err)
	}

	// Create red-ink reversal settlement
	reversal := &Settlement{
		StoreID:     settlement.StoreID,
		Code:        GenerateCode(),
		CustomerID:  settlement.CustomerID,
		BizType:     settlement.BizType,
		Status:      StatusPaid,
		TotalAmount: 0,
		PaidAmount:  -settlement.PaidAmount,
		Remark:      "退款: " + reason + " (原单: " + settlement.Code + ")",
	}
	var redOpID *int64
	if operatorID > 0 {
		redOpID = &operatorID
	}
	reversal.OperatorID = redOpID
	now := time.Now().UTC()
	reversal.PaidAt = &now

	return s.repo.Create(reversal)
}

// Void voids an unpaid settlement.
func (s *Service) Void(id int64) error {
	settlement, err := s.repo.FindByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return apperr.NotFound("结算单不存在")
		}
		return apperr.Internal(err)
	}

	if settlement.Status != StatusUnpaid {
		return apperr.New(errcode.StateTransitionInvalid, "仅可作废未支付的结算单")
	}

	settlement.Status = StatusVoid
	return s.repo.Update(settlement)
}
