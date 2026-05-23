package receipttemplate

import (
	"context"
	"database/sql"
	"errors"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Service handles receipt template configuration and receipt data generation.
type Service struct {
	db *sql.DB
}

// NewService creates a new receipt template service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Template represents a merchant's receipt template configuration.
type Template struct {
	MerchantID     int64  `json:"merchant_id"`
	LogoURL        string `json:"logo_url"`
	StoreName      string `json:"store_name"`
	ContactPhone   string `json:"contact_phone"`
	ContactAddress string `json:"contact_address"`
	FooterNote     string `json:"footer_note"`
	PaperWidth     string `json:"paper_width"`
	ShowQRCode     bool   `json:"show_qrcode"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// UpdateTemplateRequest is the request body for updating a receipt template.
type UpdateTemplateRequest struct {
	LogoURL        string `json:"logo_url"`
	StoreName      string `json:"store_name"`
	ContactPhone   string `json:"contact_phone"`
	ContactAddress string `json:"contact_address"`
	FooterNote     string `json:"footer_note"`
	PaperWidth     string `json:"paper_width"`
	ShowQRCode     bool   `json:"show_qrcode"`
}

// OrderReceipt contains all data needed to render/print a receipt for an order.
type OrderReceipt struct {
	OrderID       int64              `json:"order_id"`
	StoreName     string             `json:"store_name"`
	StoreLogo     string             `json:"store_logo"`
	ContactPhone  string             `json:"contact_phone"`
	ContactAddr   string             `json:"contact_address"`
	FooterNote    string             `json:"footer_note"`
	MemberName    string             `json:"member_name"`
	MemberPhone   string             `json:"member_phone"`
	Items         []ReceiptLineItem  `json:"items"`
	Payments      []ReceiptPayment   `json:"payments"`
	SubtotalCents int                `json:"subtotal_cents"`
	DiscountCents int                `json:"discount_cents"`
	TotalCents    int                `json:"total_cents"`
	PaidCents     int                `json:"paid_cents"`
	ChangeCents   int                `json:"change_cents"`
	Notes         string             `json:"notes"`
	CreatedAt     string             `json:"created_at"`
}

// ReceiptLineItem is a single line on the receipt.
type ReceiptLineItem struct {
	Name       string `json:"name"`
	Quantity   int    `json:"quantity"`
	PriceCents int    `json:"price_cents"`
	TotalCents int    `json:"total_cents"`
}

// ReceiptPayment is a payment record on the receipt.
type ReceiptPayment struct {
	Method     string `json:"method"`
	AmountCents int   `json:"amount_cents"`
}

// GetTemplate returns the receipt template for a merchant, falling back to shop defaults.
func (s *Service) GetTemplate(ctx context.Context, merchantID int64) (*Template, error) {
	var t Template
	t.MerchantID = merchantID

	err := s.db.QueryRowContext(ctx,
		`SELECT logo_url, store_name, contact_phone, contact_address,
		        footer_note, paper_width, show_qrcode, created_at, updated_at
		 FROM receipt_templates WHERE merchant_id = $1`,
		merchantID,
	).Scan(&t.LogoURL, &t.StoreName, &t.ContactPhone, &t.ContactAddress,
		&t.FooterNote, &t.PaperWidth, &t.ShowQRCode, &t.CreatedAt, &t.UpdatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		// Return defaults merged from merchant profile.
		s.fillDefaults(ctx, &t)
		return &t, nil
	}
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to get receipt template",
			Err:     err,
		}
	}
	return &t, nil
}

func (s *Service) fillDefaults(ctx context.Context, t *Template) {
	t.PaperWidth = "80mm"
	// Merge shop-level settings as defaults.
	var name, phone, addr, logo sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT name, contact_phone, address, logo_url FROM merchants WHERE id = $1 AND deleted_at IS NULL`,
		t.MerchantID,
	).Scan(&name, &phone, &addr, &logo)
	if err == nil {
		if name.Valid {
			t.StoreName = name.String
		}
		if phone.Valid {
			t.ContactPhone = phone.String
		}
		if addr.Valid {
			t.ContactAddress = addr.String
		}
		if logo.Valid {
			t.LogoURL = logo.String
		}
	}
}

