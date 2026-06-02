package inventory

import (
	"gorm.io/gorm"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/errcode"
)

// Service handles inventory business logic.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// SaleOut deducts quantity from inventory.
func (s *Service) SaleOut(storeID, productID int64, quantity int, operatorID int64, refType string, refID int64) error {
	inv, err := s.repo.GetInventory(storeID, productID)
	if err != nil {
		if err == gorm.ErrRecordNotFound { return apperr.NotFound("库存记录不存在") }
		return apperr.Internal(err)
	}

	if inv.Quantity < quantity {
		return apperr.New(errcode.InsufficientStock, "库存不足，当前库存: "+itoa(inv.Quantity))
	}

	newQty := inv.Quantity - quantity
	inv.Quantity = newQty
	inv.HasAlert = newQty <= inv.SafetyStock && inv.SafetyStock > 0

	if err := s.repo.UpdateInventory(inv); err != nil {
		return apperr.Internal(err)
	}

	var opID *int64
	if operatorID > 0 { opID = &operatorID }

	tx := &StockTransaction{
		StoreID: storeID, ProductID: productID, Type: TxSaleOut,
		Quantity: -quantity, BalanceAfter: newQty,
		RefType: refType, RefID: refID, OperatorID: opID,
	}
	return s.repo.CreateTransaction(tx)
}

// PurchaseIn adds quantity to inventory.
func (s *Service) PurchaseIn(storeID, productID int64, quantity int, operatorID int64, refType string, refID int64) error {
	inv, err := s.repo.GetInventory(storeID, productID)
	if err != nil {
		if err == gorm.ErrRecordNotFound { return apperr.NotFound("库存记录不存在") }
		return apperr.Internal(err)
	}

	newQty := inv.Quantity + quantity
	inv.Quantity = newQty
	inv.HasAlert = newQty <= inv.SafetyStock && inv.SafetyStock > 0

	if err := s.repo.UpdateInventory(inv); err != nil {
		return apperr.Internal(err)
	}

	var opID *int64
	if operatorID > 0 { opID = &operatorID }

	tx := &StockTransaction{
		StoreID: storeID, ProductID: productID, Type: TxPurchaseIn,
		Quantity: quantity, BalanceAfter: newQty,
		RefType: refType, RefID: refID, OperatorID: opID,
	}
	return s.repo.CreateTransaction(tx)
}

// Adjust adjusts inventory quantity (requires reason).
func (s *Service) Adjust(storeID, productID int64, delta int, operatorID int64, remark string) error {
	if remark == "" {
		return apperr.BadRequest("盘点调整必须填写原因")
	}

	inv, err := s.repo.GetInventory(storeID, productID)
	if err != nil {
		if err == gorm.ErrRecordNotFound { return apperr.NotFound("库存记录不存在") }
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
	if operatorID > 0 { opID = &operatorID }

	tx := &StockTransaction{
		StoreID: storeID, ProductID: productID, Type: TxAdjust,
		Quantity: delta, BalanceAfter: newQty,
		OperatorID: opID, Remark: remark,
	}
	return s.repo.CreateTransaction(tx)
}

func itoa(n int) string {
	if n == 0 { return "0" }
	s := ""
	neg := n < 0
	if neg { n = -n }
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg { s = "-" + s }
	return s
}
