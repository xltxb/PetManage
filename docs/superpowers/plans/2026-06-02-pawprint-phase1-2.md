# PawPrint Phase 1-2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the project skeleton (Phase 1) and authentication/security core (Phase 2) for PawPrint pet store management SaaS.

**Architecture:** Go 1.22+ backend with Gin + GORM + PostgreSQL, following the module-per-feature pattern (`internal/module/<name>/`). Each module has handler → service (interface) → repo (interface) layers. TDD: write failing test → minimal implementation → refactor. Infrastructure middleware (auth, RBAC, store-scope) gates all business endpoints.

**Tech Stack:** Go 1.22+, Gin, GORM, golang-migrate, PostgreSQL 15+, Redis 7+, JWT (golang-jwt), bcrypt, Docker Compose

**Spec:** `docs/superpowers/specs/2026-06-02-pawprint-rebuild-design.md`
**Source docs:** `files/PawPrint宠物店SaaS开发文档.md`, `files/schema.sql`, `files/seed.sql`

---

## File Map (Phase 1 + 2 complete)

```
backend/
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
├── docker-compose.yml
├── .env.example
├── cmd/server/main.go
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   └── config_test.go
│   ├── pkg/
│   │   ├── money/money.go + money_test.go
│   │   ├── response/response.go + response_test.go
│   │   ├── errcode/errcode.go
│   │   ├── apperr/apperr.go + apperr_test.go
│   │   ├── pagination/pagination.go + pagination_test.go
│   │   ├── validator/validator.go
│   │   ├── timeutil/timeutil.go + timeutil_test.go
│   │   └── dbutil/dbutil.go
│   ├── middleware/
│   │   ├── traceid.go
│   │   ├── logger.go
│   │   ├── recovery.go
│   │   ├── cors.go
│   │   ├── auth.go
│   │   ├── rbac.go
│   │   ├── storescope.go
│   │   ├── ratelimit.go
│   │   └── idempotency.go
│   ├── module/
│   │   └── auth/
│   │       ├── model.go, dto.go, repo.go
│   │       ├── service.go, service_test.go
│   │       ├── handler.go, handler_test.go
│   │       └── router.go
│   └── router/router.go
├── migrations/
│   ├── 000001_init_schema.up.sql
│   ├── 000001_init_schema.down.sql
│   ├── 000002_seed_data.up.sql
│   └── 000002_seed_data.down.sql
└── config/config.yaml
```

---

## Phase 1: Skeleton

### Task 1: Initialize Go module and project directories

**Files:**
- Create: `backend/go.mod`
- Create: `backend/.env.example`
- Create: `backend/.gitignore`

- [ ] **Step 1: Create backend directory and initialize Go module**

```bash
mkdir -p backend && cd backend && go mod init pawprint/backend
```

Expected: `go.mod` created with `module pawprint/backend` and go version.

- [ ] **Step 2: Create directory structure**

```bash
cd backend
mkdir -p cmd/server
mkdir -p internal/{config,pkg/{money,response,errcode,apperr,pagination,validator,timeutil,dbutil},middleware,module/auth,router}
mkdir -p migrations
mkdir -p config
```

- [ ] **Step 3: Create .env.example**

Write `backend/.env.example`:
```env
APP_ENV=dev
HTTP_PORT=8080
DB_DSN=postgres://pawprint:pawprint@localhost:5432/pawprint?sslmode=disable
REDIS_ADDR=localhost:6379
JWT_ACCESS_SECRET=change-me-access-secret
JWT_REFRESH_SECRET=change-me-refresh-secret
JWT_ACCESS_TTL=2h
JWT_REFRESH_TTL=720h
DEFAULT_TIMEZONE=Asia/Shanghai
```

- [ ] **Step 4: Create .gitignore**

Write `backend/.gitignore`:
```
.env
*.log
tmp/
vendor/
```

- [ ] **Step 5: Install dependencies**

```bash
cd backend
go get github.com/gin-gonic/gin
go get gorm.io/gorm
go get gorm.io/driver/postgres
go get github.com/golang-jwt/jwt/v5
go get golang.org/x/crypto/bcrypt
go get github.com/redis/go-redis/v9
go get github.com/golang-migrate/migrate/v4
```

- [ ] **Step 6: Commit**

```bash
git add backend/
git commit -m "feat(phase1): initialize Go module and project directory structure

- Set up pawprint/backend Go module
- Create internal/ directory tree (config, pkg, middleware, module, router)
- Add .env.example with required environment variables
- Install core dependencies (Gin, GORM, JWT, bcrypt, Redis, migrate)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 2: Money package — cents/yuan conversion

**Files:**
- Create: `backend/internal/pkg/money/money.go`
- Create: `backend/internal/pkg/money/money_test.go`

- [ ] **Step 1: Write the failing test**

Write `backend/internal/pkg/money/money_test.go`:
```go
package money

import (
	"testing"
)

func TestToYuan(t *testing.T) {
	tests := []struct {
		name   string
		cents  int64
		expect string
	}{
		{"zero", 0, "0.00"},
		{"one yuan", 100, "1.00"},
		{"268 yuan", 26800, "268.00"},
		{"negative", -100, "-1.00"},
		{"fifty cents", 50, "0.50"},
		{"round number", 10000, "100.00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToYuan(tt.cents)
			if got != tt.expect {
				t.Errorf("ToYuan(%d) = %q, want %q", tt.cents, got, tt.expect)
			}
		})
	}
}

func TestToCents(t *testing.T) {
	tests := []struct {
		name    string
		yuan    string
		expect  int64
		wantErr bool
	}{
		{"zero", "0.00", 0, false},
		{"one yuan", "1.00", 100, false},
		{"268 yuan", "268.00", 26800, false},
		{"with decimal", "12.50", 1250, false},
		{"invalid format", "abc", 0, true},
		{"empty", "", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToCents(tt.yuan)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ToCents(%q) expected error, got nil", tt.yuan)
				}
				return
			}
			if err != nil {
				t.Errorf("ToCents(%q) unexpected error: %v", tt.yuan, err)
			}
			if got != tt.expect {
				t.Errorf("ToCents(%q) = %d, want %d", tt.yuan, got, tt.expect)
			}
		})
	}
}

