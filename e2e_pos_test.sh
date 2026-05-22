#!/usr/bin/env bash
set -euo pipefail

BASE="http://localhost:8080"

echo "=== F048 POS E2E Test ==="

# 1. Admin login
ADMIN_TOKEN=$(curl -sf -X POST "$BASE/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")
echo "1. Admin logged in"

# 2. Create and approve merchant
MERCH_RESP=$(curl -sf -X POST "$BASE/api/v1/merchants/apply" \
  -H "Content-Type: application/json" \
  -d '{"name":"POS测试店2","license_number":"POS_048_T2","legal_person":"王五","contact_phone":"13900000003","address":"测试路3号"}')
MERCH_ID=$(echo "$MERCH_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "2. Merchant $MERCH_ID created"

APPROVE_RESP=$(curl -sf -X POST "$BASE/api/v1/merchants/$MERCH_ID/approve" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN")
echo "3. Merchant approved: $APPROVE_RESP"

USERNAME=$(echo "$APPROVE_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['merchant_admin']['username'])")
PASSWORD=$(echo "$APPROVE_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['merchant_admin']['password'])")
echo "4. Credentials: $USERNAME / $PASSWORD"

# 3. Login and change password
LOGIN_RESP=$(curl -sf -X POST "$BASE/api/v1/merchant/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")
MERCH_TOKEN=$(echo "$LOGIN_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

curl -sf -X POST "$BASE/api/v1/auth/change-password" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MERCH_TOKEN" \
  -d "{\"old_password\":\"$PASSWORD\",\"new_password\":\"pet@123\"}" > /dev/null

LOGIN_RESP=$(curl -sf -X POST "$BASE/api/v1/merchant/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USERNAME\",\"password\":\"pet@123\"}")
MERCH_TOKEN=$(echo "$LOGIN_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")
echo "5. Merchant logged in with new password"

# 4. Create a product
PROD_RESP=$(curl -sf -X POST "$BASE/api/v1/merchant/products" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MERCH_TOKEN" \
  -d '{"barcode":"6901234567890","name":"皇家狗粮2kg","brand":"皇家","specification":"2kg","price_cents":15800,"cost_cents":12000,"stock":50,"alert_stock":5}')
PROD_ID=$(echo "$PROD_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "6. Product $PROD_ID created"

# 5. Create service category and item
CAT_RESP=$(curl -sf -X POST "$BASE/api/v1/merchant/service-categories" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MERCH_TOKEN" \
  -d '{"name":"美容","sort_order":1}')
CAT_ID=$(echo "$CAT_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")

SVC_RESP=$(curl -sf -X POST "$BASE/api/v1/merchant/service-items" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MERCH_TOKEN" \
  -d "{\"category_id\":$CAT_ID,\"name\":\"标准洗澡-小型犬\",\"duration_minutes\":30,\"price_cents\":8000,\"member_price_cents\":6800,\"pet_type\":\"dog\",\"min_weight_kg\":0,\"max_weight_kg\":10}")
SVC_ID=$(echo "$SVC_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "7. Service item $SVC_ID created"

# 6. Create a member
MEMBER_RESP=$(curl -sf -X POST "$BASE/api/v1/merchant/members" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MERCH_TOKEN" \
  -d '{"name":"张三","phone":"13811113333"}')
MEMBER_ID=$(echo "$MEMBER_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "8. Member $MEMBER_ID created"

# 7. Test cart calculate without member
echo ""
echo "=== STEP 1: Cart calculate (no member) ==="
CART1=$(curl -sf -X POST "$BASE/api/v1/merchant/pos/cart/calculate" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MERCH_TOKEN" \
  -d "{\"items\":[{\"product_id\":$PROD_ID,\"quantity\":1},{\"service_item_id\":$SVC_ID,\"quantity\":1}]}")
echo "$CART1" | python3 -m json.tool

ORIG=$(echo "$CART1" | python3 -c "import sys,json; print(json.load(sys.stdin)['original_cents'])")
DISC=$(echo "$CART1" | python3 -c "import sys,json; print(json.load(sys.stdin)['discount_cents'])")
PAY=$(echo "$CART1" | python3 -c "import sys,json; print(json.load(sys.stdin)['payable_cents'])")

if [ "$ORIG" = "23800" ] && [ "$DISC" = "0" ] && [ "$PAY" = "23800" ]; then
  echo "PASS: original=23800 (15800+8000), discount=0, payable=23800"
else
  echo "FAIL: Expected original=23800 discount=0 payable=23800, got $ORIG/$DISC/$PAY"
  exit 1
fi

# 8. Test cart calculate with member (service discount)
echo ""
echo "=== STEP 2: Cart calculate (with member) ==="
CART2=$(curl -sf -X POST "$BASE/api/v1/merchant/pos/cart/calculate" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MERCH_TOKEN" \
  -d "{\"member_id\":$MEMBER_ID,\"items\":[{\"product_id\":$PROD_ID,\"quantity\":1},{\"service_item_id\":$SVC_ID,\"quantity\":1}]}")
echo "$CART2" | python3 -m json.tool

ORIG2=$(echo "$CART2" | python3 -c "import sys,json; print(json.load(sys.stdin)['original_cents'])")
DISC2=$(echo "$CART2" | python3 -c "import sys,json; print(json.load(sys.stdin)['discount_cents'])")
PAY2=$(echo "$CART2" | python3 -c "import sys,json; print(json.load(sys.stdin)['payable_cents'])")

if [ "$ORIG2" = "23800" ] && [ "$DISC2" = "1200" ] && [ "$PAY2" = "22600" ]; then
  echo "PASS: original=23800, discount=1200 (8000-6800), payable=22600"
else
  echo "FAIL: Expected original=23800 discount=1200 payable=22600, got $ORIG2/$DISC2/$PAY2"
  exit 1
fi

# 9. Test member lookup by phone
echo ""
echo "=== STEP 3: Member lookup by phone ==="
MEMBER_LOOKUP=$(curl -sf "$BASE/api/v1/merchant/pos/members/lookup?phone=13811113333" \
  -H "Authorization: Bearer $MERCH_TOKEN")
echo "$MEMBER_LOOKUP" | python3 -m json.tool

ML_NAME=$(echo "$MEMBER_LOOKUP" | python3 -c "import sys,json; print(json.load(sys.stdin)['name'])")
if [ "$ML_NAME" = "张三" ]; then
  echo "PASS: Member found - name=张三"
else
  echo "FAIL: Expected name=张三, got $ML_NAME"
  exit 1
fi

# 10. Test product search by barcode
echo ""
echo "=== STEP 4: Product search by barcode ==="
BARCODE_SEARCH=$(curl -sf "$BASE/api/v1/merchant/products?keyword=6901234567890" \
  -H "Authorization: Bearer $MERCH_TOKEN")
echo "$BARCODE_SEARCH" | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'Found {d[\"total\"]} product(s): {d[\"products\"][0][\"name\"]}')"

# 11. Test checkout with service item and member
echo ""
echo "=== STEP 5: Full checkout with service + member ==="
CHECKOUT_RESP=$(curl -sf -X POST "$BASE/api/v1/merchant/checkout" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MERCH_TOKEN" \
  -d "{\"member_id\":$MEMBER_ID,\"items\":[{\"product_id\":$PROD_ID,\"quantity\":2},{\"service_item_id\":$SVC_ID,\"quantity\":1}],\"payments\":[{\"method\":\"wechat\",\"amount_cents\":38400},{\"method\":\"cash\",\"amount_cents\":200}],\"order_notes\":\"备注测试\"}")
echo "$CHECKOUT_RESP" | python3 -m json.tool

CO_TOTAL=$(echo "$CHECKOUT_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['total_cents'])")
CO_NOTES=$(echo "$CHECKOUT_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['order_notes'])")
if [ "$CO_TOTAL" = "38400" ] && [ "$CO_NOTES" = "备注测试" ]; then
  echo "PASS: Checkout complete. total=38400 (15800*2+6800), notes=备注测试"
else
  echo "FAIL: Expected total=38400 notes=备注测试, got $CO_TOTAL/$CO_NOTES"
  exit 1
fi

echo ""
echo "=== ALL TESTS PASSED ==="
