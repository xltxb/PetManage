# PawPrint P0 Admin Completion Design

> Date: 2026-06-03
> Status: Approved by continuation, pending implementation plan
> Strategy: Fill P0 admin in-progress views from existing backend contracts

## Context

The backend P0 acceptance work has been implemented through focused TDD slices and currently passes `go test ./...`. The admin build also passes, but several routed pages still render static "开发中" views:

- `admin/src/views/boarding/BoardingList.vue`
- `admin/src/views/pet/PetList.vue`
- `admin/src/views/member/MemberList.vue`
- `admin/src/views/inventory/InventoryList.vue`
- `admin/src/views/settlement/SettlementList.vue`
- `admin/src/views/analytics/AnalyticsView.vue`
- `admin/src/views/setting/SettingView.vue`

The existing admin implementation is a compact Vue 3 app using lazy-loaded route views, `admin/src/api/client.ts`, Pinia auth, `kpi-card`, status badges, and restrained operational styling. The next useful milestone is to replace static in-progress views with minimal working screens that expose the backend P0 workflows.

## Goal

Complete the admin P0 user-facing workflow surface without redesigning the application:

- boarding: list orders, check in, check out, care logs
- pets: list customer pets, create pet, add health and weight records
- members: list customers, show wallet balance, recharge and adjust wallet
- inventory: sale out, purchase in, adjustment, safety alerts
- settlements: list, create simple settlement, pay, refund, void
- analytics: show backend report summary if available, otherwise keep a useful empty state
- settings: list, read, and update system settings

The completion target is practical operational coverage: the pages should load, call real API routes, show loading/error/empty states, and expose the P0 actions already supported by the backend.

## Non-Goals

- Large visual redesign or new component library
- Full P1 analytics dashboards
- Real charting library integration
- Real payment provider configuration UI
- Mini-program frontend
- Replacing the existing admin routing or auth store

## UX Direction

Use the existing utilitarian admin style:

- existing `kpi-card` surfaces
- compact headers and action buttons
- simple tables/lists with readable status badges
- small inline forms or modal-like panels within the page
- no marketing hero sections, decorative backgrounds, or nested cards

The screens should be dense enough for repeated operations and should not introduce a landing-page feel.

## Data Flow

All pages use `admin/src/api/client.ts`, which injects `Authorization` and `X-Store-Id`. Views should not duplicate auth or store logic.

Each page owns its own local loading, error, form, and list state using Vue `ref` and `computed`, matching `AppointmentList.vue` and `Dashboard.vue`.

The preferred API patterns are:

- lists: `GET /<resource>?page=1&page_size=20`
- actions: `POST` or `PUT` to existing backend endpoints
- write retries: use an `Idempotency-Key` header for financial and inventory actions where appropriate
- success: reload the relevant list or summary
- failure: show a concise inline error at the top of the page

## Page Scope

### Boarding

Use `/boarding-orders` to list orders and expose check-in/check-out/care-log actions. The page should show order status, room, pet/customer identifiers, planned dates, actual dates, nights, total amount, and action buttons. Check-out must refresh the generated total and room state after success.

### Pets

Use `/pets`, `/pets/:id`, `/pets/:id/health`, `/pets/:id/weights`, and `/customers/:id/pets`. The first P0 screen can be customer-focused: enter customer ID, load pets, create a pet, and add health or weight records for a selected pet.

### Members

Use `/customers`, `/customers/:id`, `POST /customers/:id/wallet`, and `PUT /customers/:id/wallet`. The page should show customer name, phone, tier, wallet, points, total spend, and provide recharge/adjust operations with reason validation for adjustments.

### Inventory

Use `/inventory/alerts`, `/inventory/sale-out`, `/inventory/purchase-in`, and `/inventory/adjust`. The page should show safety alerts first, then operation forms for sale out, purchase in, and adjustment. This is enough to exercise P0 stock and notification behavior.

### Settlements

Use `/settlements`, `/settlements/:id/pay`, `/settlements/:id/refund`, and `/settlements/:id/void`. The page should list settlement status and amount, allow creating a simple settlement with one item, and allow cash/wallet payment, refund, and void operations. WeChat/Alipay disabled responses should surface as normal API errors, not crash the page.

### Analytics

Use `/analytics/report`. The page should display any summary fields the backend returns in a simple grid and list/table. If the backend returns sparse data, show a clear empty state and keep build/runtime stable.

### Settings

Use `/settings`, `/settings/:key`, and `PUT /settings/:key`. The page should list current settings, allow selecting a key, editing value JSON/text, and saving. It should preserve unknown setting shapes without lossy parsing.

## Error Handling

Every page should include:

- a visible loading state
- an empty state when no rows are returned
- an inline error message when API calls fail
- disabled buttons while a write is in progress

Errors from `response.message` should be shown when available. Unknown errors can fall back to a short generic message.

## Testing Strategy

Frontend implementation should be verified with:

```bash
cd admin && npm run build
```

After each page slice, scan for remaining in-progress markers:

```bash
rg -n "开发中|图表开发中" admin/src/views
```

When all in-progress views are replaced, run the backend gate again to confirm no backend regressions:

```bash
cd backend && GOCACHE=/private/tmp/pet-gocache go test ./...
```

If a page needs behavior tests, add them only where the project already has test tooling. The current admin project has no frontend unit test harness, so the first gate is TypeScript plus Vite build.

## Implementation Order

1. Inventory, because it has a small API surface and validates the stock-alert workflow.
2. Members, because wallet actions are central to settlement flows.
3. Settlements, because payment/refund/void ties together the highest-risk backend behavior.
4. Boarding, because checkout now generates settlements.
5. Pets, because it is mostly record management.
6. Settings and analytics, because they are lower-risk operational pages.
7. Final scan and build gate.

This order keeps each page independently useful while steadily removing all admin in-progress views.
