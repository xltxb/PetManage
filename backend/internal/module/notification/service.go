package notification

import (
	"encoding/json"
	"strings"
	"time"

	"pawprint/backend/internal/pkg/apperr"
)

// Service handles notification sending logic.
type Service struct {
	repo       Repository
	smsEnabled bool
	wxEnabled  bool
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, smsEnabled: false, wxEnabled: false}
}

// SetFeatureFlags configures which channels are enabled.
func (s *Service) SetFeatureFlags(sms, wechat bool) {
	s.smsEnabled = sms
	s.wxEnabled = wechat
}

// SendRequest contains data for sending a notification.
type SendRequest struct {
	StoreID      int64
	CustomerID   int64
	TemplateCode string
	Channel      string
	ScheduledAt  *time.Time
	Payload      map[string]string
}

// Send creates a notification log and dispatches based on channel.
func (s *Service) Send(req SendRequest) error {
	template, err := s.repo.FindTemplate(req.TemplateCode, req.Channel)
	if err != nil {
		return apperr.Internal(err)
	}

	payloadJSON := "{}"
	if req.Payload != nil {
		data, _ := json.Marshal(req.Payload)
		payloadJSON = string(data)
	}

	// Determine status based on channel availability
	status := StatusSent
	if !s.isChannelEnabled(req.Channel) {
		status = StatusSkipped
	}

	// Render content if template exists
	content := ""
	if template != nil {
		content = RenderTemplate(template.Content, req.Payload)
	}

	var customerID *int64
	if req.CustomerID > 0 {
		customerID = &req.CustomerID
	}

	now := time.Now().UTC()
	log := &NotificationLog{
		StoreID:      req.StoreID,
		CustomerID:   customerID,
		TemplateCode: req.TemplateCode,
		Channel:      req.Channel,
		Payload:      payloadJSON,
		Status:       status,
		SentAt:       &now,
		ScheduledAt:  req.ScheduledAt,
	}

	if err := s.repo.CreateLog(log); err != nil {
		return apperr.Internal(err)
	}

	_ = content
	return nil
}

func (s *Service) ScanVaccineDue(now time.Time, days int) (int, error) {
	rows, err := s.repo.FindVaccineDue(now, days)
	if err != nil {
		return 0, apperr.Internal(err)
	}
	for _, row := range rows {
		for _, channel := range []string{ChannelSMS, ChannelWechatMp} {
			if err := s.Send(SendRequest{
				StoreID:      row.StoreID,
				CustomerID:   row.CustomerID,
				TemplateCode: "vaccine_due",
				Channel:      channel,
				Payload:      vaccineDuePayload(row),
			}); err != nil {
				return 0, err
			}
		}
	}
	return len(rows), nil
}

func vaccineDuePayload(row VaccineDuePet) map[string]string {
	dueAt := row.DueAt.Format("2006-01-02")
	return map[string]string{
		"petName": row.PetName,
		"dueAt":   dueAt,
		"dueDate": dueAt,
	}
}

func (s *Service) isChannelEnabled(channel string) bool {
	switch channel {
	case ChannelInApp:
		return true // in-app always enabled
	case ChannelSMS:
		return s.smsEnabled
	case ChannelWechatMp:
		return s.wxEnabled
	default:
		return false
	}
}

// RenderTemplate replaces {placeholder} values in a template string.
func RenderTemplate(template string, payload map[string]string) string {
	result := template
	for key, value := range payload {
		result = strings.ReplaceAll(result, "{"+key+"}", value)
	}
	return result
}
