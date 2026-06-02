package auth

import "time"

// User mirrors the users table.
type User struct {
	ID               int64      `gorm:"primaryKey" json:"id"`
	Username         string     `gorm:"uniqueIndex;size:64" json:"username"`
	PasswordHash     string     `gorm:"size:255" json:"-"`
	DisplayName      string     `gorm:"size:64" json:"display_name"`
	Phone            string     `gorm:"uniqueIndex;size:20" json:"phone"`
	AvatarText       string     `gorm:"size:4" json:"avatar_text"`
	Status           int16      `json:"status"`
	LastStoreID      *int64     `json:"last_store_id"`
	FailedLoginCount int        `json:"-"`
	LockedUntil      *time.Time `json:"-"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `gorm:"index" json:"-"`
}

func (User) TableName() string { return "users" }

// Role mirrors the roles table.
type Role struct {
	ID       int64  `gorm:"primaryKey" json:"id"`
	Code     string `gorm:"uniqueIndex;size:32" json:"code"`
	Name     string `gorm:"size:32" json:"name"`
	IsSystem bool   `json:"is_system"`
}

func (Role) TableName() string { return "roles" }

// UserStoreRole mirrors user_store_roles.
type UserStoreRole struct {
	ID      int64 `gorm:"primaryKey" json:"id"`
	UserID  int64 `json:"user_id"`
	StoreID int64 `json:"store_id"`
	RoleID  int64 `json:"role_id"`
	Role    Role  `gorm:"foreignKey:RoleID" json:"role"`
}

func (UserStoreRole) TableName() string { return "user_store_roles" }

// Permission mirrors the permissions table.
type Permission struct {
	ID     int64  `gorm:"primaryKey" json:"id"`
	Code   string `gorm:"uniqueIndex;size:64" json:"code"`
	Module string `gorm:"size:32" json:"module"`
	Name   string `gorm:"size:64" json:"name"`
}

func (Permission) TableName() string { return "permissions" }

// Store mirrors the stores table.
type Store struct {
	ID   int64  `gorm:"primaryKey" json:"id"`
	Code string `gorm:"uniqueIndex;size:32" json:"code"`
	Name string `gorm:"size:64" json:"name"`
}

func (Store) TableName() string { return "stores" }
