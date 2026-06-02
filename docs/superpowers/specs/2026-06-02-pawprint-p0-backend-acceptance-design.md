# PawPrint P0 Backend Acceptance Design

> Date: 2026-06-02
> Status: Approved design, pending written-spec review
> Strategy: P0 backend acceptance first

## Context

The project already contains a runnable Go/Gin/PostgreSQL backend, Vue admin shell, seed data, API notes, and P0 acceptance cases in `files/测试用例.md`. Earlier rebuild planning covered broad project scaffolding, but the current codebase is no longer empty: it has module packages for auth, dashboard, appointment, boarding, pet, member, inventory, settlement, notification, analytics, and settings.

The next useful milestone is not another broad rebuild. It is to close the backend gaps that block P0 acceptance and later end-to-end UI work.

## Goal

Make backend behavior pass the P0 cases that define PawPrint's core operating flows:

- auth, RBAC, and store isolation
- appointment creation, conflict prevention, state transitions, and notification trigger
- mini-program customer-facing appointment APIs with mock login
- settlement payment/refund side effects, idempotency, wallet, points, inventory, and print jobs
- boarding check-in/check-out billing and settlement generation
- inventory and notification alerts needed by dashboard and daily operations

Frontend placeholder pages and visual polish are outside this design. Admin UI completion should start only after the backend contracts are reliable enough to drive it.

## Non-Goals

- Real WeChat Pay, Alipay, or card payment integration
- Real SMS, WeChat official-account, or external push delivery
- Large admin frontend redesign
- P1 analytics, finance, and advanced settings
- Replacing the existing module structure

Mock or skipped external-channel behavior is acceptable when the P0 tests require graceful handling, such as `FEATURE_SMS_ENABLED=false` producing a skipped notification log.

## Current Gaps

The existing backend has many modules present, but several P0 cross-module behaviors are incomplete or not wired:

- `RequirePermission`, `Idempotency`, and `RateLimiter` middleware exist but are not mounted on protected business routes.
- Login rejects bad passwords, but account lockout after repeated failures is not implemented.
- Auth logout currently returns success without invalidation semantics.
- Settlement `pay` marks the settlement paid, but does not yet own all required side effects: wallet deduction, points, total spend, membership upgrade, inventory decrement, notification or print job generation, and idempotent replay.
- Settlement refund creates red-ink settlement data, but does not yet reverse all relevant side effects.
- Inventory stock reads and mutations need transaction-safe row locking for oversell prevention.
- Notification creation exists as a service, but business modules do not consistently trigger notification logs.
- Mini-program `/wx/*` APIs are not present as first-class customer-facing routes.

## Architecture

Keep the existing module-per-feature layout:

```text
handler -> service interface -> repository interface -> GORM implementation
```

Cross-module operations should be coordinated in service-layer transactions, not in handlers. Settlement is the main orchestrator for payment side effects because it is the point where cash, wallet, points, inventory, and receipt generation become final.

### Route and Middleware Policy

Protected admin APIs remain under `/api/v1` with:

- `AuthRequired`
- `StoreScope`
- `RequirePermission` at module/action granularity
- `Idempotency` on mutating endpoints that can create financial or inventory side effects
- `RateLimiter` on login and sensitive write endpoints

Customer mini-program APIs should be registered separately under `/api/v1/wx`. They need customer authentication after mock login, but should not reuse admin RBAC assumptions.

### Transaction Boundaries

The following operations must be atomic:

- wallet balance update plus `wallet_transactions`
- inventory quantity update plus `stock_transactions`
- settlement payment status plus payment record plus all side effects
- refund red-ink settlement plus reverse stock, wallet, and points effects
- boarding checkout plus settlement creation plus room status update

Any operation that changes more than one table should use a single database transaction. Inventory decrement must lock the relevant inventory row before validating quantity.

### Idempotency

Mutating financial operations should require and honor `Idempotency-Key`:

- settlement pay
- settlement refund
- wallet recharge and adjustment
- inventory purchase receipt
- mini-program appointment create

Repeated requests with the same key, method, path, store, and authenticated actor should return the first successful response and must not repeat side effects.

## Delivery Stages

### Stage 1: M1 Security Gate

Target cases:

- `TC-AUTH-03`
- `TC-RBAC-01` through `TC-RBAC-04`
- `TC-ISO-01` through `TC-ISO-03`

Work:

- mount permissions on business routes
- define a small route-to-permission matrix in router/module registration
- implement failed-login counting and 10-minute lockout
- add rate limiting to login
- verify missing `X-Store-Id`, unauthorized store access, and super-admin wildcard behavior