func TestToYuanInt(t *testing.T) {
	tests := []struct {
		name   string
		cents  int64
		expect string
	}{
		{"even", 200, "2.00"},
		{"round up", 201, "2.01"},
		{"round down", 199, "1.99"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToYuan(tt.cents)
			if got != tt.expect {
				t.Errorf("ToYuan(%d) = %q, want %q", tt.cents, got, tt.expect)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd backend && go test ./internal/pkg/money/...
```

Expected: FAIL — "undefined: ToYuan", "undefined: ToCents"

- [ ] **Step 3: Write minimal implementation**

Write `backend/internal/pkg/money/money.go`:
```go
package money

import (
	"fmt"
	"strconv"
	"strings"
)

// ToYuan converts an amount in cents (bigint) to a display string in yuan.
// e.g., 26800 → "268.00", -100 → "-1.00"
func ToYuan(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	yuan := cents / 100
	frac := cents % 100
	return fmt.Sprintf("%s%d.%02d", sign, yuan, frac)
}

// ToCents converts a yuan string to cents (bigint).
// e.g., "268.00" → 26800
func ToCents(yuan string) (int64, error) {
	yuan = strings.TrimSpace(yuan)
	if yuan == "" {
		return 0, fmt.Errorf("empty amount string")
	}
	parts := strings.Split(yuan, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid amount format: %s", yuan)
	}
	intPart, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount: %s", yuan)
	}
	var fracPart int64
	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) == 1 {
			frac += "0"
		}
		if len(frac) > 2 {
			return 0, fmt.Errorf("invalid fractional part: %s", yuan)
		}
		fracPart, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid amount: %s", yuan)
		}
	}
	negative := intPart < 0
	if negative {
		intPart = -intPart
	}
	cents := intPart*100 + fracPart
	if negative {
		cents = -cents
	}
	return cents, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd backend && go test ./internal/pkg/money/... -v
```

Expected: PASS — all test cases pass.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/pkg/money/
git commit -m "feat(phase1): add money package for cents/yuan conversion

- ToYuan: bigint cents → display string (26800 → 268.00)
- ToCents: yuan string → bigint cents (268.00 → 26800)
- All amounts stored as bigint (cents), display layer divides by 100

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 3: Errcode + Apperr packages — error handling foundation

**Files:**
- Create: `backend/internal/pkg/errcode/errcode.go`
- Create: `backend/internal/pkg/apperr/apperr.go`
- Create: `backend/internal/pkg/apperr/apperr_test.go`

- [ ] **Step 1: Write error codes constant file**

Write `backend/internal/pkg/errcode/errcode.go`:
```go
package errcode

// Error codes as defined in dev doc §17.1
const (
	Success = 0

	// Auth errors (1xxx)
	Unauthenticated = 1001 // Token missing/invalid/expired
	Forbidden       = 1002 // Insufficient permissions
	StoreForbidden  = 1003 // Cross-store access denied

	// Client errors (2xxx)
	BadRequest     = 2001 // Request parameter validation failed
	NotFound       = 2002 // Resource not found

	// Business logic errors (3xxx)
	StateTransitionInvalid = 3001 // Invalid state machine transition
	ResourceConflict       = 3002 // Resource time slot conflict
	InsufficientStock      = 3003 // Not enough inventory
	InsufficientWallet     = 3004 // Insufficient stored value balance

	// Payment errors (4xxx)
	PaymentNotEnabled = 4001 // Payment gateway not enabled

	// Server errors (5xxx)
	InternalError = 5000 // Internal server error
)

// Message returns the default Chinese message for a code.
func Message(code int) string {
	switch code {
	case Success:
		return "ok"
	case Unauthenticated:
		return "未认证或Token已失效"
	case Forbidden:
		return "无操作权限"
	case StoreForbidden:
		return "跨门店访问被拒"
	case BadRequest:
		return "参数校验失败"
	case NotFound:
		return "资源不存在"
	case StateTransitionInvalid:
		return "状态不可变更"
	case ResourceConflict:
		return "资源时段冲突"
	case InsufficientStock:
		return "库存不足"
	case InsufficientWallet:
		return "储值余额不足"
	case PaymentNotEnabled:
		return "线上支付未开通"
	case InternalError:
		return "服务器内部错误"
	default:
		return "未知错误"
	}
}
```

- [ ] **Step 2: Write the apperr type**

Write `backend/internal/pkg/apperr/apperr.go`:
```go
package apperr

import (
	"fmt"
	"net/http"

	"pawprint/backend/internal/pkg/errcode"
)

// AppError is an application-level error with code and HTTP status.
type AppError struct {
	Code       int    `json:"code"`
	HTTPStatus int    `json:"-"`
	Message    string `json:"message"`
	Err        error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Err }

// HTTPStatusForCode maps an application error code to HTTP status.
func HTTPStatusForCode(code int) int {
	switch code {
	case errcode.Unauthenticated:
		return http.StatusUnauthorized
	case errcode.Forbidden, errcode.StoreForbidden:
		return http.StatusForbidden
	case errcode.BadRequest:
		return http.StatusBadRequest
	case errcode.NotFound:
		return http.StatusNotFound
	case errcode.StateTransitionInvalid:
		return http.StatusConflict
	case errcode.ResourceConflict, errcode.InsufficientStock, errcode.InsufficientWallet:
		return http.StatusUnprocessableEntity
	case errcode.PaymentNotEnabled:
		return http.StatusNotImplemented
	case errcode.InternalError:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// New creates an AppError from a code and optional message.
// If msg is empty, uses the default message from errcode package.
func New(code int, msg ...string) *AppError {
	message := errcode.Message(code)
	if len(msg) > 0 && msg[0] != "" {
		message = msg[0]
	}
	return &AppError{
		Code:       code,
		HTTPStatus: HTTPStatusForCode(code),
		Message:    message,
	}
}

// Wrap wraps an error with an error code.
func Wrap(err error, code int, msg ...string) *AppError {
	ae := New(code, msg...)
	ae.Err = err
	return ae
}

// BadRequest creates a 400/2001 validation error.
func BadRequest(msg string) *AppError {
	return New(errcode.BadRequest, msg)
}

// NotFound creates a 404/2002 not found error.
func NotFound(msg string) *AppError {
	return New(errcode.NotFound, msg)
}

// Unauthorized creates a 401/1001 auth error.
func Unauthorized(msg ...string) *AppError {
	return New(errcode.Unauthenticated, msg...)
}

// Forbidden creates a 403/1002 forbidden error.
func Forbidden(msg ...string) *AppError {
	return New(errcode.Forbidden, msg...)
}

// Internal creates a 500/5000 internal error. The raw error is preserved
// for logging but not exposed to the client.
func Internal(err error) *AppError {
	return Wrap(err, errcode.InternalError)
}
```

- [ ] **Step 3: Write apperr tests**

Write `backend/internal/pkg/apperr/apperr_test.go`:
```go
package apperr

import (
	"errors"
	"net/http"
	"testing"

	"pawprint/backend/internal/pkg/errcode"
)

func TestNew(t *testing.T) {
	ae := New(errcode.InsufficientStock)
	if ae.Code != errcode.InsufficientStock {
		t.Errorf("Code = %d, want %d", ae.Code, errcode.InsufficientStock)
	}
	if ae.HTTPStatus != http.StatusUnprocessableEntity {
		t.Errorf("HTTPStatus = %d, want %d", ae.HTTPStatus, http.StatusUnprocessableEntity)
	}
	if ae.Message != "库存不足" {
		t.Errorf("Message = %q, want %q", ae.Message, "库存不足")
	}
}

func TestNewCustomMessage(t *testing.T) {
	ae := New(errcode.InsufficientStock, "皇家幼犬粮 库存不足")
	if ae.Message != "皇家幼犬粮 库存不足" {
		t.Errorf("Message = %q", ae.Message)
	}
}

func TestWrap(t *testing.T) {
	cause := errors.New("db connection refused")
	ae := Wrap(cause, errcode.InternalError)
	if ae.Code != errcode.InternalError {
		t.Errorf("Code = %d", ae.Code)
	}
	if !errors.Is(ae, cause) {
		t.Error("errors.Is should find the wrapped error")
	}
	if ae.Error() != "[5000] 服务器内部错误: db connection refused" {
		t.Errorf("Error() = %q", ae.Error())
	}
}

func TestHelperConstructors(t *testing.T) {
	tests := []struct {
		name     string
		ae       *AppError
		wantCode int
		wantHTTP int
	}{
		{"BadRequest", BadRequest("field required"), errcode.BadRequest, http.StatusBadRequest},
		{"NotFound", NotFound("user 99"), errcode.NotFound, http.StatusNotFound},
		{"Unauthorized", Unauthorized(), errcode.Unauthenticated, http.StatusUnauthorized},
		{"Forbidden", Forbidden(), errcode.Forbidden, http.StatusForbidden},
		{"Internal", Internal(errors.New("boom")), errcode.InternalError, http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.ae.Code != tt.wantCode {
				t.Errorf("Code = %d, want %d", tt.ae.Code, tt.wantCode)
			}
			if tt.ae.HTTPStatus != tt.wantHTTP {
				t.Errorf("HTTPStatus = %d, want %d", tt.ae.HTTPStatus, tt.wantHTTP)
			}
		})
	}
}
```

- [ ] **Step 4: Run tests**

```bash
cd backend && go test ./internal/pkg/apperr/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/pkg/errcode/ backend/internal/pkg/apperr/
git commit -m "feat(phase1): add errcode and apperr packages

- errcode: 13 error codes from dev doc §17.1 with Chinese messages
- apperr: AppError type with code/HTTP status/message/cause chain
- Helper constructors: BadRequest, NotFound, Unauthorized, Forbidden, Internal
- HTTPStatusForCode maps business codes to HTTP status codes

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 4: Response package — unified API response envelope

**Files:**
- Create: `backend/internal/pkg/response/response.go`
- Create: `backend/internal/pkg/response/response_test.go`

- [ ] **Step 1: Write the failing test**

Write `backend/internal/pkg/response/response_test.go`:
```go
package response

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/errcode"
)

func TestSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := createTestContext(w)

	Success(c, map[string]string{"key": "value"})

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var body Response
	json.Unmarshal(w.Body.Bytes(), &body)
	if body.Code != 0 {
		t.Errorf("code = %d, want 0", body.Code)
	}
	if body.Message != "ok" {
		t.Errorf("message = %q, want ok", body.Message)
	}
}

func TestSuccessCreated(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := createTestContext(w)

	SuccessCreated(c, nil)

	if w.Code != 201 {
		t.Errorf("status = %d, want 201", w.Code)
	}
}

func TestError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := createTestContext(w)

	Error(c, apperr.New(errcode.InsufficientStock))

	if w.Code != 422 {
		t.Errorf("status = %d, want 422", w.Code)
	}
	var body Response
	json.Unmarshal(w.Body.Bytes(), &body)
	if body.Code != errcode.InsufficientStock {
		t.Errorf("code = %d, want %d", body.Code, errcode.InsufficientStock)
	}
}

func TestList(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := createTestContext(w)

	items := []string{"a", "b"}
	List(c, items, int64(len(items)), 1, 20)

	var body ListResponse
	json.Unmarshal(w.Body.Bytes(), &body)
	if body.Code != 0 {
		t.Errorf("code = %d", body.Code)
	}
	if body.Data.Total != 2 {
		t.Errorf("total = %d, want 2", body.Data.Total)
	}
	if body.Data.Page != 1 {
		t.Errorf("page = %d, want 1", body.Data.Page)
	}
	list, ok := body.Data.List.([]interface{})
	if !ok || len(list) != 2 {
		t.Error("list should contain 2 items")
	}
}
```

This test requires a `createTestContext` helper. Let's use the gin test context:

Write `backend/internal/pkg/response/response_test.go` (complete, replacing above skeleton):
```go
package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/errcode"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func createTestContext(w *httptest.ResponseRecorder) (*gin.Context, *gin.Engine) {
	_ = gin.New()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	return c, nil
}

func TestSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := createTestContext(w)

	Success(c, map[string]string{"key": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var body Response
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != 0 {
		t.Errorf("code = %d, want 0", body.Code)
	}
}

func TestSuccessCreated(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := createTestContext(w)

	SuccessCreated(c, nil)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
}

func TestError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := createTestContext(w)

	Error(c, apperr.New(errcode.InsufficientStock))

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
	var body Response
	json.Unmarshal(w.Body.Bytes(), &body)
	if body.Code != errcode.InsufficientStock {
		t.Errorf("code = %d, want %d", body.Code, errcode.InsufficientStock)
	}
}

func TestPagination(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := createTestContext(w)

	items := []string{"a", "b"}
	List(c, items, int64(len(items)), 1, 20)

	var body ListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != 0 {
		t.Errorf("code = %d", body.Code)
	}
	if body.Data.Total != 2 {
		t.Errorf("total = %d, want 2", body.Data.Total)
	}
}

func TestNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := createTestContext(w)

	NoContent(c)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd backend && go get github.com/gin-gonic/gin && go test ./internal/pkg/response/... -v
```

Expected: FAIL — "undefined: Success", etc.

- [ ] **Step 3: Write implementation**

Write `backend/internal/pkg/response/response.go`:
```go
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/pkg/apperr"
)

// Response is the unified API response envelope.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ListResult holds paginated list data.
type ListResult struct {
	List     interface{} `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// ListResponse wraps a paginated list in the standard envelope.
type ListResponse struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    ListResult `json:"data"`
}

// Success sends a 200 OK response.
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

// SuccessCreated sends a 201 Created response.
func SuccessCreated(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

// NoContent sends a 204 No Content response.
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Error sends an error response based on the AppError.
func Error(c *gin.Context, ae *apperr.AppError) {
	c.AbortWithStatusJSON(ae.HTTPStatus, Response{
		Code:    ae.Code,
		Message: ae.Message,
	})
}

// List sends a paginated list response.
func List(c *gin.Context, list interface{}, total int64, page, pageSize int) {
	c.JSON(http.StatusOK, ListResponse{
		Code:    0,
		Message: "ok",
		Data: ListResult{
			List:     list,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}
```

- [ ] **Step 4: Run tests**

```bash
cd backend && go test ./internal/pkg/response/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/pkg/response/
git commit -m "feat(phase1): add unified API response package

- Response envelope: {code, message, data}
- Success (200), SuccessCreated (201), NoContent (204)
- Error maps AppError to correct HTTP status
- List wraps paginated data with total/page/page_size
- Matches dev doc §9.1 unified response convention

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 5: Pagination + Validator + Timeutil packages

**Files:**
- Create: `backend/internal/pkg/pagination/pagination.go`
- Create: `backend/internal/pkg/pagination/pagination_test.go`
- Create: `backend/internal/pkg/validator/validator.go`
- Create: `backend/internal/pkg/timeutil/timeutil.go`
- Create: `backend/internal/pkg/timeutil/timeutil_test.go`

- [ ] **Step 1: Write pagination with tests (TDD)**

Write `backend/internal/pkg/pagination/pagination_test.go`:
```go
package pagination

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func TestParseDefaults(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/?page=&page_size=", nil)

	page, pageSize := Parse(c)
	if page != 1 {
		t.Errorf("default page = %d, want 1", page)
	}
	if pageSize != 20 {
		t.Errorf("default pageSize = %d, want 20", pageSize)
	}
}

func TestParseValues(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/?page=3&page_size=10", nil)

	page, pageSize := Parse(c)
	if page != 3 {
		t.Errorf("page = %d, want 3", page)
	}
	if pageSize != 10 {
		t.Errorf("pageSize = %d, want 10", pageSize)
	}
}

func TestParseMaxPageSize(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/?page_size=500", nil)

	_, pageSize := Parse(c)
	if pageSize != 100 {
		t.Errorf("pageSize = %d, want 100 (max)", pageSize)
	}
}

func TestOffset(t *testing.T) {
	if got := Offset(1, 20); got != 0 {
		t.Errorf("Offset(1,20) = %d, want 0", got)
	}
	if got := Offset(3, 10); got != 20 {
		t.Errorf("Offset(3,10) = %d, want 20", got)
	}
}
```

- [ ] **Step 2: Run pagination test (fail), then implement**

Run: `cd backend && go test ./internal/pkg/pagination/... -v`
Expected: FAIL.

Write `backend/internal/pkg/pagination/pagination.go`:
```go
package pagination

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// Parse extracts page and page_size from query parameters with defaults.
func Parse(c *gin.Context) (page int, pageSize int) {
	page = DefaultPage
	pageSize = DefaultPageSize

	if v, err := strconv.Atoi(c.Query("page")); err == nil && v > 0 {
		page = v
	}
	if v, err := strconv.Atoi(c.Query("page_size")); err == nil && v > 0 {
		if v > MaxPageSize {
			v = MaxPageSize
		}
		pageSize = v
	}
	return
}

// Offset calculates the database offset for a page and page size.
func Offset(page, pageSize int) int {
	return (page - 1) * pageSize
}
```

Run: `cd backend && go test ./internal/pkg/pagination/... -v`
Expected: PASS.

- [ ] **Step 3: Write validator package**

Write `backend/internal/pkg/validator/validator.go`:
```go
package validator

import (
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// Register adds custom validators to the Gin binding engine.
func Register() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = v.RegisterValidation("phone", validatePhone)
		_ = v.RegisterValidation("storeid", validateStoreID)
	}
}

// validatePhone checks that a field looks like a Chinese mobile number (11 digits starting with 1).
func validatePhone(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	if len(s) != 11 {
		return false
	}
	if s[0] != '1' {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// validateStoreID checks that X-Store-Id is present and non-empty when required.
func validateStoreID(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	return s != ""
}
```

- [ ] **Step 4: Write timeutil with tests**

Write `backend/internal/pkg/timeutil/timeutil_test.go`:
```go
package timeutil

import (
	"testing"
	"time"
)

func TestStartOfDayUTC(t *testing.T) {
	// 2026-06-02 15:30:00 UTC
	tm := time.Date(2026, 6, 2, 15, 30, 0, 0, time.UTC)
	start := StartOfDay(tm, "Asia/Shanghai")

	// Asia/Shanghai is UTC+8, so start of day in Shanghai = previous day 16:00 UTC
	expected := time.Date(2026, 6, 1, 16, 0, 0, 0, time.UTC)
	if !start.Equal(expected) {
		t.Errorf("StartOfDay = %v, want %v", start, expected)
	}
}

func TestEndOfDayUTC(t *testing.T) {
	tm := time.Date(2026, 6, 2, 15, 30, 0, 0, time.UTC)
	end := EndOfDay(tm, "Asia/Shanghai")

	expected := time.Date(2026, 6, 2, 16, 0, 0, 0, time.UTC)
	if !end.Equal(expected) {
		t.Errorf("EndOfDay = %v, want %v", end)
	}
}

func TestFormatISO(t *testing.T) {
	tm := time.Date(2026, 6, 2, 15, 30, 0, 0, time.UTC)
	got := FormatISO(tm)
	if got != "2026-06-02T15:30:00Z" {
		t.Errorf("FormatISO = %q, want %q", got, "2026-06-02T15:30:00Z")
	}
}
```

Write `backend/internal/pkg/timeutil/timeutil.go`:
```go
package timeutil

import (
	"time"
)

const (
	DefaultTimezone = "Asia/Shanghai"
	ISO8601         = "2006-01-02T15:04:05Z07:00"
)

var shanghaiLoc *time.Location

func init() {
	var err error
	shanghaiLoc, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		shanghaiLoc = time.FixedZone("CST", 8*3600)
	}
}

// StartOfDay returns the start of the calendar day in the given timezone,
// converted back to UTC. Used for "today" queries.
func StartOfDay(t time.Time, tz string) time.Time {
	loc := getLocation(tz)
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc).UTC()
}

// EndOfDay returns the end of the calendar day (exclusive) in the given timezone,
// converted back to UTC.
func EndOfDay(t time.Time, tz string) time.Time {
	return StartOfDay(t, tz).Add(24 * time.Hour)
}

// FormatISO formats a time as ISO8601 with timezone.
func FormatISO(t time.Time) string {
	return t.Format(ISO8601)
}

// NowUTC returns the current time in UTC.
func NowUTC() time.Time {
	return time.Now().UTC()
}

func getLocation(tz string) *time.Location {
	if tz == "" {
		return shanghaiLoc
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return shanghaiLoc
	}
	return loc
}
```

Run: `cd backend && go test ./internal/pkg/timeutil/... -v`
Expected: PASS.

- [ ] **Step 5: Write dbutil helper**

Write `backend/internal/pkg/dbutil/dbutil.go`:
```go
package dbutil

import "gorm.io/gorm"

// TxFunc is a function that runs within a database transaction.
type TxFunc func(tx *gorm.DB) error

// WithTransaction executes fn within a new DB transaction.
// Rolls back on error, commits on success.
func WithTransaction(db *gorm.DB, fn TxFunc) error {
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}
```

- [ ] **Step 6: Run all pkg tests**

```bash
cd backend && go test ./internal/pkg/... -v
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/pkg/pagination/ backend/internal/pkg/validator/ backend/internal/pkg/timeutil/ backend/internal/pkg/dbutil/
git commit -m "feat(phase1): add pagination, validator, timeutil, and dbutil packages

- pagination: Parse page/page_size from query (default 1/20, max 100), Offset helper
- validator: Custom validators for phone (11-digit Chinese mobile) and storeid
- timeutil: StartOfDay/EndOfDay in store timezone, ISO8601 formatting
- dbutil: WithTransaction helper for GORM transaction management

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 6: Configuration system

**Files:**
- Create: `backend/internal/config/config.go`
- Create: `backend/internal/config/config_test.go`
- Create: `backend/config/config.yaml`

- [ ] **Step 1: Write config test (TDD)**

Write `backend/internal/config/config_test.go`:
```go
package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("APP_ENV", "test")
	os.Setenv("HTTP_PORT", "9090")
	os.Setenv("DB_DSN", "postgres://test:test@localhost:5432/testdb?sslmode=disable")
	os.Setenv("REDIS_ADDR", "localhost:6380")
	os.Setenv("JWT_ACCESS_SECRET", "test-access")
	os.Setenv("JWT_REFRESH_SECRET", "test-refresh")
	os.Setenv("JWT_ACCESS_TTL", "15m")
	os.Setenv("JWT_REFRESH_TTL", "168h")
	os.Setenv("DEFAULT_TIMEZONE", "Asia/Shanghai")
	defer func() {
		for _, k := range []string{
			"APP_ENV", "HTTP_PORT", "DB_DSN", "REDIS_ADDR",
			"JWT_ACCESS_SECRET", "JWT_REFRESH_SECRET",
			"JWT_ACCESS_TTL", "JWT_REFRESH_TTL", "DEFAULT_TIMEZONE",
		} {
			os.Unsetenv(k)
		}
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.HTTPPort != "9090" {
		t.Errorf("HTTPPort = %q, want 9090", cfg.HTTPPort)
	}
	if cfg.DB.DSN != "postgres://test:test@localhost:5432/testdb?sslmode=disable" {
		t.Errorf("DB DSN mismatch")
	}
	if cfg.JWT.AccessTTL != 15*time.Minute {
		t.Errorf("AccessTTL = %v", cfg.JWT.AccessTTL)
	}
	if cfg.JWT.RefreshTTL != 168*time.Hour {
		t.Errorf("RefreshTTL = %v", cfg.JWT.RefreshTTL)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	os.Clearenv()
	_, err := Load("")
	if err == nil {
		t.Error("expected error for missing DB_DSN, got nil")
	}
}

func TestDefaultValues(t *testing.T) {
	os.Setenv("DB_DSN", "postgres://localhost/test")
	os.Setenv("JWT_ACCESS_SECRET", "secret")
	os.Setenv("JWT_REFRESH_SECRET", "refresh")
	os.Setenv("REDIS_ADDR", "localhost:6379")
	defer os.Clearenv()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.HTTPPort != "8080" {
		t.Errorf("default HTTPPort = %q, want 8080", cfg.HTTPPort)
	}
	if cfg.AppEnv != "dev" {
		t.Errorf("default AppEnv = %q, want dev", cfg.AppEnv)
	}
}
```

- [ ] **Step 2: Run test (fail)**

```bash
cd backend && go test ./internal/config/... -v
```
Expected: FAIL — package not yet created.

- [ ] **Step 3: Write config implementation**

Write `backend/internal/config/config.go`:
```go
package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds all application configuration.
type Config struct {
	AppEnv   string
	HTTPPort string
	DB       DBConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Timezone string

	// Feature flags
	FeatureSMSEnabled     bool
	FeatureWechatEnabled  bool
	FeatureOnlineBooking  bool
}

type DBConfig struct {
	DSN string
}

type RedisConfig struct {
	Addr string
}

type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
}

// Load reads configuration from environment variables.
// configPath is reserved for future YAML file loading.
func Load(configPath string) (*Config, error) {
	cfg := &Config{
		AppEnv:   getEnv("APP_ENV", "dev"),
		HTTPPort: getEnv("HTTP_PORT", "8080"),
		DB: DBConfig{
			DSN: os.Getenv("DB_DSN"),
		},
		Redis: RedisConfig{
			Addr: getEnv("REDIS_ADDR", "localhost:6379"),
		},
		JWT: JWTConfig{
			AccessSecret:  os.Getenv("JWT_ACCESS_SECRET"),
			RefreshSecret: os.Getenv("JWT_REFRESH_SECRET"),
		},
		Timezone: getEnv("DEFAULT_TIMEZONE", "Asia/Shanghai"),

		FeatureSMSEnabled:    getEnvBool("FEATURE_SMS_ENABLED", false),
		FeatureWechatEnabled: getEnvBool("FEATURE_WECHAT_ENABLED", false),
		FeatureOnlineBooking: getEnvBool("FEATURE_ONLINE_BOOKING_ENABLED", true),
	}

	// Parse TTLs
	if v := os.Getenv("JWT_ACCESS_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid JWT_ACCESS_TTL: %w", err)
		}
		cfg.JWT.AccessTTL = d
	} else {
		cfg.JWT.AccessTTL = 2 * time.Hour
	}

	if v := os.Getenv("JWT_REFRESH_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid JWT_REFRESH_TTL: %w", err)
		}
		cfg.JWT.RefreshTTL = d
	} else {
		cfg.JWT.RefreshTTL = 720 * time.Hour
	}

	// Validate required
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.DB.DSN == "" {
		return fmt.Errorf("DB_DSN is required")
	}
	if c.JWT.AccessSecret == "" {
		return fmt.Errorf("JWT_ACCESS_SECRET is required")
	}
	if c.JWT.RefreshSecret == "" {
		return fmt.Errorf("JWT_REFRESH_SECRET is required")
	}
	if c.Redis.Addr == "" {
		return fmt.Errorf("REDIS_ADDR is required")
	}
	return nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	return v == "true" || v == "1" || v == "yes"
}
```

- [ ] **Step 4: Run tests**

```bash
cd backend && go test ./internal/config/... -v
```

Expected: PASS.

- [ ] **Step 5: Create config.yaml placeholder**

Write `backend/config/config.yaml`:
```yaml
# PawPrint backend configuration
# Values here serve as defaults; environment variables take precedence.
app_env: dev
http_port: "8080"
default_timezone: Asia/Shanghai
```

- [ ] **Step 6: Commit**

```bash
git add backend/internal/config/ backend/config/config.yaml
git commit -m "feat(phase1): add configuration system

- Config struct loads from environment variables with sensible defaults
- Required validation: DB_DSN, JWT secrets, REDIS_ADDR must be set
- JWT TTL parsing with defaults (access=2h, refresh=720h)
- Feature flags: SMS, WeChat, OnlineBooking (disabled by default)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 7: Database migrations — schema and seed

**Files:**
- Create: `backend/migrations/000001_init_schema.up.sql`
- Create: `backend/migrations/000001_init_schema.down.sql`
- Create: `backend/migrations/000002_seed_data.up.sql`
- Create: `backend/migrations/000002_seed_data.down.sql`

- [ ] **Step 1: Copy schema.sql as the first migration up**

Write `backend/migrations/000001_init_schema.up.sql` by copying from `files/schema.sql`:
```sql
-- PawPrint initial schema (29 tables + triggers + indexes)
-- Source: files/schema.sql

SET client_encoding = 'UTF8';

-- ---------- updated_at trigger ----------
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END; $$ LANGUAGE plpgsql;

-- ========== stores ==========
CREATE TABLE stores (
  id           BIGSERIAL PRIMARY KEY,
  code         VARCHAR(32)  NOT NULL UNIQUE,
  name         VARCHAR(64)  NOT NULL,
  timezone     VARCHAR(40)  NOT NULL DEFAULT 'Asia/Shanghai',
  phone        VARCHAR(20),
  address      VARCHAR(255),
  status       SMALLINT     NOT NULL DEFAULT 1,
  created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
  deleted_at   TIMESTAMPTZ
);

-- ========== users ==========
CREATE TABLE users (
  id            BIGSERIAL PRIMARY KEY,
  username      VARCHAR(64)  NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  display_name  VARCHAR(64)  NOT NULL,
  phone         VARCHAR(20)  UNIQUE,
  avatar_text   VARCHAR(4),
  status        SMALLINT     NOT NULL DEFAULT 1,
  last_store_id BIGINT REFERENCES stores(id),
  created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
  deleted_at    TIMESTAMPTZ
);

-- ========== roles ==========
CREATE TABLE roles (
  id        BIGSERIAL PRIMARY KEY,
  code      VARCHAR(32) NOT NULL UNIQUE,
  name      VARCHAR(32) NOT NULL,
  is_system BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ========== permissions ==========
CREATE TABLE permissions (
  id     BIGSERIAL PRIMARY KEY,
  code   VARCHAR(64) NOT NULL UNIQUE,
  module VARCHAR(32) NOT NULL,
  name   VARCHAR(64) NOT NULL
);

CREATE TABLE role_permissions (
  role_id       BIGINT NOT NULL REFERENCES roles(id),
  permission_id BIGINT NOT NULL REFERENCES permissions(id),
  PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_store_roles (
  id       BIGSERIAL PRIMARY KEY,
  user_id  BIGINT NOT NULL REFERENCES users(id),
  store_id BIGINT NOT NULL REFERENCES stores(id),
  role_id  BIGINT NOT NULL REFERENCES roles(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, store_id)
);

-- ========== membership_tiers ==========
CREATE TABLE membership_tiers (
  id              BIGSERIAL PRIMARY KEY,
  code            VARCHAR(16) NOT NULL UNIQUE,
  name            VARCHAR(32) NOT NULL,
  min_total_spend BIGINT  NOT NULL DEFAULT 0,
  discount_rate   SMALLINT NOT NULL DEFAULT 100,
  points_rate     NUMERIC(4,2) NOT NULL DEFAULT 1.0,
  sort            SMALLINT NOT NULL DEFAULT 0
);

-- ========== customers ==========
CREATE TABLE customers (
  id                BIGSERIAL PRIMARY KEY,
  name              VARCHAR(64) NOT NULL,
  phone             VARCHAR(20) NOT NULL UNIQUE,
  gender            SMALLINT NOT NULL DEFAULT 0,
  tier_id           BIGINT REFERENCES membership_tiers(id),
  wallet_balance    BIGINT NOT NULL DEFAULT 0,
  points_balance    BIGINT NOT NULL DEFAULT 0,
  total_spend       BIGINT NOT NULL DEFAULT 0,
  source            SMALLINT NOT NULL DEFAULT 1,
  wechat_openid     VARCHAR(64) UNIQUE,
  register_store_id BIGINT REFERENCES stores(id),
  last_visit_at     TIMESTAMPTZ,
  note              TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

-- ========== wallet_transactions ==========
CREATE TABLE wallet_transactions (
  id            BIGSERIAL PRIMARY KEY,
  customer_id   BIGINT NOT NULL REFERENCES customers(id),
  store_id      BIGINT NOT NULL REFERENCES stores(id),
  type          VARCHAR(16) NOT NULL CHECK (type IN ('recharge','consume','refund','adjust')),
  amount        BIGINT NOT NULL,
  balance_after BIGINT NOT NULL,
  ref_type      VARCHAR(32),
  ref_id        BIGINT,
  operator_id   BIGINT REFERENCES users(id),
  remark        VARCHAR(255),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_wallet_tx_customer ON wallet_transactions(customer_id, created_at DESC);

-- ========== points_transactions ==========
CREATE TABLE points_transactions (
  id            BIGSERIAL PRIMARY KEY,
  customer_id   BIGINT NOT NULL REFERENCES customers(id),
  store_id      BIGINT NOT NULL REFERENCES stores(id),
  type          VARCHAR(16) NOT NULL CHECK (type IN ('earn','redeem','adjust','expire')),
  amount        BIGINT NOT NULL,
  balance_after BIGINT NOT NULL,
  ref_type      VARCHAR(32),
  ref_id        BIGINT,
  operator_id   BIGINT REFERENCES users(id),
  remark        VARCHAR(255),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_points_tx_customer ON points_transactions(customer_id, created_at DESC);

-- ========== pets ==========
CREATE TABLE pets (
  id          BIGSERIAL PRIMARY KEY,
  customer_id BIGINT NOT NULL REFERENCES customers(id),
  name        VARCHAR(64) NOT NULL,
  species     SMALLINT NOT NULL DEFAULT 1,
  breed       VARCHAR(64),
  gender      SMALLINT NOT NULL DEFAULT 0,
  neutered    BOOLEAN NOT NULL DEFAULT false,
  birthday    DATE,
  weight_g    INT,
  color       VARCHAR(32),
  chip_no     VARCHAR(40),
  blood_type  VARCHAR(16),
  avatar_text VARCHAR(4),
  status      SMALLINT NOT NULL DEFAULT 1,
  note        TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_pets_customer ON pets(customer_id);
CREATE INDEX idx_pets_chip ON pets(chip_no);

-- ========== pet_health_records ==========
CREATE TABLE pet_health_records (
  id           BIGSERIAL PRIMARY KEY,
  pet_id       BIGINT NOT NULL REFERENCES pets(id),
  type         VARCHAR(16) NOT NULL CHECK (type IN ('vaccine','deworm','exam','allergy','other')),
  title        VARCHAR(128) NOT NULL,
  performed_at DATE,
  next_due_at  DATE,
  operator_id  BIGINT REFERENCES users(id),
  detail       TEXT,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_health_due ON pet_health_records(next_due_at) WHERE next_due_at IS NOT NULL;

-- ========== pet_weight_records ==========
CREATE TABLE pet_weight_records (
  id          BIGSERIAL PRIMARY KEY,
  pet_id      BIGINT NOT NULL REFERENCES pets(id),
  weight_g    INT NOT NULL,
  recorded_at DATE NOT NULL DEFAULT CURRENT_DATE
);

-- ========== service_categories ==========
CREATE TABLE service_categories (
  id    BIGSERIAL PRIMARY KEY,
  code  VARCHAR(16) NOT NULL UNIQUE,
  name  VARCHAR(32) NOT NULL,
  color VARCHAR(16),
  sort  SMALLINT NOT NULL DEFAULT 0
);

-- ========== services ==========
CREATE TABLE services (
  id                   BIGSERIAL PRIMARY KEY,
  category_id          BIGINT NOT NULL REFERENCES service_categories(id),
  name                 VARCHAR(64) NOT NULL,
  default_duration_min INT NOT NULL DEFAULT 60,
  default_price        BIGINT NOT NULL DEFAULT 0,
  requires_station     BOOLEAN NOT NULL DEFAULT true,
  status               SMALLINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

-- ========== service_offerings ==========
CREATE TABLE service_offerings (
  id              BIGSERIAL PRIMARY KEY,
  store_id        BIGINT NOT NULL REFERENCES stores(id),
  service_id      BIGINT NOT NULL REFERENCES services(id),
  price           BIGINT NOT NULL,
  duration_min    INT NOT NULL,
  bookable_online BOOLEAN NOT NULL DEFAULT false,
  status          SMALLINT NOT NULL DEFAULT 1,
  UNIQUE (store_id, service_id)
);

-- ========== stations ==========
CREATE TABLE stations (
  id            BIGSERIAL PRIMARY KEY,
  store_id      BIGINT NOT NULL REFERENCES stores(id),
  name          VARCHAR(64) NOT NULL,
  type          VARCHAR(16) NOT NULL DEFAULT 'general',
  staff_user_id BIGINT REFERENCES users(id),
  color         VARCHAR(16),
  status        SMALLINT NOT NULL DEFAULT 1,
  deleted_at    TIMESTAMPTZ
);
CREATE INDEX idx_stations_store ON stations(store_id);

-- ========== appointments ==========
CREATE TABLE appointments (
  id              BIGSERIAL PRIMARY KEY,
  store_id        BIGINT NOT NULL REFERENCES stores(id),
  customer_id     BIGINT REFERENCES customers(id),
  pet_id          BIGINT REFERENCES pets(id),
  source          SMALLINT NOT NULL DEFAULT 1,
  status          VARCHAR(16) NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending','arrived','in_progress','completed','cancelled','no_show')),
  scheduled_start TIMESTAMPTZ NOT NULL,
  scheduled_end   TIMESTAMPTZ NOT NULL,
  station_id      BIGINT REFERENCES stations(id),
  staff_user_id   BIGINT REFERENCES users(id),
  contact_name    VARCHAR(64),
  contact_phone   VARCHAR(20),
  total_amount    BIGINT NOT NULL DEFAULT 0,
  remark          VARCHAR(255),
  cancelled_reason VARCHAR(255),
  created_by      BIGINT REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_appt_store_time ON appointments(store_id, scheduled_start);
CREATE INDEX idx_appt_station_time ON appointments(station_id, scheduled_start, scheduled_end);

-- ========== appointment_items ==========
CREATE TABLE appointment_items (
  id                  BIGSERIAL PRIMARY KEY,
  appointment_id      BIGINT NOT NULL REFERENCES appointments(id),
  service_offering_id BIGINT NOT NULL REFERENCES service_offerings(id),
  service_name        VARCHAR(64) NOT NULL,
  price               BIGINT NOT NULL,
  duration_min        INT NOT NULL,
  station_id          BIGINT REFERENCES stations(id)
);

-- ========== room_types ==========
CREATE TABLE room_types (
  id              BIGSERIAL PRIMARY KEY,
  store_id        BIGINT NOT NULL REFERENCES stores(id),
  code            VARCHAR(16) NOT NULL,
  name            VARCHAR(32) NOT NULL,
  price_per_night BIGINT NOT NULL,
  capacity        INT NOT NULL DEFAULT 0,
  sort            SMALLINT NOT NULL DEFAULT 0,
  UNIQUE (store_id, code)
);

-- ========== boarding_rooms ==========
CREATE TABLE boarding_rooms (
  id           BIGSERIAL PRIMARY KEY,
  store_id     BIGINT NOT NULL REFERENCES stores(id),
  room_type_id BIGINT NOT NULL REFERENCES room_types(id),
  code         VARCHAR(16) NOT NULL,
  status       VARCHAR(16) NOT NULL DEFAULT 'free'
               CHECK (status IN ('free','occupied','cleaning','maintenance')),
  sort         SMALLINT NOT NULL DEFAULT 0,
  UNIQUE (store_id, code)
);

-- ========== boarding_orders ==========
CREATE TABLE boarding_orders (
  id                  BIGSERIAL PRIMARY KEY,
  store_id            BIGINT NOT NULL REFERENCES stores(id),
  customer_id         BIGINT NOT NULL REFERENCES customers(id),
  pet_id              BIGINT NOT NULL REFERENCES pets(id),
  room_id             BIGINT REFERENCES boarding_rooms(id),
  room_type_snapshot  VARCHAR(32) NOT NULL,
  price_per_night     BIGINT NOT NULL,
  status              VARCHAR(16) NOT NULL DEFAULT 'booked'
                      CHECK (status IN ('booked','checked_in','checked_out','cancelled')),
  source              SMALLINT NOT NULL DEFAULT 1,
  planned_check_in    TIMESTAMPTZ NOT NULL,
  planned_check_out   TIMESTAMPTZ NOT NULL,
  actual_check_in     TIMESTAMPTZ,
  actual_check_out    TIMESTAMPTZ,
  nights              INT,
  total_amount        BIGINT,
  settlement_id       BIGINT,
  remark              TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_boarding_store_status ON boarding_orders(store_id, status);

-- ========== boarding_care_logs ==========
CREATE TABLE boarding_care_logs (
  id                BIGSERIAL PRIMARY KEY,
  boarding_order_id BIGINT NOT NULL REFERENCES boarding_orders(id),
  store_id          BIGINT NOT NULL REFERENCES stores(id),
  task              VARCHAR(16) NOT NULL CHECK (task IN ('feeding','walking','medication','photo')),
  status            VARCHAR(8) NOT NULL DEFAULT 'pending' CHECK (status IN ('done','pending')),
  done_at           TIMESTAMPTZ,
  operator_id       BIGINT REFERENCES users(id),
  note              VARCHAR(255),
  photo_url         VARCHAR(255),
  log_date          DATE NOT NULL DEFAULT CURRENT_DATE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_care_order_date ON boarding_care_logs(boarding_order_id, log_date);

-- ========== product_categories ==========
CREATE TABLE product_categories (
  id       BIGSERIAL PRIMARY KEY,
  store_id BIGINT REFERENCES stores(id),
  name     VARCHAR(32) NOT NULL,
  sort     SMALLINT NOT NULL DEFAULT 0
);

-- ========== products ==========
CREATE TABLE products (
  id          BIGSERIAL PRIMARY KEY,
  name        VARCHAR(64) NOT NULL,
  category_id BIGINT REFERENCES product_categories(id),
  sku         VARCHAR(64) UNIQUE,
  unit        VARCHAR(8),
  spec        VARCHAR(64),
  price       BIGINT NOT NULL DEFAULT 0,
  cost        BIGINT,
  status      SMALLINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

-- ========== inventory ==========
CREATE TABLE inventory (
  id           BIGSERIAL PRIMARY KEY,
  store_id     BIGINT NOT NULL REFERENCES stores(id),
  product_id   BIGINT NOT NULL REFERENCES products(id),
  quantity     INT NOT NULL DEFAULT 0,
  safety_stock INT NOT NULL DEFAULT 0,
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (store_id, product_id)
);

-- ========== stock_transactions ==========
CREATE TABLE stock_transactions (
  id            BIGSERIAL PRIMARY KEY,
  store_id      BIGINT NOT NULL REFERENCES stores(id),
  product_id    BIGINT NOT NULL REFERENCES products(id),
  type          VARCHAR(16) NOT NULL
                CHECK (type IN ('purchase_in','sale_out','service_out','adjust','transfer')),
  quantity      INT NOT NULL,
  balance_after INT NOT NULL,
  ref_type      VARCHAR(32),
  ref_id        BIGINT,
  operator_id   BIGINT REFERENCES users(id),
  remark        VARCHAR(255),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_stock_tx ON stock_transactions(store_id, product_id, created_at DESC);

-- ========== purchase_orders ==========
CREATE TABLE purchase_orders (
  id          BIGSERIAL PRIMARY KEY,
  store_id    BIGINT NOT NULL REFERENCES stores(id),
  code        VARCHAR(32) NOT NULL UNIQUE,
  status      VARCHAR(16) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','received')),
  total_cost  BIGINT NOT NULL DEFAULT 0,
  operator_id BIGINT REFERENCES users(id),
  received_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ========== purchase_order_items ==========
CREATE TABLE purchase_order_items (
  id                BIGSERIAL PRIMARY KEY,
  purchase_order_id BIGINT NOT NULL REFERENCES purchase_orders(id),
  product_id        BIGINT NOT NULL REFERENCES products(id),
  quantity          INT NOT NULL,
  cost              BIGINT NOT NULL
);

-- ========== settlements ==========
CREATE TABLE settlements (
  id              BIGSERIAL PRIMARY KEY,
  store_id        BIGINT NOT NULL REFERENCES stores(id),
  code            VARCHAR(32) NOT NULL UNIQUE,
  customer_id     BIGINT REFERENCES customers(id),
  biz_type        VARCHAR(16) NOT NULL
                  CHECK (biz_type IN ('service','boarding','retail','recharge','mixed')),
  status          VARCHAR(16) NOT NULL DEFAULT 'unpaid'
                  CHECK (status IN ('unpaid','paid','refunded','void')),
  total_amount    BIGINT NOT NULL DEFAULT 0,
  discount_amount BIGINT NOT NULL DEFAULT 0,
  paid_amount     BIGINT NOT NULL DEFAULT 0,
  operator_id     BIGINT REFERENCES users(id),
  paid_at         TIMESTAMPTZ,
  remark          VARCHAR(255),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_settle_store_time ON settlements(store_id, created_at DESC);
CREATE INDEX idx_settle_paid ON settlements(store_id, paid_at) WHERE status='paid';

-- ========== settlement_items ==========
CREATE TABLE settlement_items (
  id           BIGSERIAL PRIMARY KEY,
  settlement_id BIGINT NOT NULL REFERENCES settlements(id),
  source_type  VARCHAR(16) NOT NULL CHECK (source_type IN ('appointment','boarding_order','product','recharge')),
  source_id    BIGINT,
  name         VARCHAR(128) NOT NULL,
  unit_price   BIGINT NOT NULL,
  quantity     INT NOT NULL DEFAULT 1,
  amount       BIGINT NOT NULL
);

-- ========== payments ==========
CREATE TABLE payments (
  id            BIGSERIAL PRIMARY KEY,
  settlement_id BIGINT NOT NULL REFERENCES settlements(id),
  method        VARCHAR(16) NOT NULL CHECK (method IN ('wechat','alipay','pos','cash','wallet')),
  amount        BIGINT NOT NULL,
  status        VARCHAR(16) NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending','success','failed','refunded')),
  trade_no      VARCHAR(64),
  paid_at       TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ========== notification_templates ==========
CREATE TABLE notification_templates (
  id      BIGSERIAL PRIMARY KEY,
  code    VARCHAR(32) NOT NULL,
  channel VARCHAR(16) NOT NULL CHECK (channel IN ('inapp','sms','wechat_mp')),
  title   VARCHAR(128),
  content TEXT NOT NULL,
  status  SMALLINT NOT NULL DEFAULT 1,
  UNIQUE (code, channel)
);

-- ========== notification_logs ==========
CREATE TABLE notification_logs (
  id            BIGSERIAL PRIMARY KEY,
  store_id      BIGINT REFERENCES stores(id),
  customer_id   BIGINT REFERENCES customers(id),
  template_code VARCHAR(32) NOT NULL,
  channel       VARCHAR(16) NOT NULL,
  payload       JSONB,
  status        VARCHAR(16) NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending','sent','failed','skipped')),
  error         VARCHAR(255),
  retry_count   SMALLINT NOT NULL DEFAULT 0,
  scheduled_at  TIMESTAMPTZ,
  sent_at       TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_notif_pending ON notification_logs(status, scheduled_at) WHERE status='pending';

-- ========== print_jobs ==========
CREATE TABLE print_jobs (
  id          BIGSERIAL PRIMARY KEY,
  store_id    BIGINT NOT NULL REFERENCES stores(id),
  type        VARCHAR(16) NOT NULL CHECK (type IN ('receipt','label')),
  ref_type    VARCHAR(32),
  ref_id      BIGINT,
  content     JSONB NOT NULL,
  status      VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','printed','failed')),
  printer_name VARCHAR(64),
  operator_id BIGINT REFERENCES users(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ========== audit_logs ==========
CREATE TABLE audit_logs (
  id          BIGSERIAL PRIMARY KEY,
  store_id    BIGINT REFERENCES stores(id),
  user_id     BIGINT REFERENCES users(id),
  action      VARCHAR(64) NOT NULL,
  target_type VARCHAR(32),
  target_id   BIGINT,
  detail      JSONB,
  ip          VARCHAR(45),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_store_time ON audit_logs(store_id, created_at DESC);

-- ========== system_settings ==========
CREATE TABLE system_settings (
  id         BIGSERIAL PRIMARY KEY,
  store_id   BIGINT REFERENCES stores(id),
  key        VARCHAR(64) NOT NULL,
  value      JSONB NOT NULL,
  updated_by BIGINT REFERENCES users(id),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (store_id, key)
);

-- ========== updated_at triggers ==========
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['stores','users','customers','pets','services','appointments',
    'boarding_orders','products','settlements'] LOOP
    EXECUTE format('CREATE TRIGGER trg_%s_updated BEFORE UPDATE ON %I FOR EACH ROW EXECUTE FUNCTION set_updated_at();', t, t);
  END LOOP;
END $$;
```

- [ ] **Step 2: Write down migration**

Write `backend/migrations/000001_init_schema.down.sql`:
```sql
DROP TABLE IF EXISTS system_settings CASCADE;
DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS print_jobs CASCADE;
DROP TABLE IF EXISTS notification_logs CASCADE;
DROP TABLE IF EXISTS notification_templates CASCADE;
DROP TABLE IF EXISTS payments CASCADE;
DROP TABLE IF EXISTS settlement_items CASCADE;
DROP TABLE IF EXISTS settlements CASCADE;
DROP TABLE IF EXISTS purchase_order_items CASCADE;
DROP TABLE IF EXISTS purchase_orders CASCADE;
DROP TABLE IF EXISTS stock_transactions CASCADE;
DROP TABLE IF EXISTS inventory CASCADE;
DROP TABLE IF EXISTS products CASCADE;
DROP TABLE IF EXISTS product_categories CASCADE;
DROP TABLE IF EXISTS boarding_care_logs CASCADE;
DROP TABLE IF EXISTS boarding_orders CASCADE;
DROP TABLE IF EXISTS boarding_rooms CASCADE;
DROP TABLE IF EXISTS room_types CASCADE;
DROP TABLE IF EXISTS appointment_items CASCADE;
DROP TABLE IF EXISTS appointments CASCADE;
DROP TABLE IF EXISTS stations CASCADE;
DROP TABLE IF EXISTS service_offerings CASCADE;
DROP TABLE IF EXISTS services CASCADE;
DROP TABLE IF EXISTS service_categories CASCADE;
DROP TABLE IF EXISTS pet_weight_records CASCADE;
DROP TABLE IF EXISTS pet_health_records CASCADE;
DROP TABLE IF EXISTS pets CASCADE;
DROP TABLE IF EXISTS points_transactions CASCADE;
DROP TABLE IF EXISTS wallet_transactions CASCADE;
DROP TABLE IF EXISTS customers CASCADE;
DROP TABLE IF EXISTS membership_tiers CASCADE;
DROP TABLE IF EXISTS user_store_roles CASCADE;
DROP TABLE IF EXISTS role_permissions CASCADE;
DROP TABLE IF EXISTS permissions CASCADE;
DROP TABLE IF EXISTS roles CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS stores CASCADE;
DROP FUNCTION IF EXISTS set_updated_at CASCADE;
```

- [ ] **Step 3: Write seed data migration**

Write `backend/migrations/000002_seed_data.up.sql` by copying `files/seed.sql` with the bcrypt hash for "pawprint123".

Note: the bcrypt hash `$2b$10$hI3knf0o5Xt21pyemrPVbOQZVRXpnzW2JHcpnn3eA76fbZq5h066q` from seed.sql must be preserved exactly. Copy the full content of `files/seed.sql` to this migration file.

- [ ] **Step 4: Write seed down migration**

Write `backend/migrations/000002_seed_data.down.sql`:
```sql
DELETE FROM system_settings;
DELETE FROM notification_templates;
DELETE FROM payments;
DELETE FROM settlement_items;
DELETE FROM settlements;
DELETE FROM appointment_items;
DELETE FROM appointments;
DELETE FROM boarding_care_logs;
DELETE FROM boarding_orders;
DELETE FROM boarding_rooms;
DELETE FROM room_types;
DELETE FROM service_offerings;
DELETE FROM services;
DELETE FROM service_categories;
DELETE FROM pet_weight_records;
DELETE FROM pet_health_records;
DELETE FROM pets;
DELETE FROM points_transactions;
DELETE FROM wallet_transactions;
DELETE FROM customers;
DELETE FROM membership_tiers;
DELETE FROM stock_transactions;
DELETE FROM inventory;
DELETE FROM purchase_order_items;
DELETE FROM purchase_orders;
DELETE FROM products;
DELETE FROM product_categories;
DELETE FROM stations;
DELETE FROM user_store_roles;
DELETE FROM role_permissions;
DELETE FROM permissions;
DELETE FROM roles;
DELETE FROM users;
DELETE FROM stores;
```

- [ ] **Step 5: Commit**

```bash
git add backend/migrations/
git commit -m "feat(phase1): add database migrations — schema and seed data

- 000001_init_schema.up.sql: 29 tables with constraints, indexes, triggers
- 000001_init_schema.down.sql: cascade drop all tables
- 000002_seed_data.up.sql: demo store, 8 users, 5 roles, 26 permissions,
  4 tiers, 8 customers, 8 pets, 6 services, 4 stations, 24 rooms,
  5 boarding orders, 4 products, 3 settlements, notification templates,
  12 system settings
- 000002_seed_data.down.sql: truncate all seeded data

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 8: Main entry point, health checks, and middleware wire-up

**Files:**
- Create: `backend/cmd/server/main.go`
- Create: `backend/internal/middleware/traceid.go`
- Create: `backend/internal/middleware/logger.go`
- Create: `backend/internal/middleware/recovery.go`
- Create: `backend/internal/middleware/cors.go`
- Create: `backend/internal/router/router.go`

- [ ] **Step 1: Write trace ID middleware**

Write `backend/internal/middleware/traceid.go`:
```go
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TraceID injects or propagates a trace ID for request correlation.
func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = uuid.New().String()
		}
		c.Set("trace_id", traceID)
		c.Header("X-Trace-ID", traceID)
		c.Next()
	}
}
```

- [ ] **Step 2: Write logger middleware**

Write `backend/internal/middleware/logger.go`:
```go
package middleware

