# PawPrint Issue Review

Review date: 2026-06-03
Scope: whole repository review focused on current backend/admin behavior, acceptance tests, route wiring, permissions, and build gates.

## Current Baseline

- Backend gate: `GOCACHE=/private/tmp/pet-gocache go test ./...` passed before fixes.
- Admin gate: `npm run build` passed before fixes.
- Existing unrelated dirty workspace items were not reviewed as committed code:
  - `admin/vite.config.ts`
  - `.playwright-mcp/`

## Findings

### P0-1 Settlement refund does not reverse member and inventory side effects

Status: Fixed

Evidence:
- [files/测试用例.md](files/测试用例.md) `TC-SET-04` requires refunding a paid settlement to create a red-ink reversal and reverse points/inventory effects.
- [backend/internal/module/settlement/service.go](backend/internal/module/settlement/service.go) defines `MemberEffects.ReverseSettlement` and `InventoryEffects.ReverseSaleOut`, but `Refund` does not call either.
- Existing settlement tests assert the red-ink settlement, but do not assert member or inventory reversal.
- [backend/internal/module/member/service.go](backend/internal/module/member/service.go) previously reversed total spend only; it did not deduct earned points or write a refund points transaction.

Impact:
- Paid product settlements can be refunded while customer spend/points and sold inventory remain unchanged, causing accounting and stock drift.

TDD repair record:
- RED: added `TestRefundSettlementReversesMemberAndProductEffects`; it failed because `member reverse amount = 0`.
- RED: added `TestReverseSettlementDeductsEarnedPoints`; it failed because `PointsBalance = 300, want 0`.
- GREEN: updated `Refund` to load settlement items, call `ReverseSettlement`, and reverse each product sale through `ReverseSaleOut`.
- GREEN: updated `ReverseSettlement` to deduct earned points and write a `refund` points transaction.
- Verification:
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/settlement -run TestRefundSettlementReversesMemberAndProductEffects -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/settlement -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/member -run TestReverseSettlementDeductsEarnedPoints -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/member -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./...`
  - `npm run test:p0-admin`
  - `npm run build`
- Latest verification: all commands above were re-run on 2026-06-03 and passed against the current worktree.

### P0-2 System settings page exposes raw JSON editing

Status: Fixed

Evidence:
- The previous [admin/src/views/setting/SettingView.vue](admin/src/views/setting/SettingView.vue) used a free-form `配置值 JSON` textarea and asked operators to edit setting values directly.
- Store operators need page-level controls for operational settings, not raw JSON/key editing.

Impact:
- Incorrect JSON or accidental key edits can break store configuration, and the page is not usable for non-technical back-office operators.

TDD repair record:
- RED: updated `admin/scripts/verify-p0-admin-completion.mjs` to require page controls such as `form.businessOpen`, `form.smsEnabled`, `form.checkoutRound`, and to reject `配置值 JSON` / `<textarea`; it failed against the old page.
- RED: added `TestUpsertStoreSettingUsesTypedStorePredicate`; it failed because store-specific setting upsert generated `store_id IS NULL AND 1 IS NULL`, which PostgreSQL rejects during save.
- GREEN: replaced the raw JSON editor with grouped form controls for feature flags, business hours, appointment reminders, boarding checkout rules, inventory, member, and points settings.
- GREEN: `saveAllSettings` now serializes form values into the existing `/settings/:key` API payloads, so the backend contract stays unchanged while the UI becomes page-configurable.
- GREEN: split settings repository upsert into typed store-specific and global predicates, and ordered `GetAll` so store-specific overrides win over global defaults.
- Verification:
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/setting -run TestUpsertStoreSettingUsesTypedStorePredicate -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/setting -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./...`
  - `npm run test:p0-admin`
  - `npm run build`
  - Browser verification: logged in as `admin`, opened `/settings`, confirmed form controls render, clicked `保存全部设置`, and observed all `/api/v1/settings/:key` PUT requests return 200.

### P0-3 System settings API accepts unsupported keys and invalid values

Status: Fixed

Evidence:
- After replacing the raw JSON editor with page controls, the backend still accepted arbitrary setting keys and malformed values through `PUT /api/v1/settings/:key`.
- The settings page also relied on native input attributes only, so invalid time ranges, negative reminders, or invalid points values could still reach the API.

