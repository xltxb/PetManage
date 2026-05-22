package checkout

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// CheckoutItem represents a single item in the checkout.
type CheckoutItem struct {
	ProductID int64  `json:"product_id"`
	SkuID     *int64 `json:"sku_id"`
	Quantity  int    `json:"quantity"`
}

// CheckoutPayment represents a payment in the checkout.
type CheckoutPayment struct {
	Method     string `json:"method"`
	AmountCents int   `json:"amount_cents"`
}

// CheckoutRequest is the request body for creating an order.
type CheckoutRequest struct {
	MemberID *int64            `json:"member_id"`
	Items    []CheckoutItem    `json:"items"`
	Payments []CheckoutPayment `json:"payments"`
}

// CheckoutResponse is the response after a successful checkout.
type CheckoutResponse struct {
	OrderID    int64              `json:"order_id"`
	MerchantID int64              `json:"merchant_id"`
	TotalCents int                `json:"total_cents"`
	PaidCents  int                `json:"paid_cents"`
	Status     string             `json:"status"`
	Items      []OrderItemDetail  `json:"items"`
	Payments   []PaymentDetail    `json:"payments"`
	CreatedAt  time.Time          `json:"created_at"`
}

// OrderItemDetail is an item in the order response.
type OrderItemDetail struct {
	ProductID   int64             `json:"product_id"`
	ProductName string            `json:"product_name"`
	SkuID       *int64            `json:"sku_id,omitempty"`
	SkuSpecInfo map[string]string `json:"sku_spec_info,omitempty"`
	PriceCents  int               `json:"price_cents"`
	Quantity    int               `json:"quantity"`
}

// PaymentDetail is a payment in the order response.
type PaymentDetail struct {
	Method     string `json:"method"`
	AmountCents int   `json:"amount_cents"`
}

