# PawPrint P1 Remaining Acceptance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the explicit P1 acceptance rows in `files/测试用例.md`.

**Architecture:** Keep the existing Go module boundaries. Add small route/service/repository methods only where a P1 acceptance case needs a contract, and reuse current permissions instead of creating a new role model.

**Tech Stack:** Go, Gin, GORM, PostgreSQL, Vue/Vite build smoke gate

**Spec:** `docs/superpowers/specs/2026-06-03-pawprint-p1-remaining-acceptance-design.md`

---

## File Map

- Modify: `backend/internal/module/appointment/dto.go`
  - Add weekly schedule response types.
- Modify: `backend/internal/module/appointment/service.go`
  - Add `GetWeekSchedule`.
- Modify: `backend/internal/module/appointment/handler.go`
  - Add `WeekSchedule` handler.
- Modify: `backend/internal/module/appointment/router.go`
  - Register `GET /appointments/week`.
- Modify/Test: `backend/internal/module/appointment/service_test.go`
  - Cover day grouping and station filtering.
- Modify: `backend/internal/module/boarding/*`
  - Add cancel request, service method, handler, route, and tests.
- Modify: `backend/internal/module/pet/*`
  - Add consumption DTO, repo query, service method, handler, route, and tests.
- Modify: `backend/internal/module/notification/service.go`
  - Send `wechat_mp` during vaccine scan.
- Modify/Test: `backend/internal/module/notification/service_test.go`
  - Cover enabled WeChat mock status.

## Task 1: Appointment Weekly Schedule

**Files:**
- Modify: `backend/internal/module/appointment/dto.go`
- Modify: `backend/internal/module/appointment/service.go`
- Modify: `backend/internal/module/appointment/handler.go`
- Modify: `backend/internal/module/appointment/router.go`
- Test: `backend/internal/module/appointment/service_test.go`

- [ ] **Step 1: Write failing service test**

Append a test that creates three appointments across one week, two on station 7 and one on station 8, then asserts `GetWeekSchedule(1, 7, 2026-06-01)` returns seven days and only the station 7 appointments in the correct dates.

- [ ] **Step 2: Run RED**

```bash
cd backend && GOCACHE=/private/tmp/pet-gocache go test ./internal/module/appointment -run TestGetWeekScheduleGroupsStationAppointments -count=1
```

Expected: fail because `GetWeekSchedule` and response types do not exist.

- [ ] **Step 3: Implement minimal service and route**

Add response structs, service grouping logic, handler query parsing, and route registration at `GET /appointments/week` with `appointment:view`.

- [ ] **Step 4: Run GREEN**

```bash
cd backend && GOCACHE=/private/tmp/pet-gocache go test ./internal/module/appointment -run TestGetWeekScheduleGroupsStationAppointments -count=1
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/module/appointment docs/superpowers
git commit -m "feat(appointment): add weekly station schedule"
```

## Task 2: Boarding Abnormal Cancel

**Files:**
- Modify: `backend/internal/module/boarding/dto.go`
- Modify: `backend/internal/module/boarding/service.go`
- Modify: `backend/internal/module/boarding/handler.go`
- Modify: `backend/internal/module/boarding/router.go`
- Test: `backend/internal/module/boarding/service_test.go`

- [ ] **Step 1: Write failing service test**

Create `TestCancelCheckedInOrderReleasesRoom` with a checked-in order and occupied room. Assert cancel sets order status to `cancelled` and room status to `free`.

- [ ] **Step 2: Run RED**

```bash
cd backend && GOCACHE=/private/tmp/pet-gocache go test ./internal/module/boarding -run TestCancelCheckedInOrderReleasesRoom -count=1
```

- [ ] **Step 3: Implement cancel**

Add `Cancel(id, storeID, operatorID int64, reason string) error`, route `POST /boarding-orders/:id/cancel`, and permission `boarding:checkout`.

- [ ] **Step 4: Run GREEN and commit**

```bash
cd backend && GOCACHE=/private/tmp/pet-gocache go test ./internal/module/boarding -run TestCancelCheckedInOrderReleasesRoom -count=1
git add backend/internal/module/boarding
git commit -m "feat(boarding): add abnormal order cancel"
```

## Task 3: Pet Consumption History

**Files:**
- Modify: `backend/internal/module/pet/dto.go`
- Modify: `backend/internal/module/pet/repo.go`
- Modify: `backend/internal/module/pet/service.go`
- Modify: `backend/internal/module/pet/handler.go`
- Modify: `backend/internal/module/pet/router.go`
- Test: `backend/internal/module/pet/service_test.go`

- [ ] **Step 1: Write failing service test**

Add a mock repo method returning consumption rows and assert the service returns rows ordered newest first.

- [ ] **Step 2: Run RED**

```bash
cd backend && GOCACHE=/private/tmp/pet-gocache go test ./internal/module/pet -run TestGetConsumptionHistory -count=1
```

- [ ] **Step 3: Implement API**

Add `GET /pets/:id/consumption` with `pet:view`. Repository SQL should union appointment, boarding, and settlement rows for the pet.

- [ ] **Step 4: Run GREEN and commit**

```bash
cd backend && GOCACHE=/private/tmp/pet-gocache go test ./internal/module/pet -run TestGetConsumptionHistory -count=1
git add backend/internal/module/pet
git commit -m "feat(pet): add consumption history"
```

## Task 4: WeChat MP Vaccine Notification Mock

**Files:**
- Modify: `backend/internal/module/notification/service.go`
- Test: `backend/internal/module/notification/service_test.go`

- [ ] **Step 1: Write failing service test**

Set feature flags with WeChat enabled, run `ScanVaccineDue`, and assert a `wechat_mp` log is created with status `sent`.

- [ ] **Step 2: Run RED**

```bash
cd backend && GOCACHE=/private/tmp/pet-gocache go test ./internal/module/notification -run TestScanVaccineDueSendsWechatWhenEnabled -count=1
```

- [ ] **Step 3: Implement scan fan-out**

For each due pet, send both `sms` and `wechat_mp`; channel feature flags decide `sent` or `skipped`.

- [ ] **Step 4: Run GREEN and commit**

```bash
cd backend && GOCACHE=/private/tmp/pet-gocache go test ./internal/module/notification -run TestScanVaccineDueSendsWechatWhenEnabled -count=1
git add backend/internal/module/notification
git commit -m "feat(notification): add wechat vaccine mock scan"
```

## Final Gate

Run:

```bash
cd backend && GOCACHE=/private/tmp/pet-gocache go test ./...
cd admin && npm run test:p0-admin
cd admin && npm run build
rg -n "TC-APPT-06|TC-BRD-06|TC-PET-04|TC-NOTIF-03" docs/superpowers/specs docs/superpowers/plans
```

Expected:

- backend tests pass
- admin smoke gates pass
- P1 acceptance rows are represented in design and plan