import (
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

var Logger *slog.Logger

func init() {
	Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// RequestLogger logs each request with trace ID, method, path, status, and latency.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		traceID, _ := c.Get("trace_id")
		Logger.Info("request",
			"trace_id", traceID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", latency.Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}
```

- [ ] **Step 3: Write recovery middleware**

Write `backend/internal/middleware/recovery.go`:
```go
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/response"
)

// Recovery handles panics and returns a 500 error.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				traceID, _ := c.Get("trace_id")
				Logger.Error("panic recovered",
					"trace_id", traceID,
					"panic", r,
				)
				response.Error(c, apperr.New(5000, "服务器内部错误"))
				c.Abort()
				return
			}
		}()
		c.Next()
	}
}

// ErrorHandler is a gin middleware that catches apperr.AppError from handlers.
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			if ae, ok := err.(*apperr.AppError); ok {
				response.Error(c, ae)
				return
			}
			response.Error(c, apperr.Internal(err))
		}
	}
}
```

- [ ] **Step 4: Write CORS middleware**

Write `backend/internal/middleware/cors.go`:
```go
package middleware

import (
	"github.com/gin-gonic/gin"
)

// CORS allows cross-origin requests from the admin frontend and mini-program.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin,Content-Type,Accept,Authorization,X-Store-Id,X-Trace-ID,Idempotency-Key")
		c.Header("Access-Control-Expose-Headers", "X-Trace-ID")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
