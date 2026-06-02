# PawPrint P0 Backend Acceptance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the backend behavior required by PawPrint P0 acceptance cases before expanding the admin frontend.

**Architecture:** Keep the existing Go module-per-feature structure and add narrowly scoped service, repository, router, and middleware changes. Cross-module business side effects are coordinated in service-layer transactions. Tests drive every slice from the P0 cases in `files/测试用例.md`.

**Tech Stack:** Go 1.22+, Gin, GORM, PostgreSQL, Redis, Vue/Vite build smoke gate

**Spec:** `docs/superpowers/specs/2026-06-02-pawprint-p0-backend-acceptance-design.md`

---

## File Map

- Modify: `backend/internal/router/router.go`
  - Construct Redis client from `cfg.Redis.Addr`.
  - Pass auth service, notification service, Redis-backed idempotency middleware, and permission middleware into route registration.
  - Register `/api/v1/wx` customer-facing routes.
- Modify: `backend/internal/module/*/router.go`
  - Add route-level `RequirePermission` and `Idempotency` middleware where the P0 cases require them.
- Modify: `backend/internal/module/auth/service.go`
  - Add failed-login count and lockout logic.
- Modify: `backend/internal/module/auth/repo.go`
  - Persist failed-login and lockout data using existing user table columns if present; otherwise add the small migration in Task 1.
- Create: `backend/migrations/000003_auth_lockout_idempotency.up.sql`
- Create: `backend/migrations/000003_auth_lockout_idempotency.down.sql`
  - Add minimal auth lockout fields only if absent.
- Modify: `backend/internal/middleware/idempotency.go`
  - Include method, path, store, user/customer actor, and key in the cache hash.
- Modify/Test: `backend/internal/middleware/rbac_test.go`
  - Keep existing incomplete-context tests and add route-level permission tests.
- Modify/Test: `backend/internal/module/appointment/*`
  - Add notification dependency for create, enforce computed end time when needed, and keep conflict/state-machine behavior covered.
- Create: `backend/internal/module/wx/*`
  - Add mini-program mock login, service offering list, appointment create, and appointment cancel endpoints.
- Modify/Test: `backend/internal/module/settlement/*`
  - Add transactional payment/refund side effects, wallet, points, inventory, print job, and idempotency behavior.
- Modify/Test: `backend/internal/module/member/*`
  - Add transaction-aware wallet and points operations.
- Modify/Test: `backend/internal/module/inventory/*`
  - Add row-locking sale/purchase operations and stock-low notification integration.
- Modify/Test: `backend/internal/module/boarding/*`
  - Generate settlement during checkout and wrap room/order/settlement updates in one transaction.
- Modify/Test: `backend/internal/module/notification/*`
  - Add stock-low and vaccine-due helper APIs.

## Common Commands

Use these gates repeatedly:

```bash
cd backend && go test ./internal/middleware
cd backend && go test ./internal/module/auth
cd backend && go test ./internal/module/appointment
cd backend && go test ./internal/module/wx
cd backend && go test ./internal/module/settlement
cd backend && go test ./internal/module/boarding
cd backend && go test ./internal/module/inventory
cd backend && go test ./internal/module/notification
cd backend && go test ./...
cd admin && npm run build
```

## Task 1: Security Gate, Lockout, RBAC, Store Scope

**Files:**
- Create: `backend/migrations/000003_auth_lockout_idempotency.up.sql`
- Create: `backend/migrations/000003_auth_lockout_idempotency.down.sql`
- Modify: `backend/internal/module/auth/model.go`
- Modify: `backend/internal/module/auth/repo.go`
- Modify: `backend/internal/module/auth/service.go`
- Modify: `backend/internal/module/auth/service_test.go`
- Modify: `backend/internal/router/router.go`
- Modify: `backend/internal/module/appointment/router.go`
- Modify: `backend/internal/module/member/router.go`
- Modify: `backend/internal/module/settlement/router.go`
- Modify: `backend/internal/module/inventory/router.go`
- Test: `backend/internal/middleware/rbac_test.go`

- [ ] **Step 1: Add failing auth lockout test**

Append this test to `backend/internal/module/auth/service_test.go`:

```go
func TestLoginLocksAccountAfterFiveFailures(t *testing.T) {
	repo := newMockRepo()
	repo.users["admin"] = &User{ID: 1, Username: "admin", PasswordHash: mustHashPassword("pawprint123"), Status: 1}
	repo.stores[1] = []StoreInfo{{ID: 1, Name: "旗舰店", Role: "super_admin"}}

	svc := NewService(repo, "access-secret-32-chars-minimum!!", "refresh-secret-32-chars-minimum!!")
	for i := 0; i < 5; i++ {
		_, err := svc.Login("admin", "wrong-password")
		if err == nil {
			t.Fatalf("failure %d returned nil error", i+1)
		}
	}

	_, err := svc.Login("admin", "pawprint123")
	if err == nil {
		t.Fatal("expected locked account error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != errcode.Unauthenticated {
		t.Fatalf("expected unauthenticated lockout error, got %v", err)
	}
}
```

