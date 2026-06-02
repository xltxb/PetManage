package notification

import (
	"testing"
	"time"
)

type mockRepo struct {
	logs      []NotificationLog
	templates map[string]*NotificationTemplate
	duePets   []VaccineDuePet
	nextID    int64
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		logs:      make([]NotificationLog, 0),
		templates: make(map[string]*NotificationTemplate),
		nextID:    1,
	}
}

func (m *mockRepo) CreateLog(log *NotificationLog) error {
	log.ID = m.nextID
	m.nextID++
	m.logs = append(m.logs, *log)
	return nil
}
func (m *mockRepo) FindTemplate(code, channel string) (*NotificationTemplate, error) {
	t, ok := m.templates[code+":"+channel]
	if !ok {
		return nil, nil
	}
	return t, nil
}
func (m *mockRepo) FindPendingLogs(limit int) ([]NotificationLog, error) { return nil, nil }
func (m *mockRepo) FindVaccineDue(now time.Time, days int) ([]VaccineDuePet, error) {
	return m.duePets, nil
}

func TestSendInApp(t *testing.T) {
	repo := newMockRepo()
	repo.templates["appointment_confirmed:inapp"] = &NotificationTemplate{
		Code: "appointment_confirmed", Channel: "inapp",
		Title: "预约成功", Content: "您在{storeName}的预约已确认：{serviceName} {time}",
	}
	svc := NewService(repo)

	err := svc.Send(SendRequest{
		StoreID:      1,
		CustomerID:   1,
		TemplateCode: "appointment_confirmed",
		Channel:      "inapp",
		Payload:      map[string]string{"storeName": "旗舰店", "serviceName": "全套SPA", "time": "10:00"},
	})
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if len(repo.logs) != 1 {
		t.Fatalf("logs count = %d, want 1", len(repo.logs))
	}
	log := repo.logs[0]
	if log.Channel != "inapp" {
		t.Errorf("channel = %q, want inapp", log.Channel)
	}
	if log.Status != "sent" {
		t.Errorf("status = %q, want sent (inapp always sends)", log.Status)
	}
}

func TestSendSMSDisabled(t *testing.T) {
	repo := newMockRepo()
	repo.templates["visit_reminder:sms"] = &NotificationTemplate{
		Code: "visit_reminder", Channel: "sms",
		Title: "", Content: "【爪迹】提醒：{petName}的预约将于{time}开始",
	}
	svc := NewService(repo)
	svc.smsEnabled = false

	err := svc.Send(SendRequest{
		StoreID: 1, TemplateCode: "visit_reminder", Channel: "sms",
	})
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	log := repo.logs[0]
	if log.Status != "skipped" {
		t.Errorf("status = %q, want skipped (sms disabled)", log.Status)
	}
}

func TestScanVaccineDueCreatesSkippedSMSWhenDisabled(t *testing.T) {
	repo := newMockRepo()
	repo.duePets = []VaccineDuePet{{
		StoreID:    1,
		CustomerID: 100,
		PetID:      200,
		PetName:    "布丁",
		DueAt:      time.Now().UTC().Add(7 * 24 * time.Hour),
	}}
	repo.templates["vaccine_due:sms"] = &NotificationTemplate{
		Code: "vaccine_due", Channel: "sms", Content: "【爪迹】{petName}疫苗即将到期：{dueAt}",
	}
	svc := NewService(repo)
	svc.SetFeatureFlags(false, false)

	count, err := svc.ScanVaccineDue(time.Now().UTC(), 7)
	if err != nil {
		t.Fatalf("ScanVaccineDue error = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	if len(repo.logs) != 1 {
		t.Fatalf("logs count = %d, want 1", len(repo.logs))
	}
	if repo.logs[0].TemplateCode != "vaccine_due" || repo.logs[0].Status != StatusSkipped {
		t.Fatalf("log = %#v", repo.logs[0])
	}
}

func TestTemplateRendering(t *testing.T) {
	template := "您在{storeName}的预约已确认：{serviceName} {time}"
	payload := map[string]string{
		"storeName":   "旗舰店",
		"serviceName": "全套SPA",
		"time":        "10:00",
	}
	result := RenderTemplate(template, payload)
	expected := "您在旗舰店的预约已确认：全套SPA 10:00"
	if result != expected {
		t.Errorf("RenderTemplate = %q, want %q", result, expected)
	}
}