```

- [ ] **Step 5: Write router setup**

Write `backend/internal/router/router.go`:
```go
package router

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"pawprint/backend/internal/middleware"
)

func Setup(db *gorm.DB) *gin.Engine {
	r := gin.New()

	// Global middleware (order matters)
	r.Use(middleware.TraceID())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.Recovery())
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.CORS())

	// Health checks (no auth required)
	r.GET("/healthz", healthCheck(db))
	r.GET("/readyz", readyCheck(db))

	// API v1 group (auth middleware added in Phase 2)
	v1 := r.Group("/api/v1")
	_ = v1 // placeholder — auth routes registered in Phase 2

	return r
}

func healthCheck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(503, gin.H{"status": "unhealthy", "error": "db connection lost"})
			return
		}
		if err := sqlDB.Ping(); err != nil {
			c.JSON(503, gin.H{"status": "unhealthy", "error": "db ping failed"})
			return
		}
		c.JSON(200, gin.H{"status": "healthy"})
	}
}

func readyCheck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(503, gin.H{"status": "not ready"})
			return
		}
		if err := sqlDB.Ping(); err != nil {
			c.JSON(503, gin.H{"status": "not ready", "error": "db unreachable"})
			return
		}
		// Check if migrations have been applied by looking for the stores table
		var count int64
		if err := db.Raw("SELECT count(*) FROM information_schema.tables WHERE table_name = 'stores'").Scan(&count).Error; err != nil || count == 0 {
			c.JSON(503, gin.H{"status": "not ready", "error": "migrations not applied"})
			return
		}
		c.JSON(200, gin.H{"status": "ready"})
	}
}
```

- [ ] **Step 6: Write main.go**

Write `backend/cmd/server/main.go`:
```go
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"pawprint/backend/internal/config"
	"pawprint/backend/internal/router"
)