- [ ] **Step 2: Run auth test and verify RED**

```bash
cd backend && go test ./internal/module/auth -run TestLoginLocksAccountAfterFiveFailures -count=1
```

Expected: FAIL because lockout repository methods and login lockout behavior do not exist.

- [ ] **Step 3: Add auth lockout persistence**

Add fields to `User` in `backend/internal/module/auth/model.go`:

```go
FailedLoginCount int        `json:"-"`
LockedUntil      *time.Time `json:"-"`
```

Add methods to `Repository` in `backend/internal/module/auth/repo.go`:

```go
IncrementFailedLogin(userID int64, lockedUntil *time.Time) error
ResetFailedLogin(userID int64) error
```

Update the test `mockRepo` in `backend/internal/module/auth/service_test.go` with the same two methods:

```go
func (m *mockRepo) IncrementFailedLogin(userID int64, lockedUntil *time.Time) error {
	for _, u := range m.users {
		if u.ID == userID {
			u.FailedLoginCount++
			u.LockedUntil = lockedUntil
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (m *mockRepo) ResetFailedLogin(userID int64) error {
	for _, u := range m.users {
		if u.ID == userID {
			u.FailedLoginCount = 0
			u.LockedUntil = nil
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}
```

Implement them:

```go
func (r *repo) IncrementFailedLogin(userID int64, lockedUntil *time.Time) error {
	updates := map[string]interface{}{"failed_login_count": gorm.Expr("failed_login_count + 1")}
	if lockedUntil != nil {
		updates["locked_until"] = lockedUntil
	}
	return r.db.Model(&User{}).Where("id = ?", userID).Updates(updates).Error
}

func (r *repo) ResetFailedLogin(userID int64) error {
	return r.db.Model(&User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"failed_login_count": 0,
		"locked_until":       nil,
	}).Error
}
```

Create migration `backend/migrations/000003_auth_lockout_idempotency.up.sql`:

```sql
ALTER TABLE users ADD COLUMN IF NOT EXISTS failed_login_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ;
```

Create rollback `backend/migrations/000003_auth_lockout_idempotency.down.sql`:

```sql
ALTER TABLE users DROP COLUMN IF EXISTS locked_until;
ALTER TABLE users DROP COLUMN IF EXISTS failed_login_count;
```

- [ ] **Step 4: Implement lockout in `Login`**

At the start of successful user lookup in `Login`, add:

```go
if user.LockedUntil != nil && user.LockedUntil.After(time.Now().UTC()) {
	return nil, apperr.Unauthorized("账号已锁定，请10分钟后再试")
}
```

On bad password, before returning:

```go
var lockedUntil *time.Time
if user.FailedLoginCount+1 >= 5 {
	until := time.Now().UTC().Add(10 * time.Minute)
	lockedUntil = &until
}
_ = s.repo.IncrementFailedLogin(user.ID, lockedUntil)
return nil, apperr.Unauthorized("用户名或密码错误")
```

After password success, before loading stores:

```go
_ = s.repo.ResetFailedLogin(user.ID)
```

- [ ] **Step 5: Add route permission test**

Append a router-level test to `backend/internal/middleware/rbac_test.go`:

```go
func TestRequirePermissionRejectsMissingPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authSvc := auth.NewService(&fakeAuthRepo{perms: []string{"appointment:read"}}, "access", "refresh")
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", int64(10))
		c.Set("store_id", int64(1))
		c.Set("role", "front_desk")
		c.Next()
	})
	r.POST("/settlements", RequirePermission(authSvc, "settlement:create"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/settlements", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}
```

- [ ] **Step 6: Wire permissions and Redis idempotency**

Change `router.Setup` to create Redis client:

```go
rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr})
idem := middleware.Idempotency(rdb)
```

Import Redis:

```go
import "github.com/redis/go-redis/v9"
```

Update route registration signatures so business routes receive `authSvc` and `idem`. Example for settlements:

```go
settlement.RegisterRoutes(protected, setHandler, authSvc, idem)
```

Update `backend/internal/module/settlement/router.go`:

```go
func RegisterRoutes(r *gin.RouterGroup, h *Handler, authSvc *auth.Service, idem gin.HandlerFunc) {
	settlements := r.Group("/settlements")
	{
		settlements.GET("", middleware.RequirePermission(authSvc, "settlement:read"), h.List)
		settlements.POST("", middleware.RequirePermission(authSvc, "settlement:create"), h.Create)
		settlements.POST("/:id/pay", middleware.RequirePermission(authSvc, "settlement:pay"), idem, h.Pay)
		settlements.POST("/:id/refund", middleware.RequirePermission(authSvc, "settlement:refund"), idem, h.Refund)
		settlements.POST("/:id/void", middleware.RequirePermission(authSvc, "settlement:void"), h.Void)
	}
}
```

