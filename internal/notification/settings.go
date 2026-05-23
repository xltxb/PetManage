package notification

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Predefined notification scenarios.
const (
	ScenarioAppointmentReminder = "appointment_reminder"
	ScenarioServiceProgress     = "service_progress"
	ScenarioConsumptionNotice   = "consumption_notice"
	ScenarioBirthdayGreeting    = "birthday_greeting"
	ScenarioInventoryAlert      = "inventory_alert"
)

// NotificationSetting represents per-scenario per-channel enable/disable.
type NotificationSetting struct {
	ID            int64     `json:"id"`
	MerchantID    int64     `json:"merchant_id"`
	Scenario      string    `json:"scenario"`
	SMSEnabled    bool      `json:"sms_enabled"`
	WechatEnabled bool      `json:"wechat_enabled"`
	SystemEnabled bool      `json:"system_enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// NotificationTemplate represents SMS/WeChat template configuration.
type NotificationTemplate struct {
	ID         int64     `json:"id"`
	MerchantID int64     `json:"merchant_id"`
	Scenario   string    `json:"scenario"`
	Channel    string    `json:"channel"`
	TemplateID string    `json:"template_id"`
	Signature  string    `json:"signature"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// UpdateSettingsRequest is a batch update for notification settings.
type UpdateSettingsRequest struct {
	Settings []SettingEntry `json:"settings"`
}

// SettingEntry represents one scenario's channel toggles.
type SettingEntry struct {
	Scenario      string `json:"scenario"`
	SMSEnabled    bool   `json:"sms_enabled"`
	WechatEnabled bool   `json:"wechat_enabled"`
	SystemEnabled bool   `json:"system_enabled"`
}

// UpdateTemplateRequest is a batch update for notification templates.
type UpdateTemplateRequest struct {
	Templates []TemplateEntry `json:"templates"`
}

// TemplateEntry represents one scenario-channel template config.
type TemplateEntry struct {
	Scenario   string `json:"scenario"`
	Channel    string `json:"channel"`
	TemplateID string `json:"template_id"`
	Signature  string `json:"signature"`
}

// SendRecord wraps a notification with channel information for querying.
type SendRecord struct {
	Notification
}

var allScenarios = []string{
	ScenarioAppointmentReminder,
	ScenarioServiceProgress,
	ScenarioConsumptionNotice,
	ScenarioBirthdayGreeting,
	ScenarioInventoryAlert,
}

// GetSettings returns all notification settings for a merchant, creating defaults if missing.
func (s *Service) GetSettings(ctx context.Context, merchantID int64) ([]NotificationSetting, error) {
	// Ensure all scenarios have a settings row.
	for _, scenario := range allScenarios {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO notification_settings (merchant_id, scenario) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			merchantID, scenario,
		)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to init notification settings", err)
		}
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, merchant_id, scenario, sms_enabled, wechat_enabled, system_enabled, created_at, updated_at
		 FROM notification_settings
		 WHERE merchant_id = $1
		 ORDER BY scenario`,
		merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load notification settings", err)
	}
	defer rows.Close()

	settings := make([]NotificationSetting, 0)
	for rows.Next() {
		var ns NotificationSetting
		if err := rows.Scan(&ns.ID, &ns.MerchantID, &ns.Scenario, &ns.SMSEnabled, &ns.WechatEnabled, &ns.SystemEnabled, &ns.CreatedAt, &ns.UpdatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan notification setting", err)
		}
		settings = append(settings, ns)
	}
	return settings, rows.Err()
}

// UpdateSettings updates notification settings for a merchant.
func (s *Service) UpdateSettings(ctx context.Context, merchantID int64, req UpdateSettingsRequest) error {
	validScenarios := map[string]bool{}
	for _, sc := range allScenarios {
		validScenarios[sc] = true
	}

	for _, entry := range req.Settings {
		if !validScenarios[entry.Scenario] {
			return apperrors.NewValidationError("invalid scenario: " + entry.Scenario)
		}
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO notification_settings (merchant_id, scenario, sms_enabled, wechat_enabled, system_enabled, updated_at)
			 VALUES ($1, $2, $3, $4, $5, NOW())
			 ON CONFLICT (merchant_id, scenario)
			 DO UPDATE SET sms_enabled = $3, wechat_enabled = $4, system_enabled = $5, updated_at = NOW()`,
			merchantID, entry.Scenario, entry.SMSEnabled, entry.WechatEnabled, entry.SystemEnabled,
		)
		if err != nil {
			return apperrors.NewInternalError("failed to update notification settings", err)
		}
	}
	return nil
}

