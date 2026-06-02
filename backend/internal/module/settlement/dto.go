package settlement

// CreateSettlementRequest is the POST /settlements body.
type CreateSettlementRequest struct {
	StoreID    int64                  `json:"store_id"`
	CustomerID int64                  `json:"customer_id"`
	BizType    string                 `json:"biz_type" binding:"required"`
	Items      []SettlementItemRequest `json:"items" binding:"required"`
	Remark     string                 `json:"remark"`
}

// SettlementItemRequest is an item in the settlement request.
type SettlementItemRequest struct {
	SourceType string `json:"source_type" binding:"required"`
	SourceID   int64  `json:"source_id"`
	Name       string `json:"name" binding:"required"`
	UnitPrice  int64  `json:"unit_price" binding:"required"`
	Quantity   int    `json:"quantity"`
}

// PayRequest is the POST /settlements/:id/pay body.
type PayRequest struct {
	Method     string `json:"method" binding:"required"`
	Amount     int64  `json:"amount" binding:"required"`
	OperatorID int64  `json:"operator_id"`
}

// RefundRequest is the POST /settlements/:id/refund body.
type RefundRequest struct {
	OperatorID int64  `json:"operator_id"`
	Reason     string `json:"reason" binding:"required"`
}