Use the same pattern for appointment, member wallet, inventory, boarding, pet, analytics, settings, and dashboard read routes.

- [ ] **Step 7: Verify security gate**

```bash
cd backend && go test ./internal/module/auth ./internal/middleware -count=1
cd backend && go test ./...
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add backend/migrations backend/internal/module/auth backend/internal/middleware backend/internal/router backend/internal/module/*/router.go
git commit -m "feat: enforce P0 auth lockout and route permissions"
```

## Task 2: Appointment Notifications

**Files:**
- Modify: `backend/internal/module/appointment/service.go`
- Modify: `backend/internal/module/appointment/service_test.go`
- Modify: `backend/internal/router/router.go`
- Modify: `backend/internal/module/appointment/router.go`
- Test: `backend/internal/module/appointment/service_test.go`

- [ ] **Step 1: Add failing notification test**

Add to appointment service test:

```go
func TestCreateAppointmentSendsConfirmationNotification(t *testing.T) {
	repo := newFakeAppointmentRepo()
	notifier := &fakeNotifier{}
	svc := NewService(repo, WithNotifier(notifier))
	start := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)

	appt, err := svc.Create(CreateAppointmentRequest{
		StoreID: 1, CustomerID: 100, PetID: 200, StationID: 10,
		ScheduledStart: start,
		Items: []CreateAppointmentItem{{ServiceOfferingID: 1, ServiceName: "全套SPA", Price: 26800, DurationMin: 90}},
	})
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}
	if appt.ScheduledEnd.Sub(start) != 90*time.Minute {
		t.Fatalf("scheduled end = %s, want 90m after start", appt.ScheduledEnd)
	}
	if len(notifier.sent) != 1 || notifier.sent[0].TemplateCode != "appointment_confirmed" {
		t.Fatalf("sent notifications = %#v", notifier.sent)
	}
}
```

- [ ] **Step 2: Run appointment test and verify RED**

```bash
cd backend && go test ./internal/module/appointment -run TestCreateAppointmentSendsConfirmationNotification -count=1
```

Expected: FAIL because `WithNotifier` and notification sending are missing.

- [ ] **Step 3: Add notifier interface and option**

In `appointment/service.go`, add:

```go
type Notifier interface {
	Send(notification.SendRequest) error
}

type Option func(*Service)

func WithNotifier(n Notifier) Option {
	return func(s *Service) { s.notifier = n }
}

type Service struct {
	repo     Repository
	notifier Notifier
}

func NewService(repo Repository, opts ...Option) *Service {
	s := &Service{repo: repo}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
```

- [ ] **Step 4: Calculate end time and send notification**

In `Create`, after `CalculateTotals`:

```go
if req.ScheduledEnd.IsZero() && durationMin > 0 {
	req.ScheduledEnd = req.ScheduledStart.Add(time.Duration(durationMin) * time.Minute)
}
```

After items are created:

```go
if s.notifier != nil && a.CustomerID != nil {
	_ = s.notifier.Send(notification.SendRequest{
		StoreID:      a.StoreID,
		CustomerID:   *a.CustomerID,
		TemplateCode: "appointment_confirmed",
		Channel:      notification.ChannelInApp,
		Payload: map[string]string{
			"appointment_id": strconv.FormatInt(a.ID, 10),
			"start_at":       a.ScheduledStart.Format(time.RFC3339),
		},
	})
}
```

- [ ] **Step 5: Wire notifier**

In `router.Setup`, construct appointment service with notification service:

```go
apptSvc := appointment.NewService(apptRepo, appointment.WithNotifier(notifSvc))
```

- [ ] **Step 6: Verify appointment stage**

```bash
cd backend && go test ./internal/module/appointment -count=1
cd backend && go test ./...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/module/appointment backend/internal/router/router.go
git commit -m "feat: send appointment confirmation notifications"
```

## Task 3: Mini-Program P0 APIs

**Files:**
- Create: `backend/internal/module/wx/model.go`
- Create: `backend/internal/module/wx/dto.go`
- Create: `backend/internal/module/wx/repo.go`
- Create: `backend/internal/module/wx/service.go`
- Create: `backend/internal/module/wx/handler.go`
- Create: `backend/internal/module/wx/router.go`
- Create: `backend/internal/module/wx/service_test.go`
- Modify: `backend/internal/router/router.go`

- [ ] **Step 1: Add failing wx service tests**

Create `backend/internal/module/wx/service_test.go`:

```go
package wx

import (
	"testing"
	"time"

	"pawprint/backend/internal/module/appointment"
)

func TestMockLoginCreatesCustomerForCode(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, nil)

	resp, err := svc.MockLogin(LoginRequest{Code: "mock-openid-001", StoreID: 1})
	if err != nil {
		t.Fatalf("MockLogin error = %v", err)
	}
	if resp.CustomerID == 0 || resp.Token == "" {
		t.Fatalf("response = %#v", resp)
	}
	if repo.customer.Source != 2 {
		t.Fatalf("customer source = %d, want 2", repo.customer.Source)
	}
}

func TestCreateAppointmentUsesAppointmentRules(t *testing.T) {
	repo := newFakeRepo()
	appt := &fakeAppointmentCreator{}
	svc := NewService(repo, appt)
	start := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)

	_, err := svc.CreateAppointment(1, CreateAppointmentRequest{
		StoreID: 1, PetID: 2, ScheduledStart: start, ServiceOfferingID: 8,
	})
	if err != nil {
		t.Fatalf("CreateAppointment error = %v", err)
	}
	if appt.req.Source != 2 || len(appt.req.Items) != 1 {
		t.Fatalf("appointment request = %#v", appt.req)
	}
}
```

- [ ] **Step 2: Run wx test and verify RED**

```bash
cd backend && go test ./internal/module/wx -count=1
```

Expected: FAIL because the package does not exist yet.

- [ ] **Step 3: Implement wx DTOs**

Create `dto.go`:

```go
package wx

import "time"

type LoginRequest struct {
	Code    string `json:"code" binding:"required"`
	StoreID int64  `json:"store_id" binding:"required"`
}

type LoginResponse struct {
	CustomerID int64  `json:"customer_id"`
	Token      string `json:"token"`
}

type ServiceOffering struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Price       int64  `json:"price"`
	DurationMin int    `json:"duration_min"`
}

type CreateAppointmentRequest struct {
	StoreID           int64     `json:"store_id" binding:"required"`
	PetID             int64     `json:"pet_id" binding:"required"`
	ServiceOfferingID int64     `json:"service_offering_id" binding:"required"`
	ScheduledStart    time.Time `json:"scheduled_start" binding:"required"`
}
```

- [ ] **Step 4: Implement wx repository and service**

Repository interface:

```go
type Repository interface {
	FindCustomerByOpenID(openID string) (*member.Customer, error)
	CreateCustomer(c *member.Customer) error
	ListBookableOfferings(storeID int64) ([]ServiceOffering, error)
	FindOffering(id, storeID int64) (*ServiceOffering, error)
}
```

Service constructor:

```go
type AppointmentCreator interface {
	Create(appointment.CreateAppointmentRequest) (*appointment.Appointment, error)
}

type Service struct {
	repo Repository
	appointments AppointmentCreator
}

func NewService(repo Repository, appointments AppointmentCreator) *Service {
	return &Service{repo: repo, appointments: appointments}
}
```

Mock login:

```go
func (s *Service) MockLogin(req LoginRequest) (*LoginResponse, error) {
	openID := req.Code
	c, err := s.repo.FindCustomerByOpenID(openID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c = &member.Customer{Name: "微信顾客", Source: 2, WechatOpenID: openID, RegisterStoreID: &req.StoreID}
		if err := s.repo.CreateCustomer(c); err != nil {
			return nil, apperr.Internal(err)
		}
	} else if err != nil {
		return nil, apperr.Internal(err)
	}
	return &LoginResponse{CustomerID: c.ID, Token: "mock-wx-" + strconv.FormatInt(c.ID, 10)}, nil
}
```

Appointment create:

```go
func (s *Service) CreateAppointment(customerID int64, req CreateAppointmentRequest) (*appointment.Appointment, error) {
	offering, err := s.repo.FindOffering(req.ServiceOfferingID, req.StoreID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return s.appointments.Create(appointment.CreateAppointmentRequest{
		StoreID: req.StoreID, CustomerID: customerID, PetID: req.PetID, Source: 2,
		ScheduledStart: req.ScheduledStart,
		Items: []appointment.CreateAppointmentItem{{
			ServiceOfferingID: offering.ID,
			ServiceName: offering.Name,
			Price: offering.Price,
			DurationMin: offering.DurationMin,
		}},
	})
}
```

- [ ] **Step 5: Implement handlers and routes**

Routes:

```go
func RegisterRoutes(r *gin.RouterGroup, h *Handler, idem gin.HandlerFunc) {
	wx := r.Group("/wx")
	wx.POST("/auth/login", h.Login)
	wx.GET("/service-offerings", h.ServiceOfferings)
	wx.POST("/appointments", idem, h.CreateAppointment)
	wx.POST("/appointments/:id/cancel", h.CancelAppointment)
}
```

Handler customer ID extraction for mock token:

```go
func customerIDFromHeader(c *gin.Context) (int64, error) {
	raw := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer mock-wx-")
	return strconv.ParseInt(raw, 10, 64)
}
```

- [ ] **Step 6: Wire wx module**

In `router.Setup`:

```go
wxRepo := wx.NewRepository(db)
wxSvc := wx.NewService(wxRepo, apptSvc)
wxHandler := wx.NewHandler(wxSvc)
wx.RegisterRoutes(v1, wxHandler, idem)
```