// SaveTemplate creates or updates the receipt template for a merchant.
func (s *Service) SaveTemplate(ctx context.Context, merchantID int64, req UpdateTemplateRequest) (*Template, error) {
	var t Template
	t.MerchantID = merchantID

	err := s.db.QueryRowContext(ctx,
		`INSERT INTO receipt_templates
		 (merchant_id, logo_url, store_name, contact_phone, contact_address, footer_note, paper_width, show_qrcode)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (merchant_id) DO UPDATE SET
		   logo_url = EXCLUDED.logo_url, store_name = EXCLUDED.store_name,
		   contact_phone = EXCLUDED.contact_phone, contact_address = EXCLUDED.contact_address,
		   footer_note = EXCLUDED.footer_note, paper_width = EXCLUDED.paper_width,
		   show_qrcode = EXCLUDED.show_qrcode, updated_at = NOW()
		 RETURNING logo_url, store_name, contact_phone, contact_address,
		           footer_note, paper_width, show_qrcode, created_at, updated_at`,
		merchantID, req.LogoURL, req.StoreName, req.ContactPhone,
		req.ContactAddress, req.FooterNote, req.PaperWidth, req.ShowQRCode,
	).Scan(&t.LogoURL, &t.StoreName, &t.ContactPhone, &t.ContactAddress,
		&t.FooterNote, &t.PaperWidth, &t.ShowQRCode, &t.CreatedAt, &t.UpdatedAt)

	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to save receipt template",
			Err:     err,
		}
	}
	return &t, nil
}

// GetOrderReceipt generates the full receipt data for a given order.
func (s *Service) GetOrderReceipt(ctx context.Context, merchantID, orderID int64) (*OrderReceipt, error) {
	tmpl, err := s.GetTemplate(ctx, merchantID)
	if err != nil {
		return nil, err
	}

	r := &OrderReceipt{
		OrderID:      orderID,
		StoreName:    tmpl.StoreName,
		StoreLogo:    tmpl.LogoURL,
		ContactPhone: tmpl.ContactPhone,
		ContactAddr:  tmpl.ContactAddress,
		FooterNote:   tmpl.FooterNote,
	}

	// Fetch order header.
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(m.name, ''), COALESCE(m.phone, ''),
		        o.total_cents, o.paid_cents, o.status, COALESCE(o.notes, ''),
		        o.created_at
		 FROM orders o
		 LEFT JOIN members m ON o.member_id = m.id
		 WHERE o.id = $1 AND o.merchant_id = $2`,
		orderID, merchantID,
	).Scan(&r.MemberName, &r.MemberPhone, &r.TotalCents, &r.PaidCents,
		new(string), &r.Notes, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeNotFound,
			Message: "order not found",
		}
	}
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to fetch order",
			Err:     err,
		}
	}

	// Fetch order items.
	rows, err := s.db.QueryContext(ctx,
		`SELECT COALESCE(oi.product_name, si.name, ''), oi.quantity, oi.price_cents
		 FROM order_items oi
		 LEFT JOIN service_items si ON oi.service_item_id = si.id
		 WHERE oi.order_id = $1
		 ORDER BY oi.id`,
		orderID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var item ReceiptLineItem
			if err := rows.Scan(&item.Name, &item.Quantity, &item.PriceCents); err == nil {
				item.TotalCents = item.PriceCents * item.Quantity
				r.Items = append(r.Items, item)
			}
		}
	}

	// Fetch payments.
	payRows, err := s.db.QueryContext(ctx,
		`SELECT method, amount_cents FROM payments WHERE order_id = $1 ORDER BY id`,
		orderID)
	if err == nil {
		defer payRows.Close()
		for payRows.Next() {
			var p ReceiptPayment
			if err := payRows.Scan(&p.Method, &p.AmountCents); err == nil {
				r.Payments = append(r.Payments, p)
			}
		}
	}

	r.ChangeCents = r.PaidCents - r.TotalCents
	if r.ChangeCents < 0 {
		r.ChangeCents = 0
	}

	return r, nil
}

// methodLabel returns a Chinese label for a payment method.
func MethodLabel(m string) string {
	switch m {
	case "cash":
		return "现金"
	case "wechat":
		return "微信"
	case "alipay":
		return "支付宝"
	case "balance":
		return "储值"
	case "points":
		return "积分"
	case "coupon":
		return "优惠券"
	default:
		return m
	}
}
