package auth

// LoginRequest is the POST /auth/login body.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse is returned on successful login.
type LoginResponse struct {
	AccessToken  string      `json:"access"`
	RefreshToken string      `json:"refresh"`
	Stores       []StoreInfo `json:"stores"`
}

// StoreInfo is a store the user has access to, with their role.
type StoreInfo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// RefreshRequest is the POST /auth/refresh body.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// SwitchStoreRequest is the POST /auth/switch-store body.
type SwitchStoreRequest struct {
	StoreID int64 `json:"store_id" binding:"required"`
}

// SwitchStoreResponse returns new tokens scoped to a store.
type SwitchStoreResponse struct {
	AccessToken  string `json:"access"`
	RefreshToken string `json:"refresh"`
}
