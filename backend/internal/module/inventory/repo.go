package inventory

import "gorm.io/gorm"

// Repository defines the data access interface for inventory.
type Repository interface {
	GetInventory(storeID, productID int64) (*InventoryItem, error)
	UpdateInventory(inv *InventoryItem) error
	CreateTransaction(tx *StockTransaction) error
	CheckSafetyStock(storeID int64) ([]InventoryAlert, error)
}

type repo struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repo{db: db}
}

func (r *repo) GetInventory(storeID, productID int64) (*InventoryItem, error) {
	var inv InventoryItem
	err := r.db.Where("store_id = ? AND product_id = ?", storeID, productID).First(&inv).Error
	return &inv, err
}

func (r *repo) UpdateInventory(inv *InventoryItem) error {
	return r.db.Model(&InventoryItem{}).Where("id = ?", inv.ID).Update("quantity", inv.Quantity).Error
}

func (r *repo) CreateTransaction(tx *StockTransaction) error {
	return r.db.Create(tx).Error
}

func (r *repo) CheckSafetyStock(storeID int64) ([]InventoryAlert, error) {
	var alerts []InventoryAlert
	err := r.db.Table("inventory i").
		Select("i.product_id, p.name as product_name, i.quantity, i.safety_stock").
		Joins("JOIN products p ON p.id = i.product_id AND p.deleted_at IS NULL").
		Where("i.store_id = ? AND i.quantity <= i.safety_stock AND i.safety_stock > 0", storeID).
		Scan(&alerts).Error
	return alerts, err
}
