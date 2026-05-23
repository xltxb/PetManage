#!/usr/bin/env python3
"""F034 End-to-End Test: Member Level System"""
import json, urllib.request, urllib.error, sys

BASE = "http://localhost:8080"

def api(method, path, body=None, token=None):
    url = f"{BASE}{path}"
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(url, method=method, data=data)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", f"Bearer {token}")
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return resp.status, json.loads(resp.read())
    except urllib.error.HTTPError as e:
        body = e.read().decode()
        try:
            return e.code, json.loads(body)
        except:
            return e.code, body

def check(step, condition, msg):
    if condition:
        print(f"  PASS: {step}: {msg}")
    else:
        print(f"  FAIL: {step}: {msg}")
        sys.exit(1)

# === SETUP ===
print("=== SETUP ===")
# Create new merchant via admin
status, result = api("POST", "/api/v1/auth/login", {"username":"admin","password":"admin123"})
admin_token = result["access_token"]
print(f"Admin token: {admin_token[:20]}...")

# Create merchant
import time
ts = str(int(time.time()))
status, result = api("POST", "/api/v1/merchants/apply", {
    "name": f"F034E2E{ts}",
    "license_number": f"F034-E2E-{ts}",
    "legal_person": "TestPerson",
    "contact_phone": f"13800{ts[-6:]}",
    "address": "TestAddr"
}, admin_token)
check("Apply", result.get("id"), "Merchant application submitted")
mch_id = result["id"]
print(f"  Merchant ID: {mch_id}")

# Approve
status, result = api("POST", f"/api/v1/merchants/{mch_id}/approve?reason=test", None, admin_token)
check("Approve", result.get("status") == "approved", "Merchant approved")
mch_user = result["merchant_admin"]["username"]
mch_pass = result["merchant_admin"]["password"]
print(f"  Merchant user: {mch_user} / {mch_pass}")

# Login as merchant
status, result = api("POST", "/api/v1/merchant/auth/login", {"username": mch_user, "password": mch_pass})
mtoken = result["access_token"]
check("MchLogin", "access_token" in result, "Merchant login OK")
print(f"  Merchant token: {mtoken[:20]}...")

# Create a product for checkout (lots of stock)
status, result = api("POST", "/api/v1/merchant/products", {
    "name": "Test Product",
    "barcode": f"E2E{ts}",
    "price_cents": 10000,
    "cost_cents": 5000,
    "stock": 200,
    "alert_stock": 5
}, mtoken)
check("Product", result.get("id"), "Product created")
product_id = result["id"]

# Create member
status, result = api("POST", "/api/v1/merchant/members", {
    "name": "测试会员张三",
    "phone": f"13801{ts[-6:]}",
    "gender": "M"
}, mtoken)
check("Member", result.get("id"), "Member created")
member_id = result["id"]
print(f"  Member ID: {member_id}")

# ============================================================
# STEP 1: Create level rules
# ============================================================
print("\n=== STEP 1: Create level rules ===")

# Create 黄金会员: total_spending > 500000 (5000 yuan), 10% off, 1.5x points
status, golden = api("POST", "/api/v1/merchant/member-levels", {
    "name": "黄金会员",
    "level_order": 3,
    "upgrade_type": "total_spending",
    "upgrade_value": 500000,
    "discount_percent": 90,
    "points_multiplier": 150,
    "downgrade_days": 180,
    "icon": "gold",
    "color": "#FFD700",
    "description": "累计消费满5000元升级"
}, mtoken)
check("Create黄金会员", golden.get("id") and golden.get("name") == "黄金会员", f"黄金会员 created: discount={golden['discount_percent']}%, points={golden['points_multiplier']}%")

# Create 白银会员: total_spending > 100000 (1000 yuan), 5% off, 1.2x points
status, silver = api("POST", "/api/v1/merchant/member-levels", {
    "name": "白银会员",
    "level_order": 2,
    "upgrade_type": "total_spending",
    "upgrade_value": 100000,
    "discount_percent": 95,
    "points_multiplier": 120,
    "downgrade_days": 90,
    "icon": "silver",
    "color": "#C0C0C0",
    "description": "累计消费满1000元升级"
}, mtoken)
check("Create白银会员", silver.get("id"), "白银会员 created")