- [ ] **Step 7: Verify wx stage**

```bash
cd backend && go test ./internal/module/wx ./internal/module/appointment -count=1
cd backend && go test ./...
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/module/wx backend/internal/router/router.go
git commit -m "feat: add P0 mini-program appointment APIs"
```

## Task 4: Settlement Payment and Refund Side Effects

**Files:**
- Modify: `backend/internal/module/settlement/model.go`
- Modify: `backend/internal/module/settlement/repo.go`
- Modify: `backend/internal/module/settlement/service.go`
- Modify: `backend/internal/module/settlement/service_test.go`
- Modify: `backend/internal/module/member/service.go`
- Modify: `backend/internal/module/member/repo.go`
- Modify: `backend/internal/module/inventory/service.go`
- Modify: `backend/internal/module/inventory/repo.go`

- [ ] **Step 1: Add failing settlement side-effect tests**

Append to settlement service tests:

```go
func TestPayWalletSettlementDeductsWalletAndAwardsPoints(t *testing.T) {
	repo := newFakeSettlementRepo()
	memberSvc := &fakeMemberEffects{}
	inventorySvc := &fakeInventoryEffects{}
	prints := &fakePrintJobs{}
	svc := NewService(repo, WithMemberEffects(memberSvc), WithInventoryEffects(inventorySvc), WithPrintJobs(prints))

	err := svc.Pay(1, 26800, PayWallet, 9)
	if err != nil {
		t.Fatalf("Pay error = %v", err)
	}
	if memberSvc.walletAmount != 26800 || memberSvc.pointsAmount != 26800 {
		t.Fatalf("member effects = %#v", memberSvc)
	}
	if len(prints.jobs) != 1 || prints.jobs[0].RefID != 1 {
		t.Fatalf("print jobs = %#v", prints.jobs)
	}
}

func TestPayWechatReturnsPaymentNotEnabled(t *testing.T) {
	repo := newFakeSettlementRepo()
	svc := NewService(repo)
	err := svc.Pay(1, 26800, PayWechat, 9)
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != errcode.PaymentNotEnabled {
		t.Fatalf("err = %v, want payment not enabled", err)
	}
}
```

- [ ] **Step 2: Run settlement test and verify RED**

```bash
cd backend && go test ./internal/module/settlement -run 'TestPayWalletSettlementDeductsWalletAndAwardsPoints|TestPayWechatReturnsPaymentNotEnabled' -count=1
```

Expected: FAIL because effects interfaces are missing.

- [ ] **Step 3: Add effect interfaces**

In `settlement/service.go`:

```go
type MemberEffects interface {
	WalletConsume(customerID, amount, storeID, operatorID int64, remark string) error
	EarnPoints(customerID, amountPaid, storeID, operatorID int64, refType string, refID int64) (int64, error)
	ApplyPaidSpend(customerID, amountPaid, storeID, operatorID int64, refType string, refID int64) error
	ReverseSettlement(customerID, amountPaid, storeID, operatorID int64, refID int64) error
}

type InventoryEffects interface {
	SaleOut(storeID, productID int64, quantity int, operatorID int64, refType string, refID int64) error
	ReverseSaleOut(storeID, productID int64, quantity int, operatorID int64, refType string, refID int64) error
}

type PrintJobs interface {
	CreateReceipt(storeID, settlementID, operatorID int64, content map[string]interface{}) error
}
```

Add service options:

```go
type Option func(*Service)
func WithMemberEffects(e MemberEffects) Option { return func(s *Service) { s.members = e } }
func WithInventoryEffects(e InventoryEffects) Option { return func(s *Service) { s.inventory = e } }
func WithPrintJobs(p PrintJobs) Option { return func(s *Service) { s.prints = p } }
```

- [ ] **Step 4: Load settlement items and apply effects in Pay**

Add repository method:

```go
FindItems(settlementID int64) ([]SettlementItem, error)
WithTx(fn func(Repository) error) error
```

In `Pay`, wrap state update, payment, and side effects:

```go
return s.repo.WithTx(func(tx Repository) error {
	settlement, err := tx.FindByID(id)
	if err != nil { return err }
	if settlement.Status != StatusUnpaid {
		return apperr.New(errcode.StateTransitionInvalid, "仅可对未支付结算单进行收款")
	}
	if method == PayWechat || method == PayAlipay {
		return apperr.New(errcode.PaymentNotEnabled, "线上支付未开通，请选择其他方式")
	}
	if method == PayWallet && settlement.CustomerID != nil && s.members != nil {
		if err := s.members.WalletConsume(*settlement.CustomerID, amount, settlement.StoreID, operatorID, "结算消费 "+settlement.Code); err != nil {
			return err
		}
	}
	items, err := tx.FindItems(id)
	if err != nil { return apperr.Internal(err) }
	for _, item := range items {
		if item.SourceType == "product" && s.inventory != nil {
			if err := s.inventory.SaleOut(settlement.StoreID, item.SourceID, item.Quantity, operatorID, "settlement", settlement.ID); err != nil {
				return err
			}
		}
	}
	now := time.Now().UTC()
	settlement.Status = StatusPaid
	settlement.PaidAmount = amount
	settlement.PaidAt = &now
	if err := tx.Update(settlement); err != nil { return apperr.Internal(err) }
	if err := tx.CreatePayment(&Payment{SettlementID: id, Method: method, Amount: amount, Status: "success", PaidAt: &now}); err != nil {
		return apperr.Internal(err)
	}
	if settlement.CustomerID != nil && s.members != nil {
		if err := s.members.ApplyPaidSpend(*settlement.CustomerID, amount, settlement.StoreID, operatorID, "settlement", settlement.ID); err != nil {
			return err
		}
		_, _ = s.members.EarnPoints(*settlement.CustomerID, amount, settlement.StoreID, operatorID, "settlement", settlement.ID)
	}
	if s.prints != nil {
		return s.prints.CreateReceipt(settlement.StoreID, settlement.ID, operatorID, map[string]interface{}{"code": settlement.Code, "paid_amount": amount})
	}
	return nil
})
```

- [ ] **Step 5: Add member transactional helpers**

In `member.Service`, implement:

```go
func (s *Service) ApplyPaidSpend(customerID, amountPaid, storeID, operatorID int64, refType string, refID int64) error {
	c, err := s.repo.FindCustomerByID(customerID)
	if err != nil { return nil }
	c.TotalSpend += amountPaid
	if err := s.repo.UpdateCustomer(c); err != nil { return apperr.Internal(err) }
	s.CheckTierUpgrade(customerID, c.TotalSpend)
	return nil
}

func (s *Service) ReverseSettlement(customerID, amountPaid, storeID, operatorID int64, refID int64) error {
	c, err := s.repo.FindCustomerByID(customerID)
	if err != nil { return nil }
	if c.TotalSpend >= amountPaid { c.TotalSpend -= amountPaid }
	c.PointsBalance = 0
	return s.repo.UpdateCustomer(c)
}
```

Keep later refinements test-driven if exact points reversal needs per-transaction matching.

- [ ] **Step 6: Add inventory reverse sale**

In `inventory.Service`:

```go
func (s *Service) ReverseSaleOut(storeID, productID int64, quantity int, operatorID int64, refType string, refID int64) error {
	return s.PurchaseIn(storeID, productID, quantity, operatorID, refType, refID)
}
```

- [ ] **Step 7: Verify settlement stage**

```bash
cd backend && go test ./internal/module/settlement ./internal/module/member ./internal/module/inventory -count=1
cd backend && go test ./...
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/module/settlement backend/internal/module/member backend/internal/module/inventory
git commit -m "feat: apply settlement payment side effects"
```

## Task 5: Inventory Row Locks and Alerts

**Files:**
- Modify: `backend/internal/module/inventory/repo.go`
- Modify: `backend/internal/module/inventory/service.go`
- Modify: `backend/internal/module/inventory/service_test.go`
- Modify: `backend/internal/module/notification/service.go`
- Modify: `backend/internal/module/notification/service_test.go`

- [ ] **Step 1: Add failing oversell and alert tests**

Add inventory tests:

```go
func TestSaleOutCreatesStockLowAlertWhenBelowSafetyStock(t *testing.T) {
	repo := newFakeInventoryRepo(InventoryItem{ID: 1, StoreID: 1, ProductID: 10, Quantity: 6, SafetyStock: 8})
	notifier := &fakeNotifier{}
	svc := NewService(repo, WithNotifier(notifier))

	err := svc.SaleOut(1, 10, 2, 9, "settlement", 1)
	if err != nil {
		t.Fatalf("SaleOut error = %v", err)
	}
	if repo.item.Quantity != 4 {
		t.Fatalf("quantity = %d, want 4", repo.item.Quantity)
	}
	if len(notifier.sent) != 1 || notifier.sent[0].TemplateCode != "stock_low" {
		t.Fatalf("notifications = %#v", notifier.sent)
	}
}
```

- [ ] **Step 2: Run inventory test and verify RED**

```bash
cd backend && go test ./internal/module/inventory -run TestSaleOutCreatesStockLowAlertWhenBelowSafetyStock -count=1
```

Expected: FAIL because inventory has no notifier option.

- [ ] **Step 3: Add row-lock repository method**

In `inventory/repo.go`:

```go
GetInventoryForUpdate(storeID, productID int64) (*InventoryItem, error)
WithTx(fn func(Repository) error) error
```

Implementation:

```go
func (r *repo) GetInventoryForUpdate(storeID, productID int64) (*InventoryItem, error) {
	var inv InventoryItem
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("store_id = ? AND product_id = ?", storeID, productID).
		First(&inv).Error
	return &inv, err
}

func (r *repo) WithTx(fn func(Repository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return fn(&repo{db: tx})
	})
}
```

