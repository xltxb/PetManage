# PawPrint P1 Remaining Acceptance Design

> Date: 2026-06-03
> Status: Approved by continuation
> Strategy: Implement the explicit P1 gaps from `files/测试用例.md` without expanding into full Phase 2 scope

## Context

The current repository has completed the P0 backend and admin acceptance surface. The remaining explicit acceptance gaps are the P1 rows in `files/测试用例.md`:

- `TC-APPT-06`: station weekly schedule view
- `TC-BRD-06`: abnormal boarding cancellation by store manager, releasing the room
- `TC-PET-04`: pet consumption history aggregated from appointments, boarding, and settlements
- `TC-NOTIF-03`: mock WeChat MP notification sends when the feature flag is enabled

The goal is to make these P1 acceptance cases testable through backend contracts. This is not a full P1 product expansion.

## Goals

1. Add a weekly appointment schedule API grouped by day for a station.
2. Add an abnormal boarding cancel operation for checked-in orders.
3. Add a pet consumption-history API that returns a single chronological feed from existing records.
4. Extend vaccine-due notification scanning so enabled `wechat_mp` behaves as a mock sent channel.

## Non-Goals

- Full calendar UI redesign.
- Full boarding exception workflow with approvals, fees, or audit pages.
- Full finance/day-close module.
- Real WeChat, SMS, or payment provider integration.
- New frontend pages unless needed to expose a completed backend contract.

## API Design

### Appointment Weekly Schedule

Add:

```text
GET /api/v1/appointments/week?station_id=1&week_start=2026-06-01
```

Response:

```json
{
  "station_id": 1,
  "week_start": "2026-06-01",
  "week_end": "2026-06-08",
  "days": [
    {
      "date": "2026-06-01",
      "appointments": [
        {
          "id": 10,
          "status": "pending",
          "scheduled_start": "2026-06-01T10:00:00Z",
          "scheduled_end": "2026-06-01T11:30:00Z",
          "customer_id": 1,
          "pet_id": 1,
          "contact_name": "王梓萱",
          "total_amount": 26800
        }
      ]
    }
  ]
}
```

The service should reuse `ListByStore`, filter by `station_id`, and exclude deleted rows. Cancelled/no-show rows remain visible because a schedule view should show historical occupancy and gaps explicitly.

### Boarding Abnormal Cancel

Add:

```text
POST /api/v1/boarding-orders/:id/cancel
```

Request:

```json
{ "reason": "宠物提前接走", "operator_id": 3 }
```

Only `checked_in` orders can be cancelled. The order becomes `cancelled`; if it has a room, the room becomes `free`. Route permission uses `boarding:checkout`, because seed data grants this to store managers and it is the closest existing operational permission.

### Pet Consumption History

Add:

```text
GET /api/v1/pets/:id/consumption
```

Response is a chronological list:

```json
[
  { "type": "appointment", "source_id": 1, "occurred_at": "...", "title": "预约服务", "amount": 26800, "status": "completed" },
  { "type": "boarding", "source_id": 2, "occurred_at": "...", "title": "寄养服务", "amount": 50400, "status": "checked_out" },
  { "type": "settlement", "source_id": 3, "occurred_at": "...", "title": "结算单", "amount": 50400, "status": "paid" }
]
```

The first implementation can use SQL joins in the pet repository so the service remains a thin coordinator.

### WeChat MP Mock Notification

`NotificationService.ScanVaccineDue` currently sends SMS. It should also send `wechat_mp` for each due pet. Channel state remains controlled by `SetFeatureFlags`; when WeChat is enabled, the log status is `sent`, and when disabled it is `skipped`.

## Testing Strategy

Use backend TDD for every P1 gap:

- appointment service/handler test for weekly schedule grouping and station filtering
- boarding service/handler test for cancel state and room release
- pet service/repo-facing test for consumption aggregation shape
- notification service test for `wechat_mp` scan behavior

Run gates after each slice:

```bash
cd backend && GOCACHE=/private/tmp/pet-gocache go test ./internal/module/<module>
cd backend && GOCACHE=/private/tmp/pet-gocache go test ./...
cd admin && npm run build
```

## Completion Criteria

- All four P1 rows from `files/测试用例.md` have explicit backend routes or service behavior.
- Each route is protected by the closest existing permission.
- Each behavior has a failing test observed before implementation and a passing test after implementation.
- Existing P0 gates remain green.