// Service provides checkout operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new checkout Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Checkout creates an order, deducts inventory, records payments and stock flows.
func (s *Service) Checkout(ctx context.Context, merchantID int64, req CheckoutRequest) (*CheckoutResponse, error) {
	if len(req.Items) == 0 {
		return nil, apperrors.NewValidationError("at least one item is required")
	}
	if len(req.Payments) == 0 {
		return nil, apperrors.NewValidationError("at least one payment is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	// Calculate total and validate products/SKUs.
	var totalCents int
	var itemDetails []OrderItemDetail
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			return nil, apperrors.NewValidationError("quantity must be positive")
		}

		var priceCents int
		var name string
		var stock int
		var skuSpecInfo map[string]string

		if item.SkuID != nil {
			// SKU-based item: query product_skus.
			var specJSON []byte
			err := tx.QueryRowContext(ctx,
				`SELECT ps.price_cents, ps.stock, ps.spec_info, p.name
				 FROM product_skus ps
				 JOIN products p ON p.id = ps.product_id
				 WHERE ps.id = $1 AND ps.status = 'active' AND ps.deleted_at IS NULL
				   AND p.merchant_id = $2 AND p.status = 'active' AND p.deleted_at IS NULL
				 FOR UPDATE OF ps`,
				*item.SkuID, merchantID,
			).Scan(&priceCents, &stock, &specJSON, &name)
			if err == sql.ErrNoRows {
				return nil, apperrors.NewNotFoundError("SKU not found or inactive: " + strconv.FormatInt(*item.SkuID, 10))
			}
			if err != nil {
				return nil, apperrors.NewInternalError("failed to query SKU", err)
			}
			if specJSON != nil {
				json.Unmarshal(specJSON, &skuSpecInfo)
			}
		} else {
			// Product-level item: query products.
			err := tx.QueryRowContext(ctx,
				`SELECT name, price_cents, stock FROM products
				 WHERE id = $1 AND merchant_id = $2 AND status = 'active' AND deleted_at IS NULL
				 FOR UPDATE`, item.ProductID, merchantID,
			).Scan(&name, &priceCents, &stock)
			if err == sql.ErrNoRows {
				return nil, apperrors.NewNotFoundError("product not found or inactive: " + strconv.FormatInt(item.ProductID, 10))
			}
			if err != nil {
				return nil, apperrors.NewInternalError("failed to query product", err)
			}
		}
		if stock < item.Quantity {
			return nil, apperrors.NewValidationError("insufficient stock for: " + name)
		}

		totalCents += priceCents * item.Quantity
		itemDetails = append(itemDetails, OrderItemDetail{
			ProductID:   item.ProductID,
			ProductName: name,
			SkuID:       item.SkuID,
			SkuSpecInfo: skuSpecInfo,
			PriceCents:  priceCents,
			Quantity:    item.Quantity,
		})
	}

	// Validate payments total.
	var paidCents int
	var paymentDetails []PaymentDetail
	for _, p := range req.Payments {
		if p.AmountCents <= 0 {
			return nil, apperrors.NewValidationError("payment amount must be positive")
		}
		switch p.Method {
		case "cash", "wechat", "alipay", "balance":
		default:
			return nil, apperrors.NewValidationError("invalid payment method: " + p.Method)
		}
		paidCents += p.AmountCents
		paymentDetails = append(paymentDetails, PaymentDetail{
			Method:     p.Method,
			AmountCents: p.AmountCents,
		})
	}
	if paidCents < totalCents {
		return nil, apperrors.NewValidationError("payment amount insufficient")
	}

	// Create order.
	var orderID int64
	var orderCreatedAt time.Time
	err = tx.QueryRowContext(ctx,
		`INSERT INTO orders (merchant_id, member_id, total_cents, paid_cents, status)
		 VALUES ($1, $2, $3, $4, 'completed')
		 RETURNING id, created_at`,
		merchantID, req.MemberID, totalCents, paidCents,
	).Scan(&orderID, &orderCreatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create order", err)
	}

	// Create order items, deduct inventory, record stock flows.
	for i, item := range req.Items {
		detail := itemDetails[i]

		var skuSpecJSON []byte
		if detail.SkuSpecInfo != nil {
			skuSpecJSON, _ = json.Marshal(detail.SkuSpecInfo)
		}

		if item.SkuID != nil {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO order_items (order_id, product_id, product_name, price_cents, quantity, product_sku_id, sku_spec_info)
				 SELECT $1, $2, name, $3, $4, $5, $6 FROM products WHERE id = $2`,
				orderID, item.ProductID, detail.PriceCents, item.Quantity, *item.SkuID, skuSpecJSON,
			)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to create order item", err)
			}

			_, err = tx.ExecContext(ctx,
				`UPDATE product_skus SET stock = stock - $1, updated_at = NOW()
				 WHERE id = $2`,
				item.Quantity, *item.SkuID,
			)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to deduct SKU inventory", err)
			}

			_, err = tx.ExecContext(ctx,
				`INSERT INTO stock_flows (merchant_id, product_id, product_sku_id, order_id, type, quantity_change)
				 VALUES ($1, $2, $3, $4, 'sale', $5)`,
				merchantID, item.ProductID, *item.SkuID, orderID, -item.Quantity,
			)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to record stock flow", err)
			}
		} else {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO order_items (order_id, product_id, product_name, price_cents, quantity)
				 SELECT $1, $2, name, $3, $4 FROM products WHERE id = $2`,
				orderID, item.ProductID, detail.PriceCents, item.Quantity,
			)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to create order item", err)
			}

			_, err = tx.ExecContext(ctx,
				`UPDATE products SET stock = stock - $1, updated_at = NOW()
				 WHERE id = $2 AND merchant_id = $3`,
				item.Quantity, item.ProductID, merchantID,
			)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to deduct inventory", err)
			}

			_, err = tx.ExecContext(ctx,
				`INSERT INTO stock_flows (merchant_id, product_id, order_id, type, quantity_change)
				 VALUES ($1, $2, $3, 'sale', $4)`,
				merchantID, item.ProductID, orderID, -item.Quantity,
			)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to record stock flow", err)
			}
		}
	}

	// Create payments.
	for _, p := range req.Payments {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO payments (order_id, method, amount_cents)
			 VALUES ($1, $2, $3)`,
			orderID, p.Method, p.AmountCents,
		)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to create payment", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit transaction", err)
	}

	return &CheckoutResponse{
		OrderID:    orderID,
		MerchantID: merchantID,
		TotalCents: totalCents,
		PaidCents:  paidCents,
		Status:     "completed",
		Items:      itemDetails,
		Payments:   paymentDetails,
		CreatedAt:  orderCreatedAt,
	}, nil
}