Import `gorm.io/gorm/clause`.

- [ ] **Step 4: Use locked inventory and send alerts**

Add notifier option like appointment. In `SaleOut`, run:

```go
return s.repo.WithTx(func(tx Repository) error {
	inv, err := tx.GetInventoryForUpdate(storeID, productID)
	if err != nil { return err }
	if inv.Quantity < quantity {
		return apperr.New(errcode.InsufficientStock, "库存不足，当前库存: "+itoa(inv.Quantity))
	}
	newQty := inv.Quantity - quantity
	inv.Quantity = newQty
	inv.HasAlert = newQty <= inv.SafetyStock && inv.SafetyStock > 0
	if err := tx.UpdateInventory(inv); err != nil { return apperr.Internal(err) }
	if err := tx.CreateTransaction(&StockTransaction{StoreID: storeID, ProductID: productID, Type: TxSaleOut, Quantity: -quantity, BalanceAfter: newQty, RefType: refType, RefID: refID}); err != nil {
		return apperr.Internal(err)
	}
	if inv.HasAlert && s.notifier != nil {
		return s.notifier.Send(notification.SendRequest{StoreID: storeID, TemplateCode: "stock_low", Channel: notification.ChannelInApp, Payload: map[string]string{"product_id": strconv.FormatInt(productID, 10)}})
	}
	return nil
})
```

- [ ] **Step 5: Verify inventory stage**

```bash
cd backend && go test ./internal/module/inventory ./internal/module/notification -count=1
cd backend && go test ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/module/inventory backend/internal/module/notification
git commit -m "feat: lock inventory mutations and emit stock alerts"
```

## Task 6: Boarding Checkout Settlement

**Files:**
- Modify: `backend/internal/module/boarding/service.go`
- Modify: `backend/internal/module/boarding/repo.go`
- Modify: `backend/internal/module/boarding/service_test.go`
- Modify: `backend/internal/router/router.go`

- [ ] **Step 1: Add failing checkout settlement test**

Append to boarding tests:

```go
func TestCheckOutCreatesBoardingSettlementAndCleansRoom(t *testing.T) {
	checkIn := time.Now().UTC().Add(-51 * time.Hour)
	repo := newFakeBoardingRepo()
	repo.order = &BoardingOrder{ID: 1, StoreID: 1, CustomerID: 100, PetID: 200, RoomID: ptrInt64(5), Status: StatusCheckedIn, ActualCheckIn: &checkIn, PricePerNight: 12000}
	repo.room = &BoardingRoom{ID: 5, StoreID: 1, Status: RoomStatusOccupied}
	settlements := &fakeSettlementCreator{}
	svc := NewService(repo, WithSettlementCreator(settlements))

	resp, err := svc.CheckOut(1, 1)
	if err != nil {
		t.Fatalf("CheckOut error = %v", err)
	}
	if resp.Nights != 3 || resp.TotalAmount != 36000 {
		t.Fatalf("resp = %#v", resp)
	}
	if repo.room.Status != RoomStatusCleaning {
		t.Fatalf("room status = %s", repo.room.Status)
	}
	if settlements.req.BizType != "boarding" || settlements.req.Items[0].UnitPrice*int64(settlements.req.Items[0].Quantity) != 36000 {
		t.Fatalf("settlement request = %#v", settlements.req)
	}
}
```

- [ ] **Step 2: Run boarding test and verify RED**

```bash
cd backend && go test ./internal/module/boarding -run TestCheckOutCreatesBoardingSettlementAndCleansRoom -count=1
```

Expected: FAIL because checkout does not create settlement.

- [ ] **Step 3: Add settlement creator option**

In boarding service:

```go
type SettlementCreator interface {
	Create(settlement.CreateSettlementRequest) (*settlement.Settlement, error)
}

func WithSettlementCreator(c SettlementCreator) Option {
	return func(s *Service) { s.settlements = c }
}
```

- [ ] **Step 4: Create settlement in checkout**

After calculating `totalAmount`, before returning:

```go
if s.settlements != nil {
	_, err := s.settlements.Create(settlement.CreateSettlementRequest{
		StoreID: order.StoreID,
		CustomerID: order.CustomerID,
		BizType: settlement.BizBoarding,
		Items: []settlement.SettlementItemRequest{{
			SourceType: "boarding",
			SourceID: order.ID,
			Name: "寄养服务",
			UnitPrice: order.PricePerNight,
			Quantity: nights,
		}},
	})
	if err != nil { return nil, err }
}
```

- [ ] **Step 5: Wire settlement creator**

In `router.Setup`:

```go
boardingSvc := boarding.NewService(boardingRepo, boarding.WithSettlementCreator(setSvc))
```

- [ ] **Step 6: Verify boarding stage**