Impact:
- A malformed request or page regression could persist unsupported configuration keys or invalid operational parameters, breaking downstream appointment, boarding, inventory, member, and points rules.

TDD repair record:
- RED: added `TestSetRejectsUnsupportedSettingKey`; it failed because unsupported setting keys were persisted.
- RED: added `TestSetValidatesSettingValueShape`; it failed because string booleans, malformed business hours, invalid boarding round rules, non-positive nights, negative reminder hours, and negative points rules were accepted.
- GREEN: added explicit backend schemas for the page-configurable settings keys, including boolean flags, business hours, boarding checkout rules, appointment/pet/member numeric ranges, inventory/member switches, and points rules.
- GREEN: added `validateForm` to the settings page so obvious invalid control values are stopped before saving.
- GREEN: preserved the `设置已保存` success message after the page refreshes saved settings.
- Verification:
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/setting -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./...`
  - `npm run test:p0-admin`
  - `npm run build`
  - Browser verification: normal settings save returned 12 setting `PUT` responses with 200 plus a refreshed `GET /settings`; changing `最少计费晚数` to 0 showed validation text and sent no settings request.

### P0-4 Appointment available slots ignore configured business hours

Status: Fixed

Evidence:
- [files/PawPrint宠物店SaaS开发文档.md](files/PawPrint宠物店SaaS开发文档.md) requires customer booking slots to use `system_settings: store.business_hours`.
- [backend/internal/module/appointment/service.go](backend/internal/module/appointment/service.go) still generated available slots from hardcoded 09:00-21:00 even after the settings page could save different hours.

Impact:
- Operators could change business hours in the settings page, but appointment availability still exposed the old default hours.

TDD repair record:
- RED: added `TestGetAvailableSlotsUsesConfiguredBusinessHours`; it failed because appointment service had no settings provider and always used 09:00-21:00.
- GREEN: added `WithSettings` / `SettingsProvider` to appointment service, read `store.business_hours`, and kept 09:00-21:00 as fallback when no setting exists.
- GREEN: wired `settingSvc` into appointment service during router setup so runtime availability uses saved store settings.
- Verification:
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/appointment -run TestGetAvailableSlotsUsesConfiguredBusinessHours -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/appointment -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/setting -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./...`
  - Browser/API verification: temporarily changed `store.business_hours` to 10:00-12:00, `GET /appointments/available-slots?station_id=1&date=2026-06-03` returned only `10:00-10:30` through `11:30-12:00`, then restored the original 09:00-21:00 setting.

### P0-5 Inventory negative-stock setting is not applied to sale-out

Status: Fixed

Evidence:
- [files/PawPrint宠物店SaaS开发文档.md](files/PawPrint宠物店SaaS开发文档.md) defines `inventory.allow_negative=false` as the default setting that controls whether stock shortage blocks sale-out.
- [backend/internal/module/inventory/service.go](backend/internal/module/inventory/service.go) always rejected sale-out when `quantity < 出库量`, even if the settings page saved `inventory.allow_negative=true`.

Impact:
- Operators could enable negative stock in the settings page, but product sale-out and settlement product effects still failed on shortage.

