package inventory

import "time"

// InventoryItem mirrors the inventory table.
type InventoryItem struct {
	ID          int64     `gorm:"primaryKey" json:"id"`
	StoreID     int64     `json:"store_id"`
	ProductID   int64     `json:"product_id"`
	Quantity    int       `json:"quantity"`
	SafetyStock int       `json:"safety_stock"`
	UpdatedAt   time.Time `json:"updated_at"`
	HasAlert    bool      `gorm:"-" json:"has_alert"` // computed, not stored
}

func (InventoryItem) TableName() string { return "inventory" }

// StockTransaction mirrors stock_transactions.
type StockTransaction struct {
	ID           int64     `gorm:"primaryKey" json:"id"`
	StoreID      int64     `json:"store_id"`
	ProductID    int64     `json:"product_id"`
	Type         string    `gorm:"size:16" json:"type"`
	Quantity     int       `json:"quantity"`
	BalanceAfter int       `json:"balance_after"`
	RefType      string    `gorm:"size:32" json:"ref_type"`
	RefID        int64     `json:"ref_id"`
	OperatorID   *int64    `json:"operator_id"`
	Remark       string    `gorm:"size:255" json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
}

func (StockTransaction) TableName() string { return "stock_transactions" }

// InventoryAlert is a product below safety stock.
type InventoryAlert struct {
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name"`
	Quantity    int    `json:"quantity"`
	SafetyStock int    `json:"safety_stock"`
}

// Transaction types
const (
	TxPurchaseIn = "purchase_in"
	TxSaleOut    = "sale_out"
	TxServiceOut = "service_out"
	TxAdjust     = "adjust"
	TxTransfer   = "transfer"
)