Exit condition:

All P0 auth, RBAC, and isolation tests pass, and protected business writes cannot bypass permission checks.

### Stage 2: Appointment and Notification Trigger

Target cases:

- `TC-APPT-01` through `TC-APPT-05`
- `TC-NOTIF-01`

Work:

- confirm service duration and `scheduled_end` calculation
- reject overlapping resource windows
- enforce appointment state transitions
- release resource availability on `no_show`
- create `appointment_confirmed` notification logs when appointment creation succeeds

Exit condition:

Backend tests cover conflict detection, valid and invalid transitions, and notification-log creation.

### Stage 3: Mini-Program P0 APIs

Target cases:

- `TC-WX-01` through `TC-WX-04`
- `TC-E2E-06` backend portion

Work:

- add `/api/v1/wx/auth/login` with deterministic mock code handling
- add `/api/v1/wx/service-offerings`
- add `/api/v1/wx/appointments`
- add customer appointment cancellation with the 2-hour cutoff
- reuse appointment conflict and notification behavior instead of duplicating rules

Exit condition:

Mini-program APIs can create and cancel customer appointments with the same core validations as admin-created appointments.

### Stage 4: Settlement Side Effects

Target cases:

- `TC-MEM-02` through `TC-MEM-06`
- `TC-INV-01` through `TC-INV-03`
- `TC-SET-01` through `TC-SET-05`
- `TC-E2E-02`

Work:

- support cash and wallet payment
- return `501 PAYMENT_NOT_ENABLED` for WeChat while real payment is disabled
- generate payment record and receipt print job on successful payment
- deduct wallet and create wallet transaction for wallet payment
- update total spend, points, and membership tier after paid settlement
- decrement retail inventory and create stock transaction
- make repeated payment/refund requests idempotent
- reverse relevant wallet, points, and inventory effects on refund
- reject void on paid settlement

Exit condition:

Settlement payment and refund are transactionally consistent and replay-safe.

### Stage 5: Boarding Checkout

Target cases:

- `TC-BRD-01` through `TC-BRD-05`
- `TC-E2E-03`

Work:

- make check-in occupy the selected room
- calculate checkout nights using `ceil(duration / 24h)` with a minimum of 1 night
- generate boarding settlement on checkout
- set room status to `cleaning`
- ensure boarding ignores member discount

Exit condition:

Boarding can run from check-in to checkout settlement with correct room state and billing.

### Stage 6: Inventory and Daily Notification Alerts

Target cases:

- `TC-INV-04` through `TC-INV-06`
- `TC-NOTIF-02` and `TC-NOTIF-04`
- `TC-DASH-01` stock-alert portion
- `TC-E2E-05`

Work:

- implement purchase receipt and stock transaction creation
- enforce row-lock behavior for concurrent sale decrement
- generate stock-low notification logs
- add daily vaccine-due scan as a callable backend job function
- log skipped SMS when SMS feature flag is disabled

Exit condition:

Inventory balances reconcile with latest stock transaction, oversell is prevented, and operational alerts are persisted.

## Testing Strategy

Every stage should follow red-green-refactor:

1. Add a focused failing test for the P0 case or regression.
2. Implement the smallest service/router/repository change needed.
3. Run the targeted package test.
4. Run `go test ./...` before commit.

For cross-module flows, prefer service-level tests with transaction-backed repositories when behavior spans multiple tables. Handler tests should verify HTTP status, response code, middleware, route wiring, and idempotency behavior.

Frontend build remains a smoke gate because frontend changes are not in scope for this backend-first plan:

```bash
cd backend && go test ./...
cd admin && npm run build
```

## Data and Error Semantics

- Money remains integer cents.
- Times are stored as UTC and returned as ISO8601 values.
- Store-scoped data must always filter by `store_id` unless super-admin wildcard access is explicitly allowed.
- Permission failure uses `403` with the existing RBAC error code.
- Unauthorized store access uses the existing store authorization error code.
- Resource conflict uses `422 RESOURCE_CONFLICT`.
- Invalid state transition uses `409`.
- External payment disabled uses `501 PAYMENT_NOT_ENABLED`.
- Insufficient wallet and insufficient stock use `422`.

## Implementation Order Rationale

Security comes first because later acceptance tests depend on correct route gating and store scope. Appointment and mini-program APIs come next because they produce business records and notification triggers. Settlement follows because it ties together the largest number of side effects and is the highest consistency risk. Boarding and inventory/notification alerts then complete the operational P0 flows.

This sequence keeps the blast radius controlled: each stage can be tested and committed independently, but the final result supports the full P0 backend acceptance path.