TDD repair record:
- RED: added `TestSaleOutAllowsNegativeStockWhenConfigured`; it failed because inventory service had no settings provider and always returned `INSUFFICIENT_STOCK`.
- GREEN: added `WithSettings` / `SettingsProvider` to inventory service and read `inventory.allow_negative` during sale-out.
- GREEN: wired `settingSvc` into inventory service during router setup so runtime sale-out uses saved store settings.
- Verification:
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/inventory -run TestSaleOutAllowsNegativeStockWhenConfigured -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/inventory -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/setting -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./...`
  - Browser/API verification: with `inventory.allow_negative=false`, product 1 sale-out quantity 999 returned 422 `库存不足`; with `inventory.allow_negative=true`, the same sale-out returned 200; then quantity 999 was purchased back in and the original setting was restored.

### P0-6 Points rule setting is not applied to earned and refunded points

Status: Fixed

Evidence:
- [files/PawPrint宠物店SaaS开发文档.md](files/PawPrint宠物店SaaS开发文档.md) requires successful consumption points to be booked by `points.rule`.
- [backend/internal/module/member/service.go](backend/internal/module/member/service.go) always calculated points with `tier.points_rate` and ignored saved `points.rule.per_yuan` / `points.rule.by_tier_rate`.

Impact:
- Operators could change points settings in the settings page, but settlement payment and refund still used hardcoded tier-only point rules.

TDD repair record:
- RED: added `TestEarnPointsUsesConfiguredPointsRule`; it failed because member service had no settings provider and gold-tier ¥200 still earned 300 points instead of configured 400 points.
- RED: added `TestReverseSettlementUsesConfiguredPointsRule`; it failed for the same reason when refunding configured earned points.
- GREEN: added `WithSettings` / `SettingsProvider` to member service, parsed `points.rule`, and reused the same points calculation for earn and refund reversal.
- GREEN: wired `settingSvc` into member service during router setup so runtime settlement/member effects use saved points settings.
- Verification:
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/member -run 'TestEarnPointsUsesConfiguredPointsRule|TestReverseSettlementUsesConfiguredPointsRule' -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/member -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/settlement -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/setting -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./...`
  - Browser/API verification: temporarily changed `points.rule` to `{per_yuan:2, by_tier_rate:false}`, paid a ¥200 cash settlement for customer 5, observed points increase by 400 and total spend by 20000, refunded the settlement, observed both deltas return to 0, then restored the original points rule.

### P0-7 Settlement code generation collides during new settlement creation

Status: Fixed

Evidence:
- Runtime verification for `points.rule` exposed `ERROR: duplicate key value violates unique constraint "settlements_code_key"` while creating a new settlement.
- [backend/internal/module/settlement/repo.go](backend/internal/module/settlement/repo.go) generated codes with `now.UnixNano()%1000`, which can repeatedly produce the same `SYYYYMMDD000` suffix during bursts.

Impact:
- Cashier-created settlements can fail with 500 due to duplicate settlement codes, blocking payment and downstream member/inventory effects.

TDD repair record:
- RED: added `TestGenerateCodeDoesNotCollideInBurst`; it failed immediately with duplicate `S20260603000`.
- GREEN: changed `GenerateCode` to include the date prefix, nanosecond-of-second entropy, and an atomic in-process sequence.
- Verification:
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/settlement -run TestGenerateCodeDoesNotCollideInBurst -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/settlement -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./...`
  - Browser/API verification: created runtime settlement `S20260603025334000001` without duplicate-code failure.

### P0-8 Customer updates write the wrong WeChat OpenID column

Status: Fixed

Evidence:
- Runtime settlement payment failed while updating a member with `ERROR: column "wechat_open_id" of relation "customers" does not exist`.
- Schema defines `customers.wechat_openid`, but [backend/internal/module/member/model.go](backend/internal/module/member/model.go) let GORM infer `wechat_open_id` from `WechatOpenID`.

Impact:
- Any member update path that saves a full customer record can fail with 500, including settlement payment member spend/points effects.

TDD repair record:
- RED: added `TestCustomerWechatOpenIDMapsToSchemaColumn`; it failed because GORM mapped `WechatOpenID` to `wechat_open_id`.
- GREEN: added `gorm:"column:wechat_openid"` to the model field.
- Verification:
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/member -run TestCustomerWechatOpenIDMapsToSchemaColumn -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/member -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/settlement -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./...`
  - Browser/API verification: payment for customer 5 completed without writing an empty duplicate `wechat_openid`.

### P0-9 Receipt print jobs write invalid operator ID for system actions

Status: Fixed

Evidence:
- Runtime settlement payment failed after member effects with `insert or update on table "print_jobs" violates foreign key constraint "print_jobs_operator_id_fkey"`.
- [backend/internal/module/settlement/repo.go](backend/internal/module/settlement/repo.go) wrote `operator_id=0` into `print_jobs`, but `0` is not a valid `users.id`.

Impact:
- Settlement payment can return 500 at receipt creation even after payment/member effects have already been applied.