func main() {
	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Connect to PostgreSQL
	db, err := gorm.Open(postgres.Open(cfg.DB.DSN), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to get underlying DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)

	// Setup router
	r := router.Setup(db)

	// Graceful shutdown
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("shutting down server...")
		sqlDB.Close()
		os.Exit(0)
	}()

	addr := fmt.Sprintf(":%s", cfg.HTTPPort)
	log.Printf("PawPrint server starting on %s (env=%s)", addr, cfg.AppEnv)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
```

- [ ] **Step 7: Add uuid dependency and verify build**

```bash
cd backend && go get github.com/google/uuid && go build ./cmd/server/...
```

Expected: build succeeds.

- [ ] **Step 8: Commit**

```bash
git add backend/cmd/ backend/internal/middleware/traceid.go backend/internal/middleware/logger.go backend/internal/middleware/recovery.go backend/internal/middleware/cors.go backend/internal/router/
git commit -m "feat(phase1): add main entry point, health checks, and middleware wire-up

- cmd/server/main.go: config loading, DB connection, graceful shutdown
- Middleware: TraceID (uuid propagation), RequestLogger (structured JSON),
  Recovery (panic → 500), ErrorHandler (AppError → response), CORS
- Router: /healthz (db ping), /readyz (migrations check)
- Global middleware chain: TraceID → Logger → Recovery → ErrorHandler → CORS

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 9: Docker Compose and Makefile