# Create default 普通会员
status, default_rule = api("POST", "/api/v1/merchant/member-levels", {
    "name": "普通会员",
    "level_order": 1,
    "upgrade_type": "total_spending",
    "upgrade_value": 0,
    "discount_percent": 100,
    "points_multiplier": 100,
    "downgrade_days": 0,
    "icon": "normal",
    "color": "#888888",
    "description": "默认等级",
    "is_default": True
}, mtoken)
check("Create默认等级", default_rule.get("id") and default_rule["is_default"], "普通会员 (default) created")

# List rules
status, rules_list = api("GET", "/api/v1/merchant/member-levels", None, mtoken)
check("List规则", rules_list.get("total") == 3, f"3 rules listed: {[r['name'] for r in rules_list['rules']]}")
print(f"  Rules: {[(r['name'], r['discount_percent'], r['points_multiplier']) for r in rules_list['rules']]}")

# ============================================================
# STEP 2: Auto-upgrade after meeting conditions
# ============================================================
print("\n=== STEP 2: Auto-upgrade test ===")

# Verify member has no level
status, level_info = api("GET", f"/api/v1/merchant/members/{member_id}/level", None, mtoken)
check("Member初始等级", level_info.get("level_id") == 0, f"Member has no level initially: {level_info}")

# Create checkout for 600000 cents (6000 yuan) which triggers 黄金会员 upgrade
status, checkout = api("POST", "/api/v1/merchant/checkout", {
    "member_id": member_id,
    "items": [{"product_id": product_id, "quantity": 60}],
    "payments": [{"method": "wechat", "amount_cents": 600000}]
}, mtoken)
# 60 items * 10000 cents = 600000 cents - 0% level discount (no level yet) = 600000
if not checkout.get("status") == "completed":
    print(f"  Checkout error: {checkout}")
    sys.exit(1)
check("Checkout完成", checkout.get("status") == "completed", f"Order completed: {checkout.get('order_id')}")

# Check for automatic upgrade
status, level_info = api("GET", f"/api/v1/merchant/members/{member_id}/level", None, mtoken)
check("自动升级", level_info.get("name") == "黄金会员", f"Auto-upgraded to 黄金会员 (total spending 600000 > 500000): {level_info}")

# Check level logs
status, logs = api("GET", f"/api/v1/merchant/members/{member_id}/level-logs", None, mtoken)
check("等级变更日志", logs.get("total", 0) >= 1, f"Level change log exists: total={logs.get('total')}")
if logs.get("logs"):
    log = logs["logs"][0]
    check("升级日志类型", log.get("change_type") == "upgrade", f"Change type is upgrade: {log['change_type']}")
    check("升级目标等级", "黄金会员" in log.get("change_reason", ""), f"Upgrade reason mentions 黄金会员: {log.get('change_reason', '')[:60]}")

# ============================================================
# STEP 3: Auto-downgrade test
# ============================================================
print("\n=== STEP 3: Auto-downgrade test ===")

# Create a second member to test downgrade
status, member2 = api("POST", "/api/v1/merchant/members", {
    "name": "降级会员李四",
    "phone": f"13802{ts[-6:]}",
    "gender": "F"
}, mtoken)
member2_id = member2["id"]
print(f"  Member2 ID: {member2_id}")

# Manually set member2 to 黄金会员 level and create an old order to trigger downgrade
# We can use the check-upgrade endpoint with ?action=downgrade
# First, set the member's level to 黄金会员 by doing a large purchase
status, checkout2 = api("POST", "/api/v1/merchant/checkout", {
    "member_id": member2_id,
    "items": [{"product_id": product_id, "quantity": 60}],
    "payments": [{"method": "wechat", "amount_cents": 600000}]
}, mtoken)
check("Member2购买", checkout2.get("status") == "completed", "Member2 purchase completed")

# Verify member2 is now 黄金会员
status, level2_info = api("GET", f"/api/v1/merchant/members/{member2_id}/level", None, mtoken)
check("Member2黄金升级", level2_info.get("name") == "黄金会员", f"Member2 upgraded to 黄金会员")

