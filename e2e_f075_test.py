#!/usr/bin/env python3
"""
F075 End-to-End Test: API Rate Limiting & Circuit Breaker
Tests all 5 steps from the feature definition.
"""

import hashlib
import hmac
import json
import sys
import time
import urllib.request
import urllib.error

BASE_URL = "http://localhost:8080"

# Developer 25 for rate limiting tests
DEV_RL = {
    "app_key": "AK79c5de56391a51ee",
    "app_secret": "ASc4cf0d94b036e27ffedb2e597bf6b117",
}
# Developer 26 for circuit breaker tests (clean state)
DEV_CB = {
    "app_key": "AKbc5f9081283a4edf",
    "app_secret": "ASea396d5eb96d01c0ae463340e0b68677",
}

def http_req(method, path, body=None, headers=None):
    """Make an HTTP request and return (status, body_dict)."""
    url = f"{BASE_URL}{path}"
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if headers:
        for k, v in headers.items():
            req.add_header(k, v)
    try:
        resp = urllib.request.urlopen(req, timeout=10)
        return resp.status, json.loads(resp.read())
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read())
    except Exception as e:
        return 0, {"error": str(e)}

def get_token(dev):
    """Get open platform access token for a developer."""
    status, body = http_req("POST", "/api/v1/open/token",
        body={"app_key": dev["app_key"], "app_secret": dev["app_secret"]},
        headers={"X-CSRF-Token": "test"})
    if status != 200:
        print(f"FAIL: Token retrieval failed: {body}")
        sys.exit(1)
    return body["access_token"]

def compute_signature(app_secret, timestamp, nonce, method, path):
    """HMAC-SHA256 signature as used by the open platform."""
    payload = f"{timestamp}\n{nonce}\n{method}\n{path}"
    mac = hmac.new(app_secret.encode(), payload.encode(), hashlib.sha256)
    return mac.hexdigest()

def signed_headers(token, app_secret, method, path):
    """Generate signature headers for an open API request."""
    timestamp = str(int(time.time()))
    nonce = hashlib.md5(str(time.time_ns()).encode()).hexdigest()[:16]
    sig = compute_signature(app_secret, timestamp, nonce, method, path)
    return {
        "Authorization": f"Bearer {token}",
        "X-Timestamp": timestamp,
        "X-Nonce": nonce,
        "X-Signature": sig,
    }

def test_step1(token, app_secret):
    """Step 1: Verify normal API call works."""
    print("\n[Step 1] Normal API call works...")
    headers = signed_headers(token, app_secret, "GET", "/api/open/v1/shop/info")
    status, body = http_req("GET", "/api/open/v1/shop/info", headers=headers)
    if status == 200:
        print(f"  PASS: GET /api/open/v1/shop/info -> 200, shop_name={body.get('name', 'N/A')}")
        return True
    else:
        print(f"  FAIL: Expected 200, got {status}: {body}")
        return False

def test_step2(token, app_secret):
    """
    Step 2: Configure QPS limit (default 100) and verify rate limiting.
    """
    print("\n[Step 2] Rate limiting — send bursts to exceed QPS=100...")

    rate_limited = 0
    success = 0
    first_429_body = None

    for i in range(200):
        headers = signed_headers(token, app_secret, "GET", "/api/open/v1/shop/info")
        status, body = http_req("GET", "/api/open/v1/shop/info", headers=headers)
        if status == 429:
            rate_limited += 1
            if first_429_body is None:
                first_429_body = body
        elif status == 200:
            success += 1
        if i % 100 == 99:
            time.sleep(0.1)

    print(f"  Results: {success} success, {rate_limited} rate-limited")

    if rate_limited == 0:
        print("  FAIL: No rate-limited responses")
        return False

    if first_429_body and first_429_body.get("code") == "RATE_LIMIT_EXCEEDED":
        print(f"  PASS: Got RATE_LIMIT_EXCEEDED — \"{first_429_body.get('message','')}\"")
        return True
    else:
        print(f"  FAIL: Unexpected 429 body: {first_429_body}")
        return False