TDD repair record:
- RED: added `TestPrintJobOperatorIDValueOmitsZero`; it failed because the helper did not exist and zero operator IDs were not normalized.
- GREEN: `CreateReceipt` now stores `NULL` for `operator_id` when the operator ID is not positive.
- Verification:
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/settlement -run 'TestPrintJobOperatorIDValueOmitsZero|TestGenerateCodeDoesNotCollideInBurst' -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/settlement -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./...`
  - Browser/API verification: cash payment with `operator_id=0` completed and generated receipt work without `print_jobs_operator_id_fkey` failure.

### P0-10 Points refund transaction type violates schema

Status: Fixed

Evidence:
- Runtime settlement refund failed with `points_transactions_type_check` while inserting `type='refund'`.
- Schema allows point transaction types `earn`, `redeem`, `adjust`, and `expire`; `refund` is not a valid points transaction type.

Impact:
- Refunds can update member balances and then fail while writing the points transaction, returning 500 and leaving an incomplete audit trail.

TDD repair record:
- RED: updated `TestReverseSettlementDeductsEarnedPoints` to require the schema-valid `adjust` type for refund point deductions; it failed because the service emitted `refund`.
- GREEN: changed refund point deduction transactions to use `TxAdjust` with remark `退款扣回积分`.
- Verification:
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/member -run TestReverseSettlementDeductsEarnedPoints -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/member -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/settlement -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./...`
  - Browser/API verification: refunding the runtime settlement deducted the configured 400 points through a schema-valid points adjustment and restored the member balance.

### P0-11 Customer updates overwrite empty WeChat OpenID and violate unique index

Status: Fixed

Evidence:
- Runtime settlement payment failed with `duplicate key value violates unique constraint "customers_wechat_openid_key"` while saving a customer with empty `wechat_openid`.
- Existing seed rows have `NULL` WeChat OpenID values, but full-record saves turned missing OpenID into `''`, which is unique-indexed and can collide.

Impact:
- Any payment or member update path can fail for customers without a WeChat OpenID after another customer has already been saved with an empty string.

TDD repair record:
- RED: added `TestCustomerUpdateFieldsOmitEmptyWechatOpenID`; it failed because update-field normalization did not exist.
- GREEN: changed member repository updates to use explicit mutable fields and include `wechat_openid` only when it is non-empty.
- Verification:
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/member -run TestCustomerUpdateFieldsOmitEmptyWechatOpenID -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/member -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/settlement -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./...`

### P0-12 Mini-program appointment settings are ignored

Status: Fixed

Evidence:
- [files/PawPrint宠物店SaaS开发文档.md](files/PawPrint宠物店SaaS开发文档.md) requires a degraded mode with only backend booking when external/customer-facing channels are unavailable.
- The settings page exposes `feature.online_booking_enabled` and `appointment.cancel_deadline_hours`, but [backend/internal/module/wx/service.go](backend/internal/module/wx/service.go) always allowed mini-program booking and hardcoded cancellation cutoff to 2 hours.

Impact:
- Operators could disable online booking or change cancellation policy in settings, but customer-facing mini-program APIs did not honor those settings.

TDD repair record:
- RED: added `TestCreateAppointmentRejectsWhenOnlineBookingDisabled`; it failed because wx service had no settings provider and still created appointments.
- RED: added `TestCancelAppointmentUsesConfiguredDeadline`; it failed because wx cancellation always used a 2-hour cutoff.
- GREEN: added `WithSettings` / `SettingsProvider` to wx service, checked `feature.online_booking_enabled` before creating appointments, and read `appointment.cancel_deadline_hours` when cancelling.
- GREEN: wired `settingSvc` into wx service during router setup.
- Runtime acceptance:
  - `feature.online_booking_enabled=false` then `POST /api/v1/wx/appointments` returned `400` with `线上预约已关闭，请联系门店预约`.
  - Restored online booking, set `appointment.cancel_deadline_hours=1`, created a mini-program appointment about 90 minutes in the future, then `POST /api/v1/wx/appointments/5/cancel?store_id=1` returned `200`.
  - Restored settings to `feature.online_booking_enabled=true` and `appointment.cancel_deadline_hours=2`; deleted runtime appointment `5` and its item row.
- Verification:
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/wx -run 'TestCreateAppointmentRejectsWhenOnlineBookingDisabled|TestCancelAppointmentUsesConfiguredDeadline' -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/wx -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/appointment -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./internal/module/setting -count=1`
  - `GOCACHE=/private/tmp/pet-gocache go test ./...`