# Trigger downgrade check (黄金会员 downgrade_days=0 means no auto downgrade)
# Let's update the 白银会员 rule to have downgrade_days=0 too and test downgrade manually
# First check if downgrade works with the check-upgrade endpoint
status, downgrade_result = api("POST", f"/api/v1/merchant/members/{member2_id}/check-upgrade?action=downgrade", None, mtoken)
print(f"  Downgrade check result: {downgrade_result}")
check("降级检查", "level_changed" in downgrade_result, f"Downgrade check returned: {downgrade_result}")

# Since 黄金会员 has downgrade_days=180 and member2 just made a purchase, no downgrade should happen
check("不降级(刚消费)", downgrade_result.get("level_changed") == False, "Member with recent purchase not downgraded")

# Now test the modify rule doesn't affect existing members
print("\n=== STEP 5: Modify rule doesn't affect existing members ===")
# Update 黄金会员 discount from 90% to 85%
status, updated_golden = api("PUT", f"/api/v1/merchant/member-levels/{golden['id']}", {
    "discount_percent": 85
}, mtoken)
check("修改等级规则", updated_golden.get("discount_percent") == 85, f"Golden rule discount updated to 85%: {updated_golden['discount_percent']}")

# Verify the existing member still has 黄金会员 level (level_id unchanged)
status, member1_level = api("GET", f"/api/v1/merchant/members/{member_id}/level", None, mtoken)
check("现有会员等级不变", member1_level.get("name") == "黄金会员" and member1_level.get("level_id") == golden["id"],
      f"Existing member still 黄金会员: name={member1_level.get('name')}")

# The discount_percent should now be 85 (since we read from the rule dynamically)
check("折扣自动应用新规则", member1_level.get("discount_percent") == 85,
      f"Discount now 85% based on updated rule: discount_percent={member1_level['discount_percent']}")

# ============================================================
# STEP 4: Level discount auto-applied at POS
# ============================================================
print("\n=== STEP 4: POS auto-discount by level ===")

# Cart calculate with 黄金会员 (现在85折)
status, cart = api("POST", "/api/v1/merchant/pos/cart/calculate", {
    "member_id": member_id,
    "items": [{"product_id": product_id, "quantity": 1}]
}, mtoken)
check("POS购物车计算", cart.get("original_cents") == 10000, f"Original: {cart.get('original_cents')}")
check("等级折扣应用", cart.get("level_discount_percent") == 85,
      f"Level discount applied: {cart.get('level_discount_percent')}% off, original={cart.get('original_cents')}, discount={cart.get('level_discount_cents')}, payable={cart.get('payable_cents')}")
check("应付金额正确", cart.get("payable_cents") == 8500, f"Payable after 85% off: 8500 = {cart.get('payable_cents')}")
check("积分倍数", cart.get("points_multiplier") == 150, f"Points multiplier 1.5x: {cart.get('points_multiplier')}")

# Verify a member WITHOUT level gets no discount (100%)
status, cart_no_level = api("POST", "/api/v1/merchant/pos/cart/calculate", {
    "items": [{"product_id": product_id, "quantity": 1}]
}, mtoken)
check("无会员无折扣", cart_no_level.get("payable_cents") == 10000,
      f"No member discount applied: payable={cart_no_level.get('payable_cents')}, level_discount={cart_no_level.get('level_discount_cents')}")

# Verify member lookup returns level info
status, lookup = api("GET", f"/api/v1/merchant/pos/members/lookup?phone=1380119999", None, mtoken)
print(f"  Member lookup: {lookup}")

# Test toggle level rule status
print("\n=== Bonus: Toggle level status ===")
status, toggled = api("POST", f"/api/v1/merchant/member-levels/{silver['id']}/toggle", None, mtoken)
check("禁用白银等级", toggled.get("status") == "inactive", f"Silver level toggled to inactive: {toggled['status']}")
status, toggled2 = api("POST", f"/api/v1/merchant/member-levels/{silver['id']}/toggle", None, mtoken)
check("启用白银等级", toggled2.get("status") == "active", f"Silver level toggled back to active: {toggled2['status']}")

# Test delete level rule
status, deleted = api("DELETE", f"/api/v1/merchant/member-levels/{silver['id']}", None, mtoken)
check("删除白银等级", deleted.get("status") == "deleted", f"Silver level deleted: {deleted}")

print("\n=== ALL TESTS PASSED ===")
