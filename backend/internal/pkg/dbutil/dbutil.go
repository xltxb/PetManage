package dbutil

import "gorm.io/gorm"

// TxFunc is a function that runs within a database transaction.
type TxFunc func(tx *gorm.DB) error

// WithTransaction executes fn within a new DB transaction.
// Rolls back on error, commits on success.
func WithTransaction(db *gorm.DB, fn TxFunc) error {
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}
