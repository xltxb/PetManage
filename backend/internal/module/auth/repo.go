package auth

import "gorm.io/gorm"

// Repository defines the data access interface for auth.
type Repository interface {
	FindUserByUsername(username string) (*User, error)
	FindUserByID(id int64) (*User, error)
	FindUserStores(userID int64) ([]StoreInfo, error)
	FindUserPermissions(userID int64, storeID int64) ([]string, error)
	FindUserStoreRole(userID, storeID int64) (*UserStoreRole, error)
	UpdateLastStore(userID, storeID int64) error
}

type repo struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repo{db: db}
}

func (r *repo) FindUserByUsername(username string) (*User, error) {
	var u User
	err := r.db.Where("username = ? AND status = 1", username).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *repo) FindUserByID(id int64) (*User, error) {
	var u User
	err := r.db.Where("id = ? AND status = 1", id).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *repo) FindUserStores(userID int64) ([]StoreInfo, error) {
	var stores []StoreInfo
	err := r.db.Table("user_store_roles usr").
		Select("s.id, s.name, r.code as role").
		Joins("JOIN stores s ON s.id = usr.store_id AND s.deleted_at IS NULL AND s.status = 1").
		Joins("JOIN roles r ON r.id = usr.role_id").
		Where("usr.user_id = ?", userID).
		Scan(&stores).Error
	return stores, err
}

func (r *repo) FindUserPermissions(userID int64, storeID int64) ([]string, error) {
	var perms []string
	err := r.db.Table("user_store_roles usr").
		Select("DISTINCT p.code").
		Joins("JOIN role_permissions rp ON rp.role_id = usr.role_id").
		Joins("JOIN permissions p ON p.id = rp.permission_id").
		Where("usr.user_id = ? AND usr.store_id = ?", userID, storeID).
		Pluck("p.code", &perms).Error
	return perms, err
}

func (r *repo) FindUserStoreRole(userID, storeID int64) (*UserStoreRole, error) {
	var usr UserStoreRole
	err := r.db.Preload("Role").
		Where("user_id = ? AND store_id = ?", userID, storeID).
		First(&usr).Error
	if err != nil {
		return nil, err
	}
	return &usr, nil
}

func (r *repo) UpdateLastStore(userID, storeID int64) error {
	return r.db.Model(&User{}).Where("id = ?", userID).
		Update("last_store_id", storeID).Error
}