```bash
cd backend && go test ./internal/module/boarding ./internal/module/settlement -count=1
cd backend && go test ./...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/module/boarding backend/internal/router/router.go
git commit -m "feat: create boarding settlement on checkout"
```

## Task 7: Daily Vaccine-Due Notification Scan

**Files:**
- Modify: `backend/internal/module/notification/repo.go`
- Modify: `backend/internal/module/notification/service.go`
- Modify: `backend/internal/module/notification/service_test.go`

- [ ] **Step 1: Add failing vaccine scan test**

Add notification test:

```go
func TestScanVaccineDueCreatesSkippedSMSWhenDisabled(t *testing.T) {
	repo := newFakeNotificationRepo()
	repo.duePets = []VaccineDuePet{{StoreID: 1, CustomerID: 100, PetID: 200, PetName: "布丁", DueAt: time.Now().UTC().Add(7 * 24 * time.Hour)}}
	svc := NewService(repo)
	svc.SetFeatureFlags(false, false)

	count, err := svc.ScanVaccineDue(time.Now().UTC(), 7)
	if err != nil {
		t.Fatalf("ScanVaccineDue error = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	if repo.logs[0].TemplateCode != "vaccine_due" || repo.logs[0].Status != StatusSkipped {
		t.Fatalf("log = %#v", repo.logs[0])
	}
}
```

- [ ] **Step 2: Run notification test and verify RED**

```bash
cd backend && go test ./internal/module/notification -run TestScanVaccineDueCreatesSkippedSMSWhenDisabled -count=1
```

Expected: FAIL because scan support is missing.

- [ ] **Step 3: Add repo query and service scan**

Add model:

```go
type VaccineDuePet struct {
	StoreID int64
	CustomerID int64
	PetID int64
	PetName string
	DueAt time.Time
}
```

Add repository method:

```go
FindVaccineDue(now time.Time, days int) ([]VaccineDuePet, error)
```

GORM query:

```go
err := r.db.Table("pet_health_records phr").
	Select("p.store_id, p.customer_id, p.id as pet_id, p.name as pet_name, phr.next_due_at as due_at").
	Joins("JOIN pets p ON p.id = phr.pet_id AND p.deleted_at IS NULL").
	Where("phr.type = ? AND phr.next_due_at >= ? AND phr.next_due_at < ?", "vaccine", now, now.AddDate(0, 0, days)).
	Scan(&rows).Error
```

Service:

```go
func (s *Service) ScanVaccineDue(now time.Time, days int) (int, error) {
	rows, err := s.repo.FindVaccineDue(now, days)
	if err != nil { return 0, apperr.Internal(err) }
	for _, row := range rows {
		if err := s.Send(SendRequest{StoreID: row.StoreID, CustomerID: row.CustomerID, TemplateCode: "vaccine_due", Channel: ChannelSMS, Payload: map[string]string{"pet_name": row.PetName, "due_at": row.DueAt.Format("2006-01-02")}}); err != nil {
			return 0, err
		}
	}
	return len(rows), nil
}
```

- [ ] **Step 4: Verify notification stage**

```bash
cd backend && go test ./internal/module/notification -count=1
cd backend && go test ./...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/module/notification
git commit -m "feat: add vaccine due notification scan"
```

## Task 8: Full Backend Acceptance Gate

**Files:**
- Modify only files needed by failures discovered in this gate.
- Do not commit admin view completion in this backend-first plan.

- [ ] **Step 1: Run all backend tests**

```bash
cd backend && go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run frontend smoke build**

```bash
cd admin && npm run build
```

Expected: PASS.

- [ ] **Step 3: Check remaining P0 markers**

```bash
rg -n "开发中|panic\\(|PAYMENT_NOT_ENABLED" backend admin/src/views
```

Expected: backend has no P0 incomplete markers except intentional payment-disabled behavior for WeChat/Alipay. Admin pages with `开发中` may remain because frontend completion is outside this backend-first plan.

- [ ] **Step 4: Commit final gate evidence if code changed**

If Step 1-3 required fixes:

```bash
git add backend admin
git commit -m "test: pass P0 backend acceptance gate"
```

If no files changed, record the passing commands in the final implementation report instead of creating an empty commit.

## Completion Criteria

- P0 security gate tests prove auth lockout, RBAC, and store isolation.
- Appointment tests prove conflict checks, state transitions, computed end time, and notification creation.
- Mini-program tests prove mock login, online service list, customer appointment creation, and cancellation cutoff.
- Settlement tests prove cash/wallet payment, disabled online payment response, wallet/points/tier/inventory/receipt side effects, refund reversal, void rules, and idempotent replay.
- Boarding tests prove check-in, checkout night calculation, generated settlement, and room cleaning status.
- Inventory tests prove stock transactions reconcile, stock-low alerts are created, and row-lock path prevents oversell.
- Notification tests prove skipped SMS behavior and vaccine-due scan logs.
- `cd backend && go test ./...` passes.
- `cd admin && npm run build` passes.