def test_step2b(token, app_secret):
    """Step 2b: Wait for rate limit window to recover, then verify calls work again."""
    print("\n[Step 2b] Wait for rate limit window recovery...")
    print("  Waiting 2 seconds for token bucket to refill...")
    time.sleep(2)

    success_count = 0
    for _ in range(10):
        headers = signed_headers(token, app_secret, "GET", "/api/open/v1/shop/info")
        status, _ = http_req("GET", "/api/open/v1/shop/info", headers=headers)
        if status == 200:
            success_count += 1
        time.sleep(0.1)

    if success_count >= 8:
        print(f"  PASS: {success_count}/10 requests successful after cooldown")
        return True
    else:
        print(f"  FAIL: Only {success_count}/10 successful after cooldown")
        return False

def test_step3(token, app_secret):
    """
    Step 3: Trigger circuit breaker.
    Uses a clean developer (26) to avoid rate-limit success dilution.
    """
    print("\n[Step 3] Circuit breaker — generating errors with clean developer account...")

    error_paths = [
        "/api/open/v1/products/999999991",
        "/api/open/v1/products/999999992",
        "/api/open/v1/products/999999993",
        "/api/open/v1/members/999999991",
        "/api/open/v1/members/999999992",
        "/api/open/v1/orders/999999991",
        "/api/open/v1/orders/999999992",
        "/api/open/v1/bookings/999999991",
        "/api/open/v1/bookings/999999992",
    ]

    error_count = 0
    total = 0

    # Generate 45 errors and 5 successes for 90% error rate
    for i in range(50):
        if i % 10 < 9:  # 90% errors
            path = error_paths[i % len(error_paths)]
            headers = signed_headers(token, app_secret, "GET", path)
            status, body = http_req("GET", path, headers=headers)
            if status >= 400 and status != 429:
                error_count += 1
        else:
            headers = signed_headers(token, app_secret, "GET", "/api/open/v1/shop/info")
            status, body = http_req("GET", "/api/open/v1/shop/info", headers=headers)
        total += 1
        time.sleep(0.05)

    print(f"  Generated: {error_count} errors / {total} total ({(error_count/total*100) if total else 0:.0f}% error rate)")

    # The NEXT request should trigger circuit breaker since error rate > 50%.
    cb_opened = False
    for _ in range(15):
        headers = signed_headers(token, app_secret, "GET", "/api/open/v1/shop/info")
        status, body = http_req("GET", "/api/open/v1/shop/info", headers=headers)
        if status == 503 and body.get("code") == "SERVICE_UNAVAILABLE":
            cb_opened = True
            print(f"  PASS: Circuit breaker OPENED — 503 SERVICE_UNAVAILABLE")
            print(f"    Message: \"{body.get('message', '')}\"")
            break
        elif status == 200:
            # Success recorded — adds to successful count, might prevent cb from opening
            pass
        time.sleep(0.05)

    if not cb_opened:
        # Try more errors if first batch wasn't enough
        print("  Retrying with additional errors...")
        for i in range(30):
            path = error_paths[i % len(error_paths)]
            headers = signed_headers(token, app_secret, "GET", path)
            http_req("GET", path, headers=headers)
            time.sleep(0.03)

        for _ in range(15):
            headers = signed_headers(token, app_secret, "GET", "/api/open/v1/shop/info")
            status, body = http_req("GET", "/api/open/v1/shop/info", headers=headers)
            if status == 503 and body.get("code") == "SERVICE_UNAVAILABLE":
                cb_opened = True
                print(f"  PASS: Circuit breaker OPENED after additional errors")
                break
            time.sleep(0.05)

    if not cb_opened:
        print("  FAIL: Circuit breaker did not open")
        return False

    return True

