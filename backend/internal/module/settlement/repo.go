package settlement

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

// Repository defines the data access interface for settlements.
type Repository interface {
	FindByID(id int64) (*Settlement, error)
	Create(s *Settlement) error
	Update(s *Settlement) error
	CreateItems(items []SettlementItem) error
	FindItems(settlementID int64) ([]SettlementItem, error)
	CreatePayment(p *Payment) error
	CreateReceipt(storeID, settlementID, operatorID int64, content map[string]interface{}) error
	ListByStore(storeID int64, status string, page, pageSize int) ([]Settlement, int64, error)
}

type repo struct {
	db *gorm.DB
}

var settlementCodeSeq atomic.Uint64

func NewRepository(db *gorm.DB) Repository {
	return &repo{db: db}
}

func (r *repo) FindByID(id int64) (*Settlement, error) {
	var s Settlement
	err := r.db.First(&s, id).Error
	return &s, err
}

func (r *repo) Create(s *Settlement) error {
	return r.db.Create(s).Error
}

func (r *repo) Update(s *Settlement) error {
	return r.db.Save(s).Error
}

func (r *repo) CreateItems(items []SettlementItem) error {
	return r.db.Create(&items).Error
}

func (r *repo) FindItems(settlementID int64) ([]SettlementItem, error) {
	var items []SettlementItem
	err := r.db.Where("settlement_id = ?", settlementID).Find(&items).Error
	return items, err
}

func (r *repo) CreatePayment(p *Payment) error {
	return r.db.Create(p).Error
}

func (r *repo) CreateReceipt(storeID, settlementID, operatorID int64, content map[string]interface{}) error {
	data, err := json.Marshal(content)
	if err != nil {
		return err
	}
	return r.db.Table("print_jobs").Create(map[string]interface{}{
		"store_id":    storeID,
		"type":        "receipt",
		"ref_type":    "settlement",
		"ref_id":      settlementID,
		"content":     json.RawMessage(data),
		"operator_id": printJobOperatorIDValue(operatorID),
		"created_at":  time.Now().UTC(),
	}).Error
}

func printJobOperatorIDValue(operatorID int64) interface{} {
	if operatorID <= 0 {
		return nil
	}
	return &operatorID
}

func (r *repo) ListByStore(storeID int64, status string, page, pageSize int) ([]Settlement, int64, error) {
	var list []Settlement
	var total int64
	q := r.db.Where("store_id = ?", storeID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	q.Model(&Settlement{}).Count(&total)
	offset := (page - 1) * pageSize
	err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&list).Error
	return list, total, err
}

// GenerateCode creates a high-entropy settlement code with the current date prefix.
func GenerateCode() string {
	now := time.Now()
	seq := settlementCodeSeq.Add(1) % 1000
	return fmt.Sprintf("S%s%09d%03d", now.Format("20060102"), now.UnixNano()%1_000_000_000, seq)
}
