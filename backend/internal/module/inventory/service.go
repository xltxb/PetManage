package inventory

import (
	"strconv"

	"gorm.io/gorm"

	"pawprint/backend/internal/module/notification"
	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/errcode"
)

// Service handles inventory business logic.
type Service struct {
	repo     Repository
	notifier Notifier
}

type Notifier interface {
	Send(notification.SendRequest) error
}

type Option func(*Service)

func WithNotifier(n Notifier) Option {
	return func(s *Service) {
		s.notifier = n
	}
}

func NewService(repo Repository, opts ...Option) *Service {
	s := &Service{repo: repo}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// SaleOut deducts quantity from inventory.
func (s *Service) SaleOut(storeID, productID int64, quantity int, operatorID int64, refType string, refID int64) error {
	return s.repo.WithTx(func(txRepo Repository) error {
		inv, err := txRepo.GetInventoryForUpdate(storeID, productID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return apperr.NotFound("库存记录不存在")
			}
			return apperr.Internal(err)
		}

		if inv.Quantity < quantity {
			return apperr.New(errcode.InsufficientStock, "库存不足，当前库存: "+itoa(inv.Quantity))
		}

		newQty := inv.Quantity - quantity
		inv.Quantity = newQty
		inv.HasAlert = newQty <= inv.SafetyStock && inv.SafetyStock > 0

		if err := txRepo.UpdateInventory(inv); err != nil {
			return apperr.Internal(err)
		}

		var opID *int64
		if operatorID > 0 {
			opID = &operatorID
		}

		tx := &StockTransaction{
			StoreID: storeID, ProductID: productID, Type: TxSaleOut,
			Quantity: -quantity, BalanceAfter: newQty,
			RefType: refType, RefID: refID, OperatorID: opID,
		}
		if err := txRepo.CreateTransaction(tx); err != nil {
			return apperr.Internal(err)
		}

		if inv.HasAlert && s.notifier != nil {
			return s.notifier.Send(notification.SendRequest{
				StoreID:      storeID,
				TemplateCode: "stock_low",
				Channel:      notification.ChannelInApp,
				Payload: map[string]string{
					"product_id": strconv.FormatInt(productID, 10),
					"quantity":   strconv.Itoa(newQty),
				},
			})
		}
		return nil
	})
}

// PurchaseIn adds quantity to inventory.
func (s *Service) PurchaseIn(storeID, productID int64, quantity int, operatorID int64, refType string, refID int64) error {
	return s.repo.WithTx(func(txRepo Repository) error {
		inv, err := txRepo.GetInventoryForUpdate(storeID, productID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return apperr.NotFound("库存记录不存在")
			}
			return apperr.Internal(err)
		}

		newQty := inv.Quantity + quantity
		inv.Quantity = newQty
		inv.HasAlert = newQty <= inv.SafetyStock && inv.SafetyStock > 0

		if err := txRepo.UpdateInventory(inv); err != nil {
			return apperr.Internal(err)
		}

		var opID *int64
		if operatorID > 0 {
			opID = &operatorID
		}

		tx := &StockTransaction{
			StoreID: storeID, ProductID: productID, Type: TxPurchaseIn,
			Quantity: quantity, BalanceAfter: newQty,
			RefType: refType, RefID: refID, OperatorID: opID,
		}
		return txRepo.CreateTransaction(tx)
	})
}

// ReverseSaleOut restores stock for a refunded sale.
func (s *Service) ReverseSaleOut(storeID, productID int64, quantity int, operatorID int64, refType string, refID int64) error {
	return s.PurchaseIn(storeID, productID, quantity, operatorID, refType, refID)
}

// Adjust adjusts inventory quantity (requires reason).
func (s *Service) Adjust(storeID, productID int64, delta int, operatorID int64, remark string) error {
	if remark == "" {
		return apperr.BadRequest("盘点调整必须填写原因")
	}

	inv, err := s.repo.GetInventory(storeID, productID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return apperr.NotFound("库存记录不存在")
		}
		return apperr.Internal(err)
	}

	newQty := inv.Quantity + delta
	if newQty < 0 {
		return apperr.New(errcode.InsufficientStock, "调整后库存不可为负")
	}

	inv.Quantity = newQty
	inv.HasAlert = newQty <= inv.SafetyStock && inv.SafetyStock > 0

	if err := s.repo.UpdateInventory(inv); err != nil {
		return apperr.Internal(err)
	}

	var opID *int64
	if operatorID > 0 {
		opID = &operatorID
	}

	tx := &StockTransaction{
		StoreID: storeID, ProductID: productID, Type: TxAdjust,
		Quantity: delta, BalanceAfter: newQty,
		OperatorID: opID, Remark: remark,
	}
	return s.repo.CreateTransaction(tx)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}