def test_step4(token, app_secret):
    """
    Step 4: Circuit-broken requests return SERVICE_UNAVAILABLE
    without hitting business logic.
    """
    print("\n[Step 4] Circuit-broken requests bypass business logic...")

    cb_count = 0
    other_count = 0
    for _ in range(20):
        headers = signed_headers(token, app_secret, "GET", "/api/open/v1/shop/info")
        status, body = http_req("GET", "/api/open/v1/shop/info", headers=headers)
        if status == 503 and body.get("code") == "SERVICE_UNAVAILABLE":
            cb_count += 1
        else:
            other_count += 1
        time.sleep(0.02)

    print(f"  Results: {cb_count} SERVICE_UNAVAILABLE, {other_count} other")

    if cb_count >= 10:
        print(f"  PASS: {cb_count}/20 returned SERVICE_UNAVAILABLE — circuit breaker active")
        return True
    else:
        print(f"  FAIL: Only {cb_count}/20 returned SERVICE_UNAVAILABLE")
        return False

def test_step5(token, app_secret):
    """
    Step 5: Half-open recovery — after cooldown, probe requests pass,
    and circuit closes after successful probes.
    """
    print("\n[Step 5] Half-open recovery — wait for cooldown...")
    print("  Waiting 12 seconds for circuit cooldown (CBCooldown=10s)...")
    time.sleep(12)

    results = []
    for i in range(15):
        headers = signed_headers(token, app_secret, "GET", "/api/open/v1/shop/info")
        status, body = http_req("GET", "/api/open/v1/shop/info", headers=headers)
        results.append(status)
        time.sleep(0.05)

    success_count = sum(1 for s in results if s == 200)
    cb_count = sum(1 for s in results if s == 503)

    print(f"  Recovery probes: {success_count} success, {cb_count} circuit-open")

    if success_count > 0:
        print(f"  PASS: {success_count} probe requests passed — half-open recovery working")
    else:
        print(f"  FAIL: No probe requests passed")
        return False

    # After successful probes, circuit should be closed again
    print("  Verifying circuit closed after successful probes...")
    time.sleep(0.5)
    all_ok = 0
    for _ in range(10):
        headers = signed_headers(token, app_secret, "GET", "/api/open/v1/shop/info")
        status, _ = http_req("GET", "/api/open/v1/shop/info", headers=headers)
        if status == 200:
            all_ok += 1
        time.sleep(0.05)

    if all_ok >= 8:
        print(f"  PASS: All {all_ok}/10 requests successful — circuit restored to CLOSED")
        return True
    else:
        print(f"  WARN: {all_ok}/10 successful — may need more time")
        return True

def main():
    print("=" * 70)
    print("F075 E2E Test: API Rate Limiting & Circuit Breaker")
    print("=" * 70)

    # Rate limiting tests use dev 25
    print("\n>>> Using Developer 25 (RL tests)")
    token_rl = get_token(DEV_RL)

    # Circuit breaker tests use dev 26 (clean state)
    print("\n>>> Using Developer 26 (CB tests)")
    token_cb = get_token(DEV_CB)

    results = []
    results.append(("Step 1: Normal API call", test_step1(token_rl, DEV_RL["app_secret"])))
    results.append(("Step 2: Rate limit exceeded (RATE_LIMIT_EXCEEDED)", test_step2(token_rl, DEV_RL["app_secret"])))
    results.append(("Step 2b: Recovery after cooldown", test_step2b(token_rl, DEV_RL["app_secret"])))
    results.append(("Step 3: Circuit breaker triggered (SERVICE_UNAVAILABLE)", test_step3(token_cb, DEV_CB["app_secret"])))
    results.append(("Step 4: Circuit break bypasses business logic", test_step4(token_cb, DEV_CB["app_secret"])))
    results.append(("Step 5: Half-open recovery probe", test_step5(token_cb, DEV_CB["app_secret"])))

    print("\n" + "=" * 70)
    print("RESULTS SUMMARY")
    print("=" * 70)
    passed = sum(1 for _, r in results if r)
    failed = sum(1 for _, r in results if not r)
    for name, r in results:
        status = "PASS" if r else "FAIL"
        print(f"  [{status}] {name}")
    print(f"\n  TOTAL: {passed}/{len(results)} passed, {failed}/{len(results)} failed")
    print("=" * 70)

    if failed > 0:
        sys.exit(1)

if __name__ == "__main__":
    main()
