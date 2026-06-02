package settlement

import "time"

// Settlement mirrors the settlements table.
type Settlement struct {
	ID             int64      `gorm:"primaryKey" json:"id"`
	StoreID        int64      `json:"store_id"`
	Code           string     `gorm:"uniqueIndex;size:32" json:"code"`
	CustomerID     *int64     `json:"customer_id"`
	BizType        string     `gorm:"size:16" json:"biz_type"`
	Status         string     `gorm:"size:16;default:unpaid" json:"status"`
	TotalAmount    int64      `json:"total_amount"`
	DiscountAmount int64      `json:"discount_amount"`
	PaidAmount     int64      `json:"paid_amount"`
	OperatorID     *int64     `json:"operator_id"`
	PaidAt         *time.Time `json:"paid_at"`
	Remark         string     `gorm:"size:255" json:"remark"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (Settlement) TableName() string { return "settlements" }

// SettlementItem mirrors settlement_items.
type SettlementItem struct {
	ID           int64  `gorm:"primaryKey" json:"id"`
	SettlementID int64  `json:"settlement_id"`
	SourceType   string `gorm:"size:16" json:"source_type"`
	SourceID     int64  `json:"source_id"`
	Name         string `gorm:"size:128" json:"name"`
	UnitPrice    int64  `json:"unit_price"`
	Quantity     int    `json:"quantity"`
	Amount       int64  `json:"amount"`
}

func (SettlementItem) TableName() string { return "settlement_items" }

// Payment mirrors payments.
type Payment struct {
	ID           int64      `gorm:"primaryKey" json:"id"`
	SettlementID int64      `json:"settlement_id"`
	Method       string     `gorm:"size:16" json:"method"`
	Amount       int64      `json:"amount"`
	Status       string     `gorm:"size:16;default:pending" json:"status"`
	TradeNo      string     `gorm:"size:64" json:"trade_no"`
	PaidAt       *time.Time `json:"paid_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (Payment) TableName() string { return "payments" }

// Statuses
const (
	StatusUnpaid   = "unpaid"
	StatusPaid     = "paid"
	StatusRefunded = "refunded"
	StatusVoid     = "void"
)

// Biz types
const (
	BizService  = "service"
	BizBoarding = "boarding"
	BizRetail   = "retail"
	BizRecharge = "recharge"
	BizMixed    = "mixed"
)

// Payment methods
const (
	PayCash   = "cash"
	PayWallet = "wallet"
	PayPOS    = "pos"
	PayWechat = "wechat"
	PayAlipay = "alipay"
)