// GetTemplates returns all notification templates for a merchant, creating defaults if missing.
func (s *Service) GetTemplates(ctx context.Context, merchantID int64) ([]NotificationTemplate, error) {
	// Ensure all scenario-channel combinations exist.
	for _, scenario := range allScenarios {
		for _, ch := range []string{"sms", "wechat"} {
			s.db.ExecContext(ctx,
				`INSERT INTO notification_templates (merchant_id, scenario, channel) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
				merchantID, scenario, ch,
			)
		}
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, merchant_id, scenario, channel, template_id, signature, created_at, updated_at
		 FROM notification_templates
		 WHERE merchant_id = $1
		 ORDER BY scenario, channel`,
		merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load notification templates", err)
	}
	defer rows.Close()

	templates := make([]NotificationTemplate, 0)
	for rows.Next() {
		var nt NotificationTemplate
		if err := rows.Scan(&nt.ID, &nt.MerchantID, &nt.Scenario, &nt.Channel, &nt.TemplateID, &nt.Signature, &nt.CreatedAt, &nt.UpdatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan notification template", err)
		}
		templates = append(templates, nt)
	}
	return templates, rows.Err()
}

// UpdateTemplates updates notification templates for a merchant.
func (s *Service) UpdateTemplates(ctx context.Context, merchantID int64, req UpdateTemplateRequest) error {
	validScenarios := map[string]bool{}
	for _, sc := range allScenarios {
		validScenarios[sc] = true
	}

	for _, entry := range req.Templates {
		if !validScenarios[entry.Scenario] {
			return apperrors.NewValidationError("invalid scenario: " + entry.Scenario)
		}
		if entry.Channel != "sms" && entry.Channel != "wechat" {
			return apperrors.NewValidationError("invalid channel: " + entry.Channel + " (must be sms or wechat)")
		}
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO notification_templates (merchant_id, scenario, channel, template_id, signature, updated_at)
			 VALUES ($1, $2, $3, $4, $5, NOW())
			 ON CONFLICT (merchant_id, scenario, channel)
			 DO UPDATE SET template_id = $4, signature = $5, updated_at = NOW()`,
			merchantID, entry.Scenario, entry.Channel, entry.TemplateID, entry.Signature,
		)
		if err != nil {
			return apperrors.NewInternalError("failed to update notification templates", err)
		}
	}
	return nil
}

// SendBirthdayNotifications finds members whose birthday is today and sends
// birthday greeting notifications via enabled channels.
func (s *Service) SendBirthdayNotifications(ctx context.Context) (int, error) {
	today := time.Now().Format("01-02") // MM-DD format

	// Get all active merchants with birthday greeting enabled.
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT m.merchant_id, m.id, m.name, m.phone
		 FROM members m
		 WHERE m.deleted_at IS NULL
		 AND m.status = 'active'
		 AND m.birthday IS NOT NULL
		 AND m.birthday::text LIKE '%-`+today+`'`,
	)
	if err != nil {
		return 0, apperrors.NewInternalError("failed to query birthday members", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var merchantID, memberID int64
		var name, phone string
		if err := rows.Scan(&merchantID, &memberID, &name, &phone); err != nil {
			continue
		}

		// Send via system channel (in-app notification).
		title := "生日祝福"
		content := "亲爱的" + name + "，祝您生日快乐！愿您和您的宠物健康快乐每一天！"
		n := &Notification{
			MerchantID: merchantID,
			UserID:     memberID,
			UserType:   "member",
			Title:      title,
			Content:    content,
			Category:   ScenarioBirthdayGreeting,
			RelatedID:  memberID,
		}
		if _, err := s.Create(ctx, n); err == nil {
			count++
		}
	}

	return count, rows.Err()
}

// SendInventoryAlertNotifications sends inventory alert notifications to the merchant.
func (s *Service) SendInventoryAlertNotifications(ctx context.Context, merchantID int64) (int, error) {
	oneDayAgo := time.Now().Add(-24 * time.Hour)

	rows, err := s.db.QueryContext(ctx,
		`SELECT p.id, p.name, p.stock, p.alert_stock, p.expiry_date,
		        CASE
		            WHEN p.expiry_date IS NOT NULL AND p.expiry_date < CURRENT_DATE THEN 'expired'
		            WHEN p.expiry_date IS NOT NULL AND p.expiry_date >= CURRENT_DATE AND p.expiry_date <= CURRENT_DATE + INTERVAL '30 days' THEN 'near_expiry'
		            WHEN p.alert_stock > 0 AND p.stock < p.alert_stock THEN 'low_stock'
		        END AS alert_type
		 FROM products p
		 WHERE p.deleted_at IS NULL
		 AND p.merchant_id = $1
		 AND p.status = 'active'
		 AND (
		     (p.alert_stock > 0 AND p.stock < p.alert_stock AND (p.expiry_date IS NULL OR p.expiry_date > CURRENT_DATE + INTERVAL '30 days'))
		     OR (p.expiry_date IS NOT NULL AND p.expiry_date >= CURRENT_DATE AND p.expiry_date <= CURRENT_DATE + INTERVAL '30 days')
		     OR (p.expiry_date IS NOT NULL AND p.expiry_date < CURRENT_DATE)
		 )
		 AND NOT EXISTS (
		     SELECT 1 FROM notifications n2
		     WHERE n2.category = $2
		     AND n2.related_id = p.id
		     AND n2.merchant_id = p.merchant_id
		     AND n2.created_at > $3
		 )`,
		merchantID, ScenarioInventoryAlert, oneDayAgo,
	)
	if err != nil {
		return 0, apperrors.NewInternalError("failed to query inventory alerts", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var productID int64
		var name string
		var stock, alertStock int
		var expiryDate sql.NullString
		var alertType string
		if err := rows.Scan(&productID, &name, &stock, &alertStock, &expiryDate, &alertType); err != nil {
			continue
		}

		var title, content string
		switch alertType {
		case "low_stock":
			title = "库存预警"
			content = "商品「" + name + "」库存不足（当前库存：" + itoa(stock) + "，预警值：" + itoa(alertStock) + "），请及时补货。"
		case "near_expiry":
			title = "商品临期提醒"
			dateStr := ""
			if expiryDate.Valid {
				dateStr = expiryDate.String[:10]
			}
			content = "商品「" + name + "」临近保质期（有效期至 " + dateStr + "），请注意处理。"
		case "expired":
			title = "商品过期提醒"
			dateStr := ""
			if expiryDate.Valid {
				dateStr = expiryDate.String[:10]
			}
			content = "商品「" + name + "」已过保质期（有效期至 " + dateStr + "），请及时下架处理。"
		}

		n := &Notification{
			MerchantID: merchantID,
			UserID:     0,
			UserType:   "employee",
			Title:      title,
			Content:    content,
			Category:   ScenarioInventoryAlert,
			RelatedID:  productID,
		}
		if _, err := s.Create(ctx, n); err == nil {
			count++
		}
	}

	return count, rows.Err()
}

// scenarioToCategory maps a settings scenario to the notification category used in the notifications table.
func scenarioToCategory(scenario string) string {
	switch scenario {
	case ScenarioAppointmentReminder:
		return "appointment"
	case ScenarioServiceProgress:
		return "appointment"
	case ScenarioConsumptionNotice:
		return "consumption"
	case ScenarioBirthdayGreeting:
		return "birthday_greeting"
	case ScenarioInventoryAlert:
		return "inventory_alert"
	default:
		return scenario
	}
}

// GetSendRecords queries sent notifications with channel information.
func (s *Service) GetSendRecords(ctx context.Context, merchantID int64, params SendRecordParams) (*ListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}

	var conditions []string
	var args []interface{}
	args = append(args, merchantID)
	conditions = append(conditions, "merchant_id = $1")
	argIdx := 2

	if params.Category != "" {
		conditions = append(conditions, "category = $"+itoa(argIdx))
		args = append(args, params.Category)
		argIdx++
	}
	if params.Channel != "" {
		conditions = append(conditions, "channel = $"+itoa(argIdx))
		args = append(args, params.Channel)
		argIdx++
	}
	if params.Scenario != "" {
		cat := scenarioToCategory(params.Scenario)
		conditions = append(conditions, "category = $"+itoa(argIdx))
		args = append(args, cat)
		argIdx++
	}
	if params.UserType != "" {
		conditions = append(conditions, "user_type = $"+itoa(argIdx))
		args = append(args, params.UserType)
		argIdx++
	}

	whereClause := strings.Join(conditions, " AND ")

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notifications WHERE `+whereClause,
		args...,
	).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count send records", err)
	}

	offset := (params.Page - 1) * params.PageSize
	queryArgs := append(args, params.PageSize, offset)

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, merchant_id, user_id, user_type, title, content, category, related_id, is_read, send_status, channel, created_at
		 FROM notifications
		 WHERE `+whereClause+`
		 ORDER BY created_at DESC LIMIT $`+itoa(argIdx)+` OFFSET $`+itoa(argIdx+1),
		queryArgs...,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list send records", err)
	}
	defer rows.Close()

	records := make([]Notification, 0)
	for rows.Next() {
		var n Notification
		if err := rows.Scan(
			&n.ID, &n.MerchantID, &n.UserID, &n.UserType,
			&n.Title, &n.Content, &n.Category, &n.RelatedID,
			&n.IsRead, &n.SendStatus, &n.Channel, &n.CreatedAt,
		); err != nil {
			return nil, apperrors.NewInternalError("failed to scan send record", err)
		}
		records = append(records, n)
	}
	if records == nil {
		records = []Notification{}
	}

	return &ListResult{
		Notifications: records,
		Total:         total,
		Page:          params.Page,
		PageSize:      params.PageSize,
	}, rows.Err()
}

// SendRecordParams holds filters for querying send records.
type SendRecordParams struct {
	Category string
	Channel  string
	Scenario string
	UserType string
	Page     int
	PageSize int
}