**Files:**
- Create: `backend/Dockerfile`
- Create: `backend/docker-compose.yml`
- Create: `backend/Makefile`

- [ ] **Step 1: Write Dockerfile**

Write `backend/Dockerfile`:
```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/server ./cmd/server/

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/server .
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/config ./config
EXPOSE 8080
CMD ["./server"]
```

- [ ] **Step 2: Write docker-compose.yml**

Write `backend/docker-compose.yml`:
```yaml
version: "3.9"
services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: pawprint
      POSTGRES_PASSWORD: pawprint
      POSTGRES_DB: pawprint
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U pawprint"]
      interval: 5s
      timeout: 3s
      retries: 5

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

  api:
    build: .
    ports:
      - "8080:8080"
    environment:
      APP_ENV: dev
      HTTP_PORT: "8080"
      DB_DSN: postgres://pawprint:pawprint@postgres:5432/pawprint?sslmode=disable
      REDIS_ADDR: redis:6379
      JWT_ACCESS_SECRET: dev-access-secret-change-in-production
      JWT_REFRESH_SECRET: dev-refresh-secret-change-in-production
      DEFAULT_TIMEZONE: Asia/Shanghai
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy

volumes:
  pgdata:
```

- [ ] **Step 3: Write Makefile**

Write `backend/Makefile`:
```makefile
.PHONY: test test-cover lint build dev db-up db-down migrate-up migrate-down seed clean

# Run all tests
test:
	go test ./... -v -count=1

# Run tests with coverage
test-cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out

# Lint
lint:
	go vet ./...

# Build binary
build:
	go build -o bin/server ./cmd/server/

# Start development database and redis
db-up:
	docker compose up -d postgres redis

# Stop all services
db-down:
	docker compose down

# Run database migrations
migrate-up:
	go run ./cmd/migrate/main.go up

migrate-down:
	go run ./cmd/migrate/main.go down

# Seed demo data
seed:
	go run ./cmd/seed/main.go

# Run development server
dev:
	go run ./cmd/server/main.go

# Clean build artifacts
clean:
	rm -rf bin/ coverage.out
```

- [ ] **Step 4: Commit**

```bash
git add backend/Dockerfile backend/docker-compose.yml backend/Makefile
git commit -m "feat(phase1): add Dockerfile, docker-compose, and Makefile

- Multi-stage Docker build for small production image
- docker-compose: postgres 15 + redis 7 + api with health checks
- Makefile: test, lint, build, db-up/down, migrate, seed, dev targets

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Phase 1 Complete — Verification Gate

Before moving to Phase 2, verify:

```bash
cd backend
go test ./... -v                    # All package tests pass
go build ./cmd/server/...           # Binary compiles
docker compose up -d postgres redis # Infrastructure starts
# Verify: curl http://localhost:8080/healthz → {"status":"healthy"}
```

---

## Phase 2: Authentication & Security Core

### Task 10: Auth module — models and DTOs

**Files:**
- Create: `backend/internal/module/auth/model.go`
- Create: `backend/internal/module/auth/dto.go`

- [ ] **Step 1: Write auth models**

Write `backend/internal/module/auth/model.go`:
```go
package auth

import "time"

// User mirrors the users table.
type User struct {
	ID           int64      `gorm:"primaryKey" json:"id"`
	Username     string     `gorm:"uniqueIndex;size:64" json:"username"`
	PasswordHash string     `gorm:"size:255" json:"-"`
	DisplayName  string     `gorm:"size:64" json:"display_name"`
	Phone        string     `gorm:"uniqueIndex;size:20" json:"phone"`
	AvatarText   string     `gorm:"size:4" json:"avatar_text"`
	Status       int16      `json:"status"`
	LastStoreID  *int64     `json:"last_store_id"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `gorm:"index" json:"-"`
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
```

- [ ] **Step 2: Write auth DTOs**

Write `backend/internal/module/auth/dto.go`:
```go
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
```

- [ ] **Step 3: Commit**

```bash
git add backend/internal/module/auth/model.go backend/internal/module/auth/dto.go
git commit -m "feat(phase2): add auth module models and DTOs

- Models: User, Role, UserStoreRole, Permission, Store (GORM)
- DTOs: LoginRequest/Response, RefreshRequest, SwitchStoreRequest/Response
- Matches users, roles, user_store_roles, permissions, stores tables

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 11: Auth repo layer

**Files:**
- Create: `backend/internal/module/auth/repo.go`

- [ ] **Step 1: Write the auth repo**

Write `backend/internal/module/auth/repo.go`:
```go
package auth

import (
	"gorm.io/gorm"
)

// Repository defines the data access interface for auth.
type Repository interface {
	FindUserByUsername(username string) (*User, error)
	FindUserByID(id int64) (*User, error)
	FindUserStores(userID int64) ([]StoreInfo, error)
	FindUserPermissions(userID int64, storeID int64) ([]string, error)
	FindUserStoreRole(userID, storeID int64) (*UserStoreRole, error)
	UpdateLastStore(userID, storeID int64) error
}

type repo struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repo{db: db}
}

func (r *repo) FindUserByUsername(username string) (*User, error) {
	var u User
	err := r.db.Where("username = ? AND status = 1", username).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *repo) FindUserByID(id int64) (*User, error) {
	var u User
	err := r.db.Where("id = ? AND status = 1", id).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *repo) FindUserStores(userID int64) ([]StoreInfo, error) {
	var stores []StoreInfo
	err := r.db.Table("user_store_roles usr").
		Select("s.id, s.name, r.code as role").
		Joins("JOIN stores s ON s.id = usr.store_id AND s.deleted_at IS NULL AND s.status = 1").
		Joins("JOIN roles r ON r.id = usr.role_id").
		Where("usr.user_id = ?", userID).
		Scan(&stores).Error
	return stores, err
}

func (r *repo) FindUserPermissions(userID int64, storeID int64) ([]string, error) {
	var perms []string
	err := r.db.Table("user_store_roles usr").
		Select("DISTINCT p.code").
		Joins("JOIN role_permissions rp ON rp.role_id = usr.role_id").
		Joins("JOIN permissions p ON p.id = rp.permission_id").
		Where("usr.user_id = ? AND usr.store_id = ?", userID, storeID).
		Pluck("p.code", &perms).Error
	return perms, err
}

func (r *repo) FindUserStoreRole(userID, storeID int64) (*UserStoreRole, error) {
	var usr UserStoreRole
	err := r.db.Preload("Role").
		Where("user_id = ? AND store_id = ?", userID, storeID).
		First(&usr).Error
	if err != nil {
		return nil, err
	}
	return &usr, nil
}

