package member

// WalletRechargeRequest for POST /customers/:id/wallet.
type WalletRechargeRequest struct {
	Amount     int64  `json:"amount" binding:"required"`
	StoreID    int64  `json:"store_id"`
	OperatorID int64  `json:"operator_id"`
	Remark     string `json:"remark"`
}

// WalletAdjustRequest for PUT /customers/:id/wallet.
type WalletAdjustRequest struct {
	Amount     int64  `json:"amount" binding:"required"`
	OperatorID int64  `json:"operator_id"`
	Remark     string `json:"remark" binding:"required"`
}
