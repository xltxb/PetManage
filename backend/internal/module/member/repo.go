package member

import "gorm.io/gorm"

// Repository defines the data access interface for members.
type Repository interface {
	FindCustomerByID(id int64) (*Customer, error)
	UpdateCustomer(c *Customer) error
	CreateWalletTx(tx *WalletTransaction) error
	CreatePointsTx(tx *PointsTransaction) error
	GetTiers() ([]MembershipTier, error)
	ListCustomers(storeID int64, keyword string, page, pageSize int) ([]Customer, int64, error)
}

type repo struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repo{db: db}
}

func (r *repo) FindCustomerByID(id int64) (*Customer, error) {
	var c Customer
	err := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&c).Error
	return &c, err
}

func (r *repo) UpdateCustomer(c *Customer) error {
	return r.db.Model(&Customer{}).Where("id = ?", c.ID).Updates(customerUpdateFields(c)).Error
}

func customerUpdateFields(c *Customer) map[string]interface{} {
	fields := map[string]interface{}{
		"name":              c.Name,
		"phone":             c.Phone,
		"gender":            c.Gender,
		"tier_id":           c.TierID,
		"wallet_balance":    c.WalletBalance,
		"points_balance":    c.PointsBalance,
		"total_spend":       c.TotalSpend,
		"source":            c.Source,
		"register_store_id": c.RegisterStoreID,
		"last_visit_at":     c.LastVisitAt,
		"note":              c.Note,
	}
	if c.WechatOpenID != "" {
		fields["wechat_openid"] = c.WechatOpenID
	}
	return fields
}

func (r *repo) CreateWalletTx(tx *WalletTransaction) error {
	return r.db.Create(tx).Error
}

func (r *repo) CreatePointsTx(tx *PointsTransaction) error {
	return r.db.Create(tx).Error
}

func (r *repo) GetTiers() ([]MembershipTier, error) {
	var tiers []MembershipTier
	err := r.db.Order("sort ASC").Find(&tiers).Error
	return tiers, err
}

func (r *repo) ListCustomers(storeID int64, keyword string, page, pageSize int) ([]Customer, int64, error) {
	var list []Customer
	var total int64
	q := r.db.Where("deleted_at IS NULL")
	if keyword != "" {
		q = q.Where("name ILIKE ? OR phone ILIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	_ = storeID // global customers
	q.Model(&Customer{}).Count(&total)
	offset := (page - 1) * pageSize
	err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&list).Error
	return list, total, err
}
