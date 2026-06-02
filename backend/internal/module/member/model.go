package member

import "time"

// Customer mirrors the customers table.
type Customer struct {
	ID               int64      `gorm:"primaryKey" json:"id"`
	Name             string     `gorm:"size:64" json:"name"`
	Phone            string     `gorm:"uniqueIndex;size:20" json:"phone"`
	Gender           int16      `json:"gender"`
	TierID           int64      `json:"tier_id"`
	WalletBalance    int64      `json:"wallet_balance"`
	PointsBalance    int64      `json:"points_balance"`
	TotalSpend       int64      `json:"total_spend"`
	Source           int16      `json:"source"`
	WechatOpenID     string     `gorm:"size:64" json:"-"`
	RegisterStoreID  *int64     `json:"register_store_id"`
	LastVisitAt      *time.Time `json:"last_visit_at"`
	Note             string     `gorm:"type:text" json:"note"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `gorm:"index" json:"-"`
}

func (Customer) TableName() string { return "customers" }

// WalletTransaction mirrors wallet_transactions.
type WalletTransaction struct {
	ID           int64     `gorm:"primaryKey" json:"id"`
	CustomerID   int64     `json:"customer_id"`
	StoreID      int64     `json:"store_id"`
	Type         string    `gorm:"size:16" json:"type"`
	Amount       int64     `json:"amount"`
	BalanceAfter int64     `json:"balance_after"`
	RefType      string    `gorm:"size:32" json:"ref_type"`
	RefID        int64     `json:"ref_id"`
	OperatorID   *int64    `json:"operator_id"`
	Remark       string    `gorm:"size:255" json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
}

func (WalletTransaction) TableName() string { return "wallet_transactions" }

// PointsTransaction mirrors points_transactions.
type PointsTransaction struct {
	ID           int64     `gorm:"primaryKey" json:"id"`
	CustomerID   int64     `json:"customer_id"`
	StoreID      int64     `json:"store_id"`
	Type         string    `gorm:"size:16" json:"type"`
	Amount       int64     `json:"amount"`
	BalanceAfter int64     `json:"balance_after"`
	RefType      string    `gorm:"size:32" json:"ref_type"`
	RefID        int64     `json:"ref_id"`
	OperatorID   *int64    `json:"operator_id"`
	Remark       string    `gorm:"size:255" json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
}

func (PointsTransaction) TableName() string { return "points_transactions" }

// MembershipTier mirrors membership_tiers.
type MembershipTier struct {
	ID             int64   `gorm:"primaryKey" json:"id"`
	Code           string  `gorm:"size:16" json:"code"`
	Name           string  `gorm:"size:32" json:"name"`
	MinTotalSpend  int64   `json:"min_total_spend"`
	DiscountRate   int16   `json:"discount_rate"`
	PointsRate     float64 `json:"points_rate"`
	Sort           int16   `json:"sort"`
}

func (MembershipTier) TableName() string { return "membership_tiers" }

// Transaction types
const (
	TxRecharge = "recharge"
	TxConsume  = "consume"
	TxRefund   = "refund"
	TxAdjust   = "adjust"
	TxEarn     = "earn"
	TxRedeem   = "redeem"
	TxExpire   = "expire"
)
