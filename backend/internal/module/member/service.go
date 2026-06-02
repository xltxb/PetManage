package member

import (
	"math"

	"gorm.io/gorm"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/errcode"
)

// Service handles member business logic.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Recharge adds stored value to a customer's wallet.
func (s *Service) Recharge(customerID, amount, storeID, operatorID int64, remark string) error {
	c, err := s.repo.FindCustomerByID(customerID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return apperr.NotFound("会员不存在")
		}
		return apperr.Internal(err)
	}

	newBalance := c.WalletBalance + amount
	c.WalletBalance = newBalance

	if err := s.repo.UpdateCustomer(c); err != nil {
		return apperr.Internal(err)
	}

	var opID *int64
	if operatorID > 0 {
		opID = &operatorID
	}

	tx := &WalletTransaction{
		CustomerID: customerID, StoreID: storeID, Type: TxRecharge,
		Amount: amount, BalanceAfter: newBalance,
		OperatorID: opID, Remark: remark,
	}
	return s.repo.CreateWalletTx(tx)
}

// WalletConsume deducts from the wallet balance.
func (s *Service) WalletConsume(customerID, amount, storeID, operatorID int64, remark string) error {
	c, err := s.repo.FindCustomerByID(customerID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return apperr.NotFound("会员不存在")
		}
		return apperr.Internal(err)
	}

	if c.WalletBalance < amount {
		return apperr.New(errcode.InsufficientWallet, "储值余额不足")
	}

	newBalance := c.WalletBalance - amount
	c.WalletBalance = newBalance

	if err := s.repo.UpdateCustomer(c); err != nil {
		return apperr.Internal(err)
	}

	var opID *int64
	if operatorID > 0 {
		opID = &operatorID
	}

	tx := &WalletTransaction{
		CustomerID: customerID, StoreID: storeID, Type: TxConsume,
		Amount: -amount, BalanceAfter: newBalance,
		OperatorID: opID, Remark: remark,
	}
	return s.repo.CreateWalletTx(tx)
}

// WalletAdjust manually adjusts wallet balance (requires reason).
func (s *Service) WalletAdjust(customerID, amount, storeID, operatorID int64, remark string) error {
	if remark == "" {
		return apperr.BadRequest("人工调整必须填写原因")
	}

	c, err := s.repo.FindCustomerByID(customerID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return apperr.NotFound("会员不存在")
		}
		return apperr.Internal(err)
	}

	newBalance := c.WalletBalance + amount
	c.WalletBalance = newBalance

	if err := s.repo.UpdateCustomer(c); err != nil {
		return apperr.Internal(err)
	}

	var opID *int64
	if operatorID > 0 {
		opID = &operatorID
	}

	tx := &WalletTransaction{
		CustomerID: customerID, StoreID: storeID, Type: TxAdjust,
		Amount: amount, BalanceAfter: newBalance,
		OperatorID: opID, Remark: remark,
	}
	return s.repo.CreateWalletTx(tx)
}

// EarnPoints calculates and awards points for a successful payment.
// amountPaid is in cents (分). Returns points earned.
// Rule: points = floor(amountPaid/100 * tier.points_rate). Recharge doesn't earn points.
func (s *Service) EarnPoints(customerID, amountPaid, storeID, operatorID int64, refType string, refID int64) (int64, error) {
	c, err := s.repo.FindCustomerByID(customerID)
	if err != nil {
		return 0, nil // silently skip for non-existent members
	}

	tiers, err := s.repo.GetTiers()
	if err != nil {
		return 0, apperr.Internal(err)
	}

	var tier *MembershipTier
	for i := range tiers {
		if tiers[i].ID == c.TierID {
			tier = &tiers[i]
			break
		}
	}
	if tier == nil {
		tier = &tiers[0] // default to first tier
	}

	yuan := float64(amountPaid) / 100.0
	pts := int64(math.Floor(yuan * tier.PointsRate))

	if pts <= 0 {
		return 0, nil
	}

	newBalance := c.PointsBalance + pts
	c.PointsBalance = newBalance
	_ = s.repo.UpdateCustomer(c)

	var opID *int64
	if operatorID > 0 {
		opID = &operatorID
	}

	ptx := &PointsTransaction{
		CustomerID: customerID, StoreID: storeID, Type: TxEarn,
		Amount: pts, BalanceAfter: newBalance,
		RefType: refType, RefID: refID, OperatorID: opID,
	}
	if err := s.repo.CreatePointsTx(ptx); err != nil {
		return 0, apperr.Internal(err)
	}

	return pts, nil
}

// CheckTierUpgrade checks if total_spend qualifies for a higher tier.
// Returns (upgraded bool, new tier code string).
func (s *Service) CheckTierUpgrade(customerID int64, totalSpend int64) (bool, string) {
	c, err := s.repo.FindCustomerByID(customerID)
	if err != nil {
		return false, ""
	}

	tiers, err := s.repo.GetTiers()
	if err != nil {
		return false, ""
	}

	// Find the highest tier the customer qualifies for
	var bestTier *MembershipTier
	for i := range tiers {
		if totalSpend >= tiers[i].MinTotalSpend {
			bestTier = &tiers[i]
		}
	}

	if bestTier == nil {
		return false, ""
	}

	// Only upgrade (no downgrade per member.allow_downgrade=false)
	if bestTier.ID > c.TierID {
		c.TierID = bestTier.ID
		c.TotalSpend = totalSpend
		_ = s.repo.UpdateCustomer(c)
		return true, bestTier.Code
	}

	return false, ""
}

// ApplyPaidSpend increases total spend after a paid settlement and upgrades tier when eligible.
func (s *Service) ApplyPaidSpend(customerID, amountPaid, storeID, operatorID int64, refType string, refID int64) error {
	c, err := s.repo.FindCustomerByID(customerID)
	if err != nil {
		return nil
	}
	c.TotalSpend += amountPaid
	if err := s.repo.UpdateCustomer(c); err != nil {
		return apperr.Internal(err)
	}
	s.CheckTierUpgrade(customerID, c.TotalSpend)
	return nil
}

// ReverseSettlement reverses coarse member totals for a refunded settlement.
func (s *Service) ReverseSettlement(customerID, amountPaid, storeID, operatorID int64, refID int64) error {
	c, err := s.repo.FindCustomerByID(customerID)
	if err != nil {
		return nil
	}
	if c.TotalSpend >= amountPaid {
		c.TotalSpend -= amountPaid
	}
	return s.repo.UpdateCustomer(c)
}

// GetCustomer returns a customer by ID.
func (s *Service) GetCustomer(id int64) (*Customer, error) {
	c, err := s.repo.FindCustomerByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperr.NotFound("会员不存在")
		}
		return nil, apperr.Internal(err)
	}
	return c, nil
}

// ListCustomers returns a paginated list of customers.
func (s *Service) ListCustomers(storeID int64, keyword string, page, pageSize int) ([]Customer, int64, error) {
	return s.repo.ListCustomers(storeID, keyword, page, pageSize)
}