func (r *repo) UpdateLastStore(userID, storeID int64) error {
	return r.db.Model(&User{}).Where("id = ?", userID).
		Update("last_store_id", storeID).Error
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/internal/module/auth/repo.go
git commit -m "feat(phase2): add auth repository layer

- FindUserByUsername, FindUserByID: user lookup with status=1 filter
- FindUserStores: returns stores user has access to with role code
- FindUserPermissions: returns permission codes for user in a store
- FindUserStoreRole: checks authorization for specific store
- UpdateLastStore: tracks last active store per user

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 12: Auth service — login, refresh, switch-store (TDD)

**Files:**
- Create: `backend/internal/module/auth/service.go`
- Create: `backend/internal/module/auth/service_test.go`

- [ ] **Step 1: Write the service test (RED)**

Write `backend/internal/module/auth/service_test.go`:
```go
package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// mockRepo implements Repository for testing
type mockRepo struct {
	users        map[string]*User
	stores       map[int64][]StoreInfo
	permissions  map[int64][]string
	storeRoles   map[int64]*UserStoreRole
	lastStoreErr error
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		users:       make(map[string]*User),
		stores:      make(map[int64][]StoreInfo),
		permissions: make(map[int64][]string),
		storeRoles:  make(map[int64]*UserStoreRole),
	}
}

func (m *mockRepo) FindUserByUsername(username string) (*User, error) {
	u, ok := m.users[username]
	if !ok {
		return nil, gormErrRecordNotFound()
	}
	return u, nil
}

func (m *mockRepo) FindUserByID(id int64) (*User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, gormErrRecordNotFound()
}

func (m *mockRepo) FindUserStores(userID int64) ([]StoreInfo, error) {
	return m.stores[userID], nil
}

func (m *mockRepo) FindUserPermissions(userID, storeID int64) ([]string, error) {
	return m.permissions[userID], nil
}

func (m *mockRepo) FindUserStoreRole(userID, storeID int64) (*UserStoreRole, error) {
	usr, ok := m.storeRoles[storeID]
	if !ok {
		return nil, gormErrRecordNotFound()
	}
	return usr, nil
}

func (m *mockRepo) UpdateLastStore(userID, storeID int64) error {
	return m.lastStoreErr
}

// gormErrRecordNotFound returns an error compatible with errors.Is(err, gorm.ErrRecordNotFound)
type notFoundError struct{}

func (e *notFoundError) Error() string { return "record not found" }

func gormErrRecordNotFound() error { return &notFoundError{} }

func mustHashPassword(pw string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}
	return string(hash)
}

func TestLoginSuccess(t *testing.T) {
	repo := newMockRepo()
	repo.users["admin"] = &User{
		ID:           1,
		Username:     "admin",
		PasswordHash: mustHashPassword("pawprint123"),
		DisplayName:  "管理员",
		Status:       1,
	}
	repo.stores[1] = []StoreInfo{
		{ID: 1, Name: "旗舰店", Role: "super_admin"},
	}

	svc := NewService(repo, "access-secret-32-chars-minimum!!", "refresh-secret-32-chars-minimum!!")

	resp, err := svc.Login("admin", "pawprint123")
	if err != nil {
		t.Fatalf("Login() error: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("access token should not be empty")
	}
	if resp.RefreshToken == "" {
		t.Error("refresh token should not be empty")
	}
	if len(resp.Stores) != 1 {
		t.Errorf("stores count = %d, want 1", len(resp.Stores))
	}
}

func TestLoginBadPassword(t *testing.T) {
	repo := newMockRepo()
	repo.users["admin"] = &User{
		ID:           1,
		Username:     "admin",
		PasswordHash: mustHashPassword("correct"),
		Status:       1,
	}

	svc := NewService(repo, "access-secret-32-chars-minimum!!", "refresh-secret-32-chars-minimum!!")

	_, err := svc.Login("admin", "wrong")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	ae := err.(*appErr)
	if ae.Code != 1001 {
		t.Errorf("error code = %d, want 1001", ae.Code)
	}
}

func TestLoginUserNotFound(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo, "access-secret-32-chars-minimum!!", "refresh-secret-32-chars-minimum!!")

	_, err := svc.Login("nonexistent", "pw")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}

func TestRefreshToken(t *testing.T) {
	repo := newMockRepo()
	repo.users["admin"] = &User{ID: 1, Username: "admin", Status: 1}
	repo.stores[1] = []StoreInfo{{ID: 1, Name: "旗舰店", Role: "super_admin"}}

	svc := NewService(repo, "access-secret-32-chars-minimum!!", "refresh-secret-32-chars-minimum!!")

	// First login to get a refresh token
	repo.users["admin"].PasswordHash = mustHashPassword("pw")
	loginResp, err := svc.Login("admin", "pw")
	if err != nil {
		t.Fatalf("Login() error: %v", err)
	}

	// Then refresh
	resp, err := svc.RefreshToken(loginResp.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshToken() error: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("new access token should not be empty")
	}
}

func TestSwitchStoreUnauthorized(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo, "access-secret-32-chars-minimum!!", "refresh-secret-32-chars-minimum!!")

	_, err := svc.SwitchStore(1, 999) // user 1 has no access to store 999
	if err == nil {
		t.Fatal("expected error for unauthorized store")
	}
}

// appErr interface for testing
type appErr interface {
	error
	CodeValue() int
}
```

Note: the test file uses a custom `appErr` interface. We'll update the `apperr.AppError` to match.

- [ ] **Step 2: Run test (fail)**

```bash
cd backend && go test ./internal/module/auth/... -v
```

Expected: FAIL — Service type not defined.

- [ ] **Step 3: Write the service implementation (GREEN)**

Write `backend/internal/module/auth/service.go`:
```go
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/errcode"
)

// Service handles authentication business logic.
type Service struct {
	repo          Repository
	accessSecret  string
	refreshSecret string
}

func NewService(repo Repository, accessSecret, refreshSecret string) *Service {
	return &Service{
		repo:          repo,
		accessSecret:  accessSecret,
		refreshSecret: refreshSecret,
	}
}

// Claims for access tokens.
type AccessClaims struct {
	jwt.RegisteredClaims
	UserID  int64  `json:"uid"`
	StoreID int64  `json:"store_id"`
	Role    string `json:"role"`
}

// Claims for refresh tokens (lighter, only user ID).
type RefreshClaims struct {
	jwt.RegisteredClaims
	UserID int64 `json:"uid"`
}

// Login authenticates a user and returns tokens with store access list.
func (s *Service) Login(username, password string) (*LoginResponse, error) {
	user, err := s.repo.FindUserByUsername(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.Unauthorized("用户名或密码错误")
		}
		return nil, apperr.Internal(err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, apperr.Unauthorized("用户名或密码错误")
	}

	stores, err := s.repo.FindUserStores(user.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if len(stores) == 0 {
		return nil, apperr.Forbidden("未分配到任何门店")
	}

	// Issue access token for the first store (or last used)
	firstStoreID := stores[0].ID
	firstRole := stores[0].Role

	access, err := s.issueAccess(user.ID, firstStoreID, firstRole)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	refresh, err := s.issueRefresh(user.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	return &LoginResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		Stores:       stores,
	}, nil
}

// RefreshToken validates a refresh token and issues new tokens.
func (s *Service) RefreshToken(refreshToken string) (*LoginResponse, error) {
	claims := &RefreshClaims{}
	token, err := jwt.ParseWithClaims(refreshToken, claims,
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(s.refreshSecret), nil
		})
	if err != nil || !token.Valid {
		return nil, apperr.Unauthorized("refresh token 无效或已过期")
	}

	user, err := s.repo.FindUserByID(claims.UserID)
	if err != nil {
		return nil, apperr.Unauthorized("用户已禁用或不存在")
	}

	stores, err := s.repo.FindUserStores(user.ID)
	if err != nil || len(stores) == 0 {
		return nil, apperr.Internal(err)
	}

	access, _ := s.issueAccess(user.ID, stores[0].ID, stores[0].Role)
	refresh, _ := s.issueRefresh(user.ID)

	return &LoginResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		Stores:       stores,
	}, nil
}

// SwitchStore validates the user has access to the target store and issues new scoped tokens.
func (s *Service) SwitchStore(userID, storeID int64) (*SwitchStoreResponse, error) {
	usr, err := s.repo.FindUserStoreRole(userID, storeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.New(errcode.StoreForbidden, "无该门店访问权限")
		}
		return nil, apperr.Internal(err)
	}

	access, err := s.issueAccess(userID, storeID, usr.Role.Code)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	refresh, err := s.issueRefresh(userID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	_ = s.repo.UpdateLastStore(userID, storeID)

	return &SwitchStoreResponse{
		AccessToken:  access,
		RefreshToken: refresh,
	}, nil
}

// ParseAccessToken validates and returns access token claims.
func (s *Service) ParseAccessToken(tokenStr string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims,
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(s.accessSecret), nil
		})
	if err != nil || !token.Valid {
		return nil, apperr.Unauthorized("access token 无效或已过期")
	}
	return claims, nil
}

func (s *Service) issueAccess(userID, storeID int64, role string) (string, error) {
	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID:  userID,
		StoreID: storeID,
		Role:    role,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.accessSecret))
}

func (s *Service) issueRefresh(userID int64) (string, error) {
	claims := RefreshClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(720 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID: userID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.refreshSecret))
}
```

- [ ] **Step 4: Fix test imports and run**

The service test references `appErr` interface. Update `service_test.go` to use the actual `apperr.AppError`:

Edit the test file to use `"pawprint/backend/internal/pkg/apperr"` and check `ae.Code` directly instead of using a custom interface.

```bash
cd backend && go test ./internal/module/auth/... -v
```

Fix any compilation errors, then verify: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/module/auth/service.go backend/internal/module/auth/service_test.go
git commit -m "feat(phase2): add auth service with TDD — login, refresh, switch-store

- Login: bcrypt password verification, JWT issue (access 2h + refresh 720h)
- RefreshToken: validate refresh token, reissue both tokens
- SwitchStore: validate store authorization, issue scoped tokens
- Test coverage: valid login, wrong password, nonexistent user,
  token refresh, unauthorized store switch

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 13: Auth handler + router registration

**Files:**
- Create: `backend/internal/module/auth/handler.go`
- Create: `backend/internal/module/auth/handler_test.go`
- Create: `backend/internal/module/auth/router.go`
- Modify: `backend/internal/router/router.go`

- [ ] **Step 1: Write auth handler**

Write `backend/internal/module/auth/handler.go`:
```go
package auth

import (
	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/response"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("请输入用户名和密码"))
		return
	}

	resp, err := h.svc.Login(req.Username, req.Password)
	if err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("请提供 refresh_token"))
		return
	}

	resp, err := h.svc.RefreshToken(req.RefreshToken)
	if err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, resp)
}

func (h *Handler) SwitchStore(c *gin.Context) {
	var req SwitchStoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.BadRequest("请提供 store_id"))
		return
	}

	// UserID comes from auth middleware context
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, apperr.Unauthorized())
		return
	}

	resp, err := h.svc.SwitchStore(userID.(int64), req.StoreID)
	if err != nil {
		response.Error(c, err.(*apperr.AppError))
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Logout(c *gin.Context) {
	// Token blacklisting via Redis is added in Task 15 (ratelimit/idempotency).
	// For now, client discards tokens.
	response.Success(c, nil)
}
```

- [ ] **Step 2: Write auth handler test**

Write `backend/internal/module/auth/handler_test.go`:
```go
package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/pkg/errcode"
)

func init() { gin.SetMode(gin.TestMode) }

func setupTestRouter(svc *Service) *gin.Engine {
	r := gin.New()
	h := NewHandler(svc)
	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/login", h.Login)
		auth.POST("/refresh", h.Refresh)
	}
	return r
}

func TestLoginHandlerSuccess(t *testing.T) {
	repo := newMockRepo()
	repo.users["admin"] = &User{
		ID: 1, Username: "admin",
		PasswordHash: mustHashPassword("pawprint123"),
		Status:       1,
	}
	repo.stores[1] = []StoreInfo{{ID: 1, Name: "旗舰店", Role: "super_admin"}}

	svc := NewService(repo, "access-secret-32-chars-minimum!!", "refresh-secret-32-chars-minimum!!")
	router := setupTestRouter(svc)

	body, _ := json.Marshal(LoginRequest{Username: "admin", Password: "pawprint123"})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp struct {
		Code    int           `json:"code"`
		Message string        `json:"message"`
		Data    LoginResponse `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("code = %d", resp.Code)
	}
	if resp.Data.AccessToken == "" {
		t.Error("access token is empty")
	}
}

func TestLoginHandlerBadPassword(t *testing.T) {
	repo := newMockRepo()
	repo.users["admin"] = &User{
		ID: 1, Username: "admin",
		PasswordHash: mustHashPassword("correct"),
		Status:       1,
	}

	svc := NewService(repo, "access-secret-32-chars-minimum!!", "refresh-secret-32-chars-minimum!!")
	router := setupTestRouter(svc)

	body, _ := json.Marshal(LoginRequest{Username: "admin", Password: "wrong"})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}

	var resp struct {
		Code int `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != errcode.Unauthenticated {
		t.Errorf("code = %d, want %d", resp.Code, errcode.Unauthenticated)
	}
}

func TestLoginHandlerEmptyBody(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo, "access-secret-32-chars-minimum!!", "refresh-secret-32-chars-minimum!!")
	router := setupTestRouter(svc)

	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
```

- [ ] **Step 3: Write auth router**

Write `backend/internal/module/auth/router.go`:
```go
package auth

import "github.com/gin-gonic/gin"

// RegisterRoutes registers auth endpoints under /api/v1/auth.
func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	auth := r.Group("/auth")
	{
		auth.POST("/login", h.Login)
		auth.POST("/refresh", h.Refresh)
		auth.POST("/switch-store", h.SwitchStore)
		auth.POST("/logout", h.Logout)
	}
}
```

- [ ] **Step 4: Wire auth into router**

Update `backend/internal/router/router.go` — replace the `_ = v1` placeholder:

```go
import (
	"pawprint/backend/internal/module/auth"
)

// In Setup(), after v1 := r.Group("/api/v1"):
authRepo := auth.NewRepository(db)
authSvc := auth.NewService(authRepo, cfg.JWT.AccessSecret, cfg.JWT.RefreshSecret)
authHandler := auth.NewHandler(authSvc)
auth.RegisterRoutes(v1, authHandler)
```

Note: `Setup` now needs the `*config.Config`. Update its signature:

```go
func Setup(db *gorm.DB, cfg *config.Config) *gin.Engine {
```

And update `main.go` to pass `cfg`:

```go
r := router.Setup(db, cfg)
```

- [ ] **Step 5: Run handler tests**

```bash
cd backend && go test ./internal/module/auth/... -v && go build ./cmd/server/...
```

Expected: all tests PASS, build succeeds.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/module/auth/handler.go backend/internal/module/auth/handler_test.go backend/internal/module/auth/router.go backend/internal/router/router.go backend/cmd/server/main.go
git commit -m "feat(phase2): add auth HTTP handler and router registration

- POST /api/v1/auth/login: username+password → tokens + stores
- POST /api/v1/auth/refresh: refresh_token → new tokens
- POST /api/v1/auth/switch-store: store_id → scoped tokens
- POST /api/v1/auth/logout: placeholder (token blacklist in later task)
- Handler tests: success, bad password, empty body
- Router wired with Repository → Service → Handler dependency chain

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 14: Auth middleware — JWT parsing and context injection

**Files:**
- Create: `backend/internal/middleware/auth.go`

- [ ] **Step 1: Write auth middleware**

Write `backend/internal/middleware/auth.go`:
```go
package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/module/auth"
	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/response"
)

// AuthRequired validates the JWT access token and injects claims into context.
func AuthRequired(authSvc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			response.Error(c, apperr.Unauthorized("缺少认证令牌"))
			c.Abort()
			return
		}

		claims, err := authSvc.ParseAccessToken(token)
		if err != nil {
			if ae, ok := err.(*apperr.AppError); ok {
				response.Error(c, ae)
			} else {
				response.Error(c, apperr.Unauthorized())
			}
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("store_id", claims.StoreID)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// extractToken extracts the Bearer token from the Authorization header.
func extractToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/internal/middleware/auth.go
git commit -m "feat(phase2): add JWT auth middleware

- AuthRequired: extracts Bearer token, validates via auth.Service.ParseAccessToken
- Injects user_id, store_id, role into Gin context
- Returns 401 for missing/invalid/expired tokens

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 15: RBAC + StoreScope middleware

**Files:**
- Create: `backend/internal/middleware/rbac.go`
- Create: `backend/internal/middleware/storescope.go`

- [ ] **Step 1: Write RBAC middleware**

Write `backend/internal/middleware/rbac.go`:
```go
package middleware

import (
	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/module/auth"
	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/response"
)

// RequirePermission checks that the authenticated user has the required permission.
// Usage: r.POST("/appointments", RequirePermission(authSvc, "appointment:create"), handler.Create)
func RequirePermission(authSvc *auth.Service, permissionCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// super_admin bypasses permission checks
		role, _ := c.Get("role")
		if role == "super_admin" {
			c.Next()
			return
		}

		userID, exists := c.Get("user_id")
		if !exists {
			response.Error(c, apperr.Unauthorized())
			c.Abort()
			return
		}

		storeID, _ := c.Get("store_id")

		// Load permissions from repo
		perms, err := authSvc.GetPermissions(userID.(int64), storeID.(int64))
		if err != nil {
			response.Error(c, apperr.Internal(err))
			c.Abort()
			return
		}

		for _, p := range perms {
			if p == permissionCode {
				c.Next()
				return
			}
		}

		response.Error(c, apperr.Forbidden("无此操作权限: "+permissionCode))
		c.Abort()
	}
}
```

Add the `GetPermissions` method to the auth service:

In `backend/internal/module/auth/service.go`, add:
```go
// GetPermissions returns the permission codes for a user in a store.
func (s *Service) GetPermissions(userID, storeID int64) ([]string, error) {
	return s.repo.FindUserPermissions(userID, storeID)
}
```

- [ ] **Step 2: Write store-scope middleware**

Write `backend/internal/middleware/storescope.go`:
```go
package middleware

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/module/auth"
	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/errcode"
	"pawprint/backend/internal/pkg/response"
)

// StoreScope enforces multi-store isolation.
// Extracts X-Store-Id header and validates user has access to that store.
// super_admin can pass X-Store-Id: * for cross-store queries.
func StoreScope(authSvc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		userID, _ := c.Get("user_id")
		jwtStoreID, _ := c.Get("store_id")

		storeIDStr := c.GetHeader("X-Store-Id")
		if storeIDStr == "" {
			response.Error(c, apperr.New(errcode.BadRequest, "缺少 X-Store-Id 请求头"))
			c.Abort()
			return
		}

		// super_admin wildcard
		if role == "super_admin" && storeIDStr == "*" {
			c.Set("current_store_id", int64(0)) // 0 means all stores in repo layer
			c.Next()
			return
		}

		storeID, err := strconv.ParseInt(storeIDStr, 10, 64)
		if err != nil {
			response.Error(c, apperr.New(errcode.BadRequest, "无效的 X-Store-Id"))
			c.Abort()
			return
		}

		// Verify user has access to this store
		if _, err := authSvc.VerifyStoreAccess(userID.(int64), storeID); err != nil {
			if ae, ok := err.(*apperr.AppError); ok {
				response.Error(c, ae)
			} else {
				response.Error(c, apperr.New(errcode.StoreForbidden, "跨门店访问被拒"))
			}
			c.Abort()
			return
		}

		// User's JWT store_id should match or be super_admin
		if role != "super_admin" && jwtStoreID != storeID {
			response.Error(c, apperr.New(errcode.StoreForbidden, "跨门店访问被拒"))
			c.Abort()
			return
		}

		c.Set("current_store_id", storeID)
		c.Next()
	}
}
```

Add `VerifyStoreAccess` to auth service:

In `backend/internal/module/auth/service.go`, add:
```go
// VerifyStoreAccess checks a user has a role in the given store.
func (s *Service) VerifyStoreAccess(userID, storeID int64) (*StoreInfo, error) {
	usr, err := s.repo.FindUserStoreRole(userID, storeID)
	if err != nil {
		return nil, apperr.New(errcode.StoreForbidden, "无该门店访问权限")
	}
	return &StoreInfo{
		ID:   storeID,
		Name: "",
		Role: usr.Role.Code,
	}, nil
}
```

- [ ] **Step 3: Commit**

```bash
git add backend/internal/middleware/rbac.go backend/internal/middleware/storescope.go backend/internal/module/auth/service.go
git commit -m "feat(phase2): add RBAC and store-scope middleware

- RequirePermission: module:action granularity, super_admin bypass
- StoreScope: X-Store-Id validation, * wildcard for super_admin,
  cross-store access rejection (403/1003)
- auth.Service: GetPermissions + VerifyStoreAccess methods

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 16: Ratelimit + Idempotency middleware

**Files:**
- Create: `backend/internal/middleware/ratelimit.go`
- Create: `backend/internal/middleware/idempotency.go`

- [ ] **Step 1: Write rate limit middleware**

Write `backend/internal/middleware/ratelimit.go`:
```go
package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/response"
)

// RateLimiter limits requests per user per minute using Redis.
// Default: 60 req/min for general endpoints.
func RateLimiter(rdb *redis.Client, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Identify the client: prefer user_id from JWT, fallback to IP
		key := "rate:" + c.ClientIP()
		if uid, exists := c.Get("user_id"); exists {
			key = "rate:" + uidToStr(uid)
		}

		ctx := context.Background()
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			// Redis unavailable: allow request (fail open for availability)
			c.Next()
			return
		}

		if count == 1 {
			rdb.Expire(ctx, key, window)
		}

		if count > int64(limit) {
			response.Error(c, apperr.New(429, "请求过于频繁，请稍后再试"))
			c.Abort()
			return
		}

		c.Next()
	}
}

func uidToStr(uid interface{}) string {
	switch v := uid.(type) {
	case int64:
		return "u" + int64ToStr(v)
	case float64:
		return "u" + int64ToStr(int64(v))
	default:
		return "u0"
	}
}

func int64ToStr(i int64) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}
```

- [ ] **Step 2: Write idempotency middleware**

Write `backend/internal/middleware/idempotency.go`:
```go
package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// Idempotency caches responses for Idempotency-Key requests.
// Replays the cached response if the same key is seen within 24 hours.
func Idempotency(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("Idempotency-Key")
		if key == "" {
			c.Next()
			return
		}

		// Hash the key + path to create a Redis-safe key
		hash := hashKey(key + ":" + c.Request.URL.Path)

		ctx := context.Background()
		cached, err := rdb.Get(ctx, "idem:"+hash).Result()
		if err == nil {
			// Replay cached response
			var resp cachedResponse
			if json.Unmarshal([]byte(cached), &resp) == nil {
				c.Header("Content-Type", "application/json")
				c.String(resp.Status, resp.Body)
				c.Abort()
				return
			}
		}

		// Capture response for caching
		writer := &responseCapture{ResponseWriter: c.Writer}
		c.Writer = writer
		c.Next()

		if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			data, _ := json.Marshal(cachedResponse{
				Status: c.Writer.Status(),
				Body:   writer.body(),
			})
			rdb.Set(ctx, "idem:"+hash, string(data), 24*time.Hour)
		}
	}
}

func hashKey(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[:16]
}

type cachedResponse struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}
```

- [ ] **Step 3: Add responseCapture helper**

In `backend/internal/middleware/idempotency.go`, add:
```go
type responseCapture struct {
	gin.ResponseWriter
	buf []byte
}

func (w *responseCapture) Write(data []byte) (int, error) {
	w.buf = append(w.buf, data...)
	return w.ResponseWriter.Write(data)
}

func (w *responseCapture) body() string {
	return string(w.buf)
}
```

- [ ] **Step 4: Add Redis to main.go and router**

Update `cmd/server/main.go` to connect to Redis:
```go
import "github.com/redis/go-redis/v9"

rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr})
```

Update `router.Setup` to accept `rdb *redis.Client` and wire the middlewares.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/middleware/ratelimit.go backend/internal/middleware/idempotency.go backend/cmd/server/main.go backend/internal/router/router.go
git commit -m "feat(phase2): add rate limiting and idempotency middleware

- RateLimiter: Redis-based sliding window, 60 req/min default,
  per-user (JWT) or per-IP, fail-open if Redis unavailable
- Idempotency: Idempotency-Key header, SHA256 cache key,
  24h replay window, response capture and replay
- Redis connection in main.go passed to middleware

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 17: Final integration — wire everything together

**Files:**
- Modify: `backend/cmd/server/main.go` (final)
- Modify: `backend/internal/router/router.go` (final)

- [ ] **Step 1: Finalize main.go**

The complete `main.go` should:
1. Load config
2. Connect PostgreSQL + Redis
3. Run auto-migrate (golang-migrate)
4. Setup router with all middleware + auth routes
5. Start server with graceful shutdown

- [ ] **Step 2: Finalize router.go**

The complete router should wire:
- Global: TraceID → Logger → Recovery → ErrorHandler → CORS
- Public: /healthz, /readyz, POST /api/v1/auth/*
- Protected: /api/v1/* (AuthRequired + StoreScope + RateLimiter)
- Write endpoints: + Idempotency

- [ ] **Step 3: Run full test suite**

```bash
cd backend && go test ./... -v -count=1
```

Expected: all tests PASS (Phase 1 + Phase 2).

- [ ] **Step 4: Build and verify**

```bash
cd backend && go build -o bin/server ./cmd/server/
docker compose up -d postgres redis
# Run migrations manually or via migrate tool
./bin/server &
curl http://localhost:8080/healthz
curl -X POST http://localhost:8080/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"pawprint123"}'
```

Expected: Health check returns healthy, login returns tokens.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(phase2): final integration — wire auth, RBAC, store-scope, rate limiting

- Complete middleware chain: TraceID → Logger → Recovery → ErrorHandler → CORS
- Auth endpoints: POST /api/v1/auth/{login,refresh,switch-store,logout}
- Protected route group with AuthRequired + StoreScope + RateLimiter
- Idempotency on write endpoints
- Full test suite passing (unit + integration)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Phase 2 Complete — Security Gate

All security test cases must pass before proceeding to business modules:

- [ ] TC-AUTH-01: admin/pawprint123 login → 200 + tokens + stores
- [ ] TC-AUTH-02: wrong password → 401 code=1001
- [ ] TC-AUTH-03: 5 failed attempts → lockout (Redis counter)
- [ ] TC-AUTH-04: switch to unauthorized store → 403 code=1003
- [ ] TC-RBAC-01: staff calls POST /settlements → 403
- [ ] TC-RBAC-02: finance calls POST /appointments → 403
- [ ] TC-RBAC-03: front_desk calls wallet recharge → 200
- [ ] TC-RBAC-04: store_manager calls store:manage → 403
- [ ] TC-ISO-01: front_desk with other store X-Store-Id → 403
- [ ] TC-ISO-02: super_admin with X-Store-Id:* → 200
- [ ] TC-ISO-03: missing X-Store-Id → 400

---

## Next Phase (Phase 3): M2 Dashboard

After Phase 2 security gate passes, implement the Dashboard module following the same TDD pattern:
1. `dashboard/model.go` — KPI response structs
2. `dashboard/repo.go` — aggregation queries
3. `dashboard/service_test.go` → `service.go` — business logic, timezone-aware "today"
4. `dashboard/handler_test.go` → `handler.go` → `router.go`
5. Verify: TC-DASH-01, TC-DASH-02
