//go:build integration

package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"
)

// =============================================================================
// F084: API Interface Automated Tests — Open Platform APIs
// =============================================================================
//
// Test Matrix:
//   Public endpoints (no auth):
//     POST /api/v1/open/developers/apply
//     GET  /api/v1/open/developers/{id}
//     PUT  /api/v1/open/developers/{id}/apply
//     GET  /api/v1/open/developers/permissions/available
//     POST /api/v1/open/developers/{id}/request-permissions
//     POST /api/v1/open/token
//     POST /api/v1/open/token/refresh
//
//   Platform admin endpoints:
//     GET  /api/v1/open/developers/pending
//     GET  /api/v1/open/developers
//     POST /api/v1/open/developers/{id}/approve
//     POST /api/v1/open/developers/{id}/reject
//     PUT  /api/v1/open/developers/{id}/permissions
//
//   OpenAPI auth endpoints:
//     GET /api/v1/open/ping
//     GET /api/open/v1/shop/info
//     GET /api/open/v1/products
//     GET /api/open/v1/products/{id}
//     GET /api/open/v1/services
//     GET /api/open/v1/breeds
//
// Each endpoint tests: normal, missing-params, invalid-token, signature-error.
// =============================================================================

var (
	baseURL         string
	adminToken      string
	testDeveloperID int64
	testAppKey      string
	testAppSecret   string
	testAccessToken string
	testResults     []TestResult
)

// TestResult records a single test case outcome.
type TestResult struct {
	Endpoint   string `json:"endpoint"`
	Scenario   string `json:"scenario"`
	Passed     bool   `json:"passed"`
	StatusCode int    `json:"status_code"`
	Detail     string `json:"detail,omitempty"`
}

// --- JSON helpers -----------------------------------------------------------

type errorResp struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type loginResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type devApp struct {
	ID           int64  `json:"id"`
	CompanyName  string `json:"company_name"`
	ContactPerson string `json:"contact_person"`
	ContactPhone string `json:"contact_phone"`
	ContactEmail string `json:"contact_email"`
	UsagePurpose string `json:"usage_purpose"`
	CallbackURL  string `json:"callback_url"`
	Status       string `json:"status"`
	AppKey       string `json:"app_key,omitempty"`
	AppSecret    string `json:"app_secret,omitempty"`
}

type approveResp struct {
	devApp
	AppKeyResp    string `json:"app_key"`
	AppSecretResp string `json:"app_secret"`
}

type tokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type permListResp struct {
	Permissions []struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"permissions"`
}

// --- Helpers ----------------------------------------------------------------

func apiURL(path string) string       { return baseURL + path }
func apiURLf(format string, args ...interface{}) string { return baseURL + fmt.Sprintf(format, args...) }

func do(t *testing.T, method, url string, body interface{}, headers map[string]string) (*http.Response, []byte) {
	if t != nil {
		t.Helper()
	}
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		if t != nil {
			t.Fatalf("new request: %v", err)
		}
		panic(fmt.Sprintf("new request: %v", err))
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if t != nil {
			t.Fatalf("do request: %v", err)
		}
		panic(fmt.Sprintf("do request: %v", err))
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp, data
}

func computeSig(appSecret, timestamp, nonce, method, path string) string {
	payload := timestamp + "\n" + nonce + "\n" + method + "\n" + path
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func openAuthHeaders(token, appSecret, method, path string) map[string]string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := fmt.Sprintf("test-%d", time.Now().UnixNano())
	sig := computeSig(appSecret, ts, nonce, method, path)
	return map[string]string{
		"Authorization": "Bearer " + token,
		"X-Timestamp":   ts,
		"X-Nonce":       nonce,
		"X-Signature":   sig,
	}
}

func record(endpoint, scenario string, passed bool, statusCode int, detail string) {
	testResults = append(testResults, TestResult{
		Endpoint: endpoint, Scenario: scenario,
		Passed: passed, StatusCode: statusCode, Detail: detail,
	})
}

func assertStatus(t *testing.T, resp *http.Response, want int, endpoint, scenario string) bool {
	t.Helper()
	if resp.StatusCode == want {
		record(endpoint, scenario, true, resp.StatusCode, "")
		return true
	}
	record(endpoint, scenario, false, resp.StatusCode,
		fmt.Sprintf("expected %d, got %d", want, resp.StatusCode))
	t.Errorf("[%s/%s] status: want %d, got %d", endpoint, scenario, want, resp.StatusCode)
	return false
}

func assertErrorCode(t *testing.T, data []byte, code string, endpoint, scenario string) {
	t.Helper()
	var e errorResp
	if err := json.Unmarshal(data, &e); err != nil {
		record(endpoint, scenario, false, 0, "JSON parse: "+err.Error())
		t.Errorf("[%s/%s] JSON parse: %v", endpoint, scenario, err)
		return
	}
	if e.Code == code {
		record(endpoint, scenario, true, 0, "")
	} else {
		record(endpoint, scenario, false, 0, fmt.Sprintf("want code %s, got %s", code, e.Code))
		t.Errorf("[%s/%s] error code: want %s, got %s", endpoint, scenario, code, e.Code)
	}
}

// --- Setup ------------------------------------------------------------------

func TestMain(m *testing.M) {
	baseURL = os.Getenv("TEST_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// 1. Login as platform admin.
	fmt.Println("=== SETUP: Login as admin ===")
	resp, data := do(nil, "POST", apiURL("/api/v1/auth/login"),
		map[string]interface{}{"username": "admin", "password": "admin123"}, nil)
	if resp.StatusCode != 200 {
		fmt.Printf("FATAL: admin login returned %d: %s\n", resp.StatusCode, string(data))
		os.Exit(1)
	}
	var lr loginResp
	json.Unmarshal(data, &lr)
	adminToken = lr.AccessToken
	fmt.Println("  admin login OK, got token")

	// 2. Create a developer application.
	fmt.Println("=== SETUP: Create developer ===")
	resp, data = do(nil, "POST", apiURL("/api/v1/open/developers/apply"), map[string]interface{}{
		"company_name":   "TestDevCo",
		"contact_person": "Test Dev",
		"contact_phone":  "13800138000",
		"contact_email":  "testdev@example.com",
		"usage_purpose":  "Integration testing for CI",
		"callback_url":   "https://testdev.example.com/callback",
	}, nil)
	if resp.StatusCode != 201 {
		fmt.Printf("FATAL: create developer returned %d: %s\n", resp.StatusCode, string(data))
		os.Exit(1)
	}
	var dev devApp
	json.Unmarshal(data, &dev)
	testDeveloperID = dev.ID
	fmt.Printf("  developer created with ID=%d\n", testDeveloperID)

	// 3. Approve the developer.
	fmt.Println("=== SETUP: Approve developer ===")
	hdr := map[string]string{"Authorization": "Bearer " + adminToken}
	resp, data = do(nil, "POST", apiURLf("/api/v1/open/developers/%d/approve?merchant_id=1", testDeveloperID), nil, hdr)
	if resp.StatusCode != 200 {
		fmt.Printf("FATAL: approve developer returned %d: %s\n", resp.StatusCode, string(data))
		os.Exit(1)
	}
	var ar approveResp
	json.Unmarshal(data, &ar)
	testAppKey = ar.AppKeyResp
	testAppSecret = ar.AppSecretResp
	fmt.Printf("  developer approved, AppKey=%s\n", testAppKey)

	// 4. Get open platform access token.
	fmt.Println("=== SETUP: Get open platform token ===")
	resp, data = do(nil, "POST", apiURL("/api/v1/open/token"), map[string]interface{}{
		"app_key":    testAppKey,
		"app_secret": testAppSecret,
	}, nil)
	if resp.StatusCode != 200 {
		fmt.Printf("FATAL: get token returned %d: %s\n", resp.StatusCode, string(data))
		os.Exit(1)
	}
	var tp tokenPair
	json.Unmarshal(data, &tp)
	testAccessToken = tp.AccessToken
	fmt.Println("  open platform access token obtained")

	// Run all tests.
	code := m.Run()

	// Print summary.
	fmt.Println("\n================================================================================")
	fmt.Println(" TEST RESULTS SUMMARY")
	fmt.Println("================================================================================")
	passed, failed := 0, 0
	for _, r := range testResults {
		status := "PASS"
		if !r.Passed {
			status = "FAIL"
			failed++
		} else {
			passed++
		}
		fmt.Printf("  [%s] %-50s %-20s  %s\n", status, r.Endpoint, r.Scenario, r.Detail)
	}
	total := passed + failed
	fmt.Printf("\n  Total: %d  Passed: %d  Failed: %d  Rate: %.1f%%\n", total, passed, failed,
		float64(passed)/float64(total)*100)
	fmt.Println("================================================================================")

	os.Exit(code)
}

// =============================================================================
// Public Endpoints — POST /api/v1/open/developers/apply
// =============================================================================

func TestApplyDev_Normal(t *testing.T) {
	resp, data := do(t, "POST", apiURL("/api/v1/open/developers/apply"), map[string]interface{}{
		"company_name":   "NormalTestCo",
		"contact_person": "Person",
		"contact_phone":  "13900001111",
		"contact_email":  "normal@test.com",
		"usage_purpose":  "Testing normal scenario",
	}, nil)
	if assertStatus(t, resp, 201, "POST /developers/apply", "normal") {
		var d devApp
		json.Unmarshal(data, &d)
		if d.Status == "pending" {
			record("POST /developers/apply", "normal", true, resp.StatusCode, "")
		}
	}
}

func TestApplyDev_MissingParams(t *testing.T) {
	resp, data := do(t, "POST", apiURL("/api/v1/open/developers/apply"), map[string]interface{}{
		"company_name": "MissingFieldsCo",
		// missing contact_person, contact_phone, contact_email, usage_purpose
	}, nil)
	assertStatus(t, resp, 400, "POST /developers/apply", "missing-params")
	assertErrorCode(t, data, "INVALID_PARAMS", "POST /developers/apply", "missing-params")
}

func TestApplyDev_InvalidBody(t *testing.T) {
	resp, data := do(t, "POST", apiURL("/api/v1/open/developers/apply"),
		"not-valid-json", nil)
	assertStatus(t, resp, 400, "POST /developers/apply", "invalid-body")
	assertErrorCode(t, data, "INVALID_PARAMS", "POST /developers/apply", "invalid-body")
}

// =============================================================================
// Public Endpoints — GET /api/v1/open/developers/{id}
// =============================================================================

func TestGetDev_Normal(t *testing.T) {
	resp, _ := do(t, "GET", apiURLf("/api/v1/open/developers/%d", testDeveloperID), nil, nil)
	assertStatus(t, resp, 200, "GET /developers/{id}", "normal")
}

func TestGetDev_InvalidID(t *testing.T) {
	resp, data := do(t, "GET", apiURL("/api/v1/open/developers/abc"), nil, nil)
	assertStatus(t, resp, 400, "GET /developers/{id}", "invalid-id")
	assertErrorCode(t, data, "INVALID_PARAMS", "GET /developers/{id}", "invalid-id")
}

func TestGetDev_NotFound(t *testing.T) {
	resp, data := do(t, "GET", apiURL("/api/v1/open/developers/99999"), nil, nil)
	assertStatus(t, resp, 404, "GET /developers/{id}", "not-found")
	assertErrorCode(t, data, "NOT_FOUND", "GET /developers/{id}", "not-found")
}

// =============================================================================
// Public Endpoints — GET /api/v1/open/developers/permissions/available
// =============================================================================

func TestPermissionsList_Normal(t *testing.T) {
	resp, data := do(t, "GET", apiURL("/api/v1/open/developers/permissions/available"), nil, nil)
	if assertStatus(t, resp, 200, "GET /permissions/available", "normal") {
		var pl permListResp
		if err := json.Unmarshal(data, &pl); err == nil && len(pl.Permissions) > 0 {
			record("GET /permissions/available", "normal", true, resp.StatusCode, "")
		}
	}
}

// =============================================================================
// Public Endpoints — PUT /api/v1/open/developers/{id}/apply (resubmit)
// =============================================================================

func TestResubmit_NotRejected(t *testing.T) {
	// Our developer is approved, so resubmit should fail.
	resp, data := do(t, "PUT", apiURLf("/api/v1/open/developers/%d/apply", testDeveloperID), map[string]interface{}{
		"company_name":   "ResubmitCo",
		"contact_person": "Person",
		"contact_phone":  "13900002222",
		"contact_email":  "resubmit@test.com",
		"usage_purpose":  "Resubmitting",
	}, nil)
	assertStatus(t, resp, 400, "PUT /developers/{id}/apply", "not-rejected")
	assertErrorCode(t, data, "INVALID_PARAMS", "PUT /developers/{id}/apply", "not-rejected")
}

func TestResubmit_MissingParams(t *testing.T) {
	resp, data := do(t, "PUT", apiURL("/api/v1/open/developers/1/apply"), map[string]interface{}{
		"company_name": "Test",
	}, nil)
	assertStatus(t, resp, 400, "PUT /developers/{id}/apply", "missing-params")
	assertErrorCode(t, data, "INVALID_PARAMS", "PUT /developers/{id}/apply", "missing-params")
}

func TestResubmit_InvalidID(t *testing.T) {
	resp, data := do(t, "PUT", apiURL("/api/v1/open/developers/abc/apply"), map[string]interface{}{
		"company_name":   "Test",
		"contact_person": "P",
		"contact_phone":  "138",
		"contact_email":  "t@t.com",
		"usage_purpose":  "Testing",
	}, nil)
	assertStatus(t, resp, 400, "PUT /developers/{id}/apply", "invalid-id")
	assertErrorCode(t, data, "INVALID_PARAMS", "PUT /developers/{id}/apply", "invalid-id")
}

// =============================================================================
// Public Endpoints — POST /api/v1/open/developers/{id}/request-permissions
// =============================================================================

func TestRequestPerms_EmptyList(t *testing.T) {
	resp, data := do(t, "POST", apiURLf("/api/v1/open/developers/%d/request-permissions", testDeveloperID), map[string]interface{}{
		"permissions": []string{},
	}, nil)
	assertStatus(t, resp, 400, "POST /request-permissions", "empty-list")
	assertErrorCode(t, data, "INVALID_PARAMS", "POST /request-permissions", "empty-list")
}

func TestRequestPerms_MissingBody(t *testing.T) {
	resp, data := do(t, "POST", apiURLf("/api/v1/open/developers/%d/request-permissions", testDeveloperID), nil, nil)
	assertStatus(t, resp, 400, "POST /request-permissions", "missing-body")
	assertErrorCode(t, data, "INVALID_PARAMS", "POST /request-permissions", "missing-body")
}

func TestRequestPerms_InvalidID(t *testing.T) {
	resp, data := do(t, "POST", apiURL("/api/v1/open/developers/abc/request-permissions"), map[string]interface{}{
		"permissions": []string{"members:read"},
	}, nil)
	assertStatus(t, resp, 400, "POST /request-permissions", "invalid-id")
	assertErrorCode(t, data, "INVALID_PARAMS", "POST /request-permissions", "invalid-id")
}

func TestRequestPerms_Normal(t *testing.T) {
	resp, _ := do(t, "POST", apiURLf("/api/v1/open/developers/%d/request-permissions", testDeveloperID), map[string]interface{}{
		"permissions": []string{"members:read"},
	}, nil)
	assertStatus(t, resp, 200, "POST /request-permissions", "normal")
}

// =============================================================================
// Public Endpoints — POST /api/v1/open/token
// =============================================================================

func TestToken_Normal(t *testing.T) {
	resp, data := do(t, "POST", apiURL("/api/v1/open/token"), map[string]interface{}{
		"app_key":    testAppKey,
		"app_secret": testAppSecret,
	}, nil)
	if assertStatus(t, resp, 200, "POST /open/token", "normal") {
		var tp tokenPair
		if json.Unmarshal(data, &tp) == nil && tp.AccessToken != "" && tp.TokenType == "Bearer" {
			record("POST /open/token", "normal", true, resp.StatusCode, "")
		}
	}
}

func TestToken_MissingParams(t *testing.T) {
	resp, data := do(t, "POST", apiURL("/api/v1/open/token"), map[string]interface{}{}, nil)
	assertStatus(t, resp, 401, "POST /open/token", "missing-params")
	assertErrorCode(t, data, "APPKEY_INVALID", "POST /open/token", "missing-params")
}

func TestToken_InvalidCredentials(t *testing.T) {
	resp, data := do(t, "POST", apiURL("/api/v1/open/token"), map[string]interface{}{
		"app_key":    "AK_INVALID_KEY",
		"app_secret": "AS_INVALID_SECRET",
	}, nil)
	assertStatus(t, resp, 401, "POST /open/token", "invalid-credentials")
	assertErrorCode(t, data, "APPKEY_INVALID", "POST /open/token", "invalid-credentials")
}

func TestToken_InvalidBody(t *testing.T) {
	resp, data := do(t, "POST", apiURL("/api/v1/open/token"), "bad-json", nil)
	assertStatus(t, resp, 400, "POST /open/token", "invalid-body")
	assertErrorCode(t, data, "INVALID_PARAMS", "POST /open/token", "invalid-body")
}

// =============================================================================
// Public Endpoints — POST /api/v1/open/token/refresh
// =============================================================================

func TestRefresh_MissingToken(t *testing.T) {
	resp, data := do(t, "POST", apiURL("/api/v1/open/token/refresh"), map[string]interface{}{}, nil)
	assertStatus(t, resp, 400, "POST /open/token/refresh", "missing-token")
	assertErrorCode(t, data, "INVALID_PARAMS", "POST /open/token/refresh", "missing-token")
}

func TestRefresh_InvalidToken(t *testing.T) {
	resp, data := do(t, "POST", apiURL("/api/v1/open/token/refresh"), map[string]interface{}{
		"refresh_token": "invalid-token-string",
	}, nil)
	assertStatus(t, resp, 401, "POST /open/token/refresh", "invalid-token")
	assertErrorCode(t, data, "UNAUTHORIZED", "POST /open/token/refresh", "invalid-token")
}

func TestRefresh_InvalidBody(t *testing.T) {
	resp, data := do(t, "POST", apiURL("/api/v1/open/token/refresh"), "bad", nil)
	assertStatus(t, resp, 400, "POST /open/token/refresh", "invalid-body")
	assertErrorCode(t, data, "INVALID_PARAMS", "POST /open/token/refresh", "invalid-body")
}

// =============================================================================
// Platform Admin Endpoints — No Auth scenarios
// =============================================================================

func TestPending_NoAuth(t *testing.T) {
	resp, data := do(t, "GET", apiURL("/api/v1/open/developers/pending"), nil, nil)
	assertStatus(t, resp, 401, "GET /developers/pending", "no-auth")
	assertErrorCode(t, data, "UNAUTHORIZED", "GET /developers/pending", "no-auth")
}

func TestPending_Normal(t *testing.T) {
	hdr := map[string]string{"Authorization": "Bearer " + adminToken}
	resp, _ := do(t, "GET", apiURL("/api/v1/open/developers/pending"), nil, hdr)
	assertStatus(t, resp, 200, "GET /developers/pending", "normal")
}

func TestListDevs_NoAuth(t *testing.T) {
	resp, data := do(t, "GET", apiURL("/api/v1/open/developers"), nil, nil)
	assertStatus(t, resp, 401, "GET /developers", "no-auth")
	assertErrorCode(t, data, "UNAUTHORIZED", "GET /developers", "no-auth")
}

func TestListDevs_Normal(t *testing.T) {
	hdr := map[string]string{"Authorization": "Bearer " + adminToken}
	resp, _ := do(t, "GET", apiURL("/api/v1/open/developers"), nil, hdr)
	assertStatus(t, resp, 200, "GET /developers", "normal")
}

func TestApprove_NoAuth(t *testing.T) {
	resp, data := do(t, "POST", apiURL("/api/v1/open/developers/1/approve"), nil, nil)
	assertStatus(t, resp, 401, "POST /developers/{id}/approve", "no-auth")
	assertErrorCode(t, data, "UNAUTHORIZED", "POST /developers/{id}/approve", "no-auth")
}

func TestApprove_InvalidID(t *testing.T) {
	hdr := map[string]string{"Authorization": "Bearer " + adminToken}
	resp, data := do(t, "POST", apiURL("/api/v1/open/developers/abc/approve"), nil, hdr)
	assertStatus(t, resp, 400, "POST /developers/{id}/approve", "invalid-id")
	assertErrorCode(t, data, "INVALID_PARAMS", "POST /developers/{id}/approve", "invalid-id")
}

func TestReject_NoAuth(t *testing.T) {
	resp, data := do(t, "POST", apiURL("/api/v1/open/developers/1/reject"), map[string]interface{}{
		"remark": "test",
	}, nil)
	assertStatus(t, resp, 401, "POST /developers/{id}/reject", "no-auth")
	assertErrorCode(t, data, "UNAUTHORIZED", "POST /developers/{id}/reject", "no-auth")
}

func TestReject_InvalidID(t *testing.T) {
	hdr := map[string]string{"Authorization": "Bearer " + adminToken}
	resp, data := do(t, "POST", apiURL("/api/v1/open/developers/abc/reject"), map[string]interface{}{
		"remark": "test",
	}, hdr)
	assertStatus(t, resp, 400, "POST /developers/{id}/reject", "invalid-id")
	assertErrorCode(t, data, "INVALID_PARAMS", "POST /developers/{id}/reject", "invalid-id")
}

func TestUpdatePerms_NoAuth(t *testing.T) {
	resp, data := do(t, "PUT", apiURL("/api/v1/open/developers/1/permissions"), map[string]interface{}{
		"permissions": []string{"shop:read"},
	}, nil)
	assertStatus(t, resp, 401, "PUT /developers/{id}/permissions", "no-auth")
	assertErrorCode(t, data, "UNAUTHORIZED", "PUT /developers/{id}/permissions", "no-auth")
}

func TestUpdatePerms_EmptyList(t *testing.T) {
	hdr := map[string]string{"Authorization": "Bearer " + adminToken}
	resp, data := do(t, "PUT", apiURLf("/api/v1/open/developers/%d/permissions", testDeveloperID), map[string]interface{}{
		"permissions": []string{},
	}, hdr)
	assertStatus(t, resp, 400, "PUT /developers/{id}/permissions", "empty-list")
	assertErrorCode(t, data, "INVALID_PARAMS", "PUT /developers/{id}/permissions", "empty-list")
}

func TestUpdatePerms_InvalidID(t *testing.T) {
	hdr := map[string]string{"Authorization": "Bearer " + adminToken}
	resp, data := do(t, "PUT", apiURL("/api/v1/open/developers/abc/permissions"), map[string]interface{}{
		"permissions": []string{"shop:read"},
	}, hdr)
	assertStatus(t, resp, 400, "PUT /developers/{id}/permissions", "invalid-id")
	assertErrorCode(t, data, "INVALID_PARAMS", "PUT /developers/{id}/permissions", "invalid-id")
}

func TestUpdatePerms_Normal(t *testing.T) {
	hdr := map[string]string{"Authorization": "Bearer " + adminToken}
	resp, _ := do(t, "PUT", apiURLf("/api/v1/open/developers/%d/permissions", testDeveloperID), map[string]interface{}{
		"permissions": []string{"shop:read", "products:read", "services:read", "breeds:read"},
	}, hdr)
	assertStatus(t, resp, 200, "PUT /developers/{id}/permissions", "normal")
}

// =============================================================================
// OpenAPI Auth Endpoint — GET /api/v1/open/ping
// =============================================================================

func TestPing_NoAuth(t *testing.T) {
	resp, data := do(t, "GET", apiURL("/api/v1/open/ping"), nil, nil)
	assertStatus(t, resp, 401, "GET /open/ping", "no-auth")
	assertErrorCode(t, data, "UNAUTHORIZED", "GET /open/ping", "no-auth")
}

func TestPing_InvalidToken(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	resp, data := do(t, "GET", apiURL("/api/v1/open/ping"), nil, map[string]string{
		"Authorization": "Bearer invalid.jwt.token123",
		"X-Timestamp":   ts,
		"X-Nonce":       "test-nonce",
		"X-Signature":   "badsig",
	})
	assertStatus(t, resp, 401, "GET /open/ping", "invalid-token")
	assertErrorCode(t, data, "UNAUTHORIZED", "GET /open/ping", "invalid-token")
}

func TestPing_MissingSignature(t *testing.T) {
	resp, data := do(t, "GET", apiURL("/api/v1/open/ping"), nil, map[string]string{
		"Authorization": "Bearer " + testAccessToken,
	})
	assertStatus(t, resp, 400, "GET /open/ping", "missing-signature")
	assertErrorCode(t, data, "SIGNATURE_MISSING", "GET /open/ping", "missing-signature")
}

func TestPing_BadSignature(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	resp, data := do(t, "GET", apiURL("/api/v1/open/ping"), nil, map[string]string{
		"Authorization": "Bearer " + testAccessToken,
		"X-Timestamp":   ts,
		"X-Nonce":       "test-bad",
		"X-Signature":   "0000000000000000000000000000000000000000000000000000000000000000",
	})
	assertStatus(t, resp, 401, "GET /open/ping", "bad-signature")
	assertErrorCode(t, data, "SIGNATURE_INVALID", "GET /open/ping", "bad-signature")
}

func TestPing_ExpiredTimestamp(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	nonce := "test-expired"
	path := "/api/v1/open/ping"
	sig := computeSig(testAppSecret, ts, nonce, "GET", path)
	resp, data := do(t, "GET", apiURL("/api/v1/open/ping"), nil, map[string]string{
		"Authorization": "Bearer " + testAccessToken,
		"X-Timestamp":   ts,
		"X-Nonce":       nonce,
		"X-Signature":   sig,
	})
	assertStatus(t, resp, 401, "GET /open/ping", "expired-timestamp")
	assertErrorCode(t, data, "SIGNATURE_INVALID", "GET /open/ping", "expired-timestamp")
}

func TestPing_Normal(t *testing.T) {
	path := "/api/v1/open/ping"
	hdr := openAuthHeaders(testAccessToken, testAppSecret, "GET", path)
	resp, _ := do(t, "GET", apiURL(path), nil, hdr)
	assertStatus(t, resp, 200, "GET /open/ping", "normal")
}

// =============================================================================
// OpenAPI Auth Endpoint — GET /api/open/v1/shop/info
// =============================================================================

func TestShopInfo_NoAuth(t *testing.T) {
	resp, data := do(t, "GET", apiURL("/api/open/v1/shop/info"), nil, nil)
	assertStatus(t, resp, 401, "GET /open/v1/shop/info", "no-auth")
	assertErrorCode(t, data, "UNAUTHORIZED", "GET /open/v1/shop/info", "no-auth")
}

func TestShopInfo_InvalidToken(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	resp, data := do(t, "GET", apiURL("/api/open/v1/shop/info"), nil, map[string]string{
		"Authorization": "Bearer invalid.jwt.here",
		"X-Timestamp":   ts,
		"X-Nonce":       "test",
		"X-Signature":   "bad",
	})
	assertStatus(t, resp, 401, "GET /open/v1/shop/info", "invalid-token")
	assertErrorCode(t, data, "UNAUTHORIZED", "GET /open/v1/shop/info", "invalid-token")
}

func TestShopInfo_MissingSignature(t *testing.T) {
	resp, data := do(t, "GET", apiURL("/api/open/v1/shop/info"), nil, map[string]string{
		"Authorization": "Bearer " + testAccessToken,
	})
	assertStatus(t, resp, 400, "GET /open/v1/shop/info", "missing-signature")
	assertErrorCode(t, data, "SIGNATURE_MISSING", "GET /open/v1/shop/info", "missing-signature")
}

func TestShopInfo_BadSignature(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	resp, data := do(t, "GET", apiURL("/api/open/v1/shop/info"), nil, map[string]string{
		"Authorization": "Bearer " + testAccessToken,
		"X-Timestamp":   ts,
		"X-Nonce":       "bad-nonce",
		"X-Signature":   "0000000000000000000000000000000000000000000000000000000000000000",
	})
	assertStatus(t, resp, 401, "GET /open/v1/shop/info", "bad-signature")
	assertErrorCode(t, data, "SIGNATURE_INVALID", "GET /open/v1/shop/info", "bad-signature")
}

func TestShopInfo_Normal(t *testing.T) {
	path := "/api/open/v1/shop/info"
	hdr := openAuthHeaders(testAccessToken, testAppSecret, "GET", path)
	resp, _ := do(t, "GET", apiURL(path), nil, hdr)
	assertStatus(t, resp, 200, "GET /open/v1/shop/info", "normal")
}

// =============================================================================
// OpenAPI Auth Endpoint — GET /api/open/v1/products
// =============================================================================

func TestProductList_NoAuth(t *testing.T) {
	resp, data := do(t, "GET", apiURL("/api/open/v1/products"), nil, nil)
	assertStatus(t, resp, 401, "GET /open/v1/products", "no-auth")
	assertErrorCode(t, data, "UNAUTHORIZED", "GET /open/v1/products", "no-auth")
}

func TestProductList_InvalidToken(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	resp, data := do(t, "GET", apiURL("/api/open/v1/products"), nil, map[string]string{
		"Authorization": "Bearer bad.token.here",
		"X-Timestamp":   ts,
		"X-Nonce":       "test",
		"X-Signature":   "bad",
	})
	assertStatus(t, resp, 401, "GET /open/v1/products", "invalid-token")
	assertErrorCode(t, data, "UNAUTHORIZED", "GET /open/v1/products", "invalid-token")
}

func TestProductList_MissingSignature(t *testing.T) {
	resp, data := do(t, "GET", apiURL("/api/open/v1/products"), nil, map[string]string{
		"Authorization": "Bearer " + testAccessToken,
	})
	assertStatus(t, resp, 400, "GET /open/v1/products", "missing-signature")
	assertErrorCode(t, data, "SIGNATURE_MISSING", "GET /open/v1/products", "missing-signature")
}

func TestProductList_BadSignature(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	resp, data := do(t, "GET", apiURL("/api/open/v1/products"), nil, map[string]string{
		"Authorization": "Bearer " + testAccessToken,
		"X-Timestamp":   ts,
		"X-Nonce":       "bad-prod",
		"X-Signature":   "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	})
	assertStatus(t, resp, 401, "GET /open/v1/products", "bad-signature")
	assertErrorCode(t, data, "SIGNATURE_INVALID", "GET /open/v1/products", "bad-signature")
}

func TestProductList_Normal(t *testing.T) {
	path := "/api/open/v1/products"
	hdr := openAuthHeaders(testAccessToken, testAppSecret, "GET", path)
	resp, _ := do(t, "GET", apiURL(path), nil, hdr)
	assertStatus(t, resp, 200, "GET /open/v1/products", "normal")
}

// =============================================================================
// OpenAPI Auth Endpoint — GET /api/open/v1/products/{id}
// =============================================================================

func TestProductDetail_NoAuth(t *testing.T) {
	resp, data := do(t, "GET", apiURL("/api/open/v1/products/1"), nil, nil)
	assertStatus(t, resp, 401, "GET /open/v1/products/{id}", "no-auth")
	assertErrorCode(t, data, "UNAUTHORIZED", "GET /open/v1/products/{id}", "no-auth")
}

func TestProductDetail_InvalidToken(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	resp, data := do(t, "GET", apiURL("/api/open/v1/products/1"), nil, map[string]string{
		"Authorization": "Bearer dead.token",
		"X-Timestamp":   ts,
		"X-Nonce":       "t",
		"X-Signature":   "x",
	})
	assertStatus(t, resp, 401, "GET /open/v1/products/{id}", "invalid-token")
	assertErrorCode(t, data, "UNAUTHORIZED", "GET /open/v1/products/{id}", "invalid-token")
}

func TestProductDetail_MissingSignature(t *testing.T) {
	resp, data := do(t, "GET", apiURL("/api/open/v1/products/1"), nil, map[string]string{
		"Authorization": "Bearer " + testAccessToken,
	})
	assertStatus(t, resp, 400, "GET /open/v1/products/{id}", "missing-signature")
	assertErrorCode(t, data, "SIGNATURE_MISSING", "GET /open/v1/products/{id}", "missing-signature")
}

func TestProductDetail_BadSignature(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	resp, data := do(t, "GET", apiURL("/api/open/v1/products/1"), nil, map[string]string{
		"Authorization": "Bearer " + testAccessToken,
		"X-Timestamp":   ts,
		"X-Nonce":       "bad-detail",
		"X-Signature":   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	assertStatus(t, resp, 401, "GET /open/v1/products/{id}", "bad-signature")
	assertErrorCode(t, data, "SIGNATURE_INVALID", "GET /open/v1/products/{id}", "bad-signature")
}

func TestProductDetail_InvalidID(t *testing.T) {
	path := "/api/open/v1/products/abc"
	hdr := openAuthHeaders(testAccessToken, testAppSecret, "GET", path)
	resp, data := do(t, "GET", apiURL(path), nil, hdr)
	assertStatus(t, resp, 400, "GET /open/v1/products/{id}", "invalid-id")
	assertErrorCode(t, data, "INVALID_PARAMS", "GET /open/v1/products/{id}", "invalid-id")
}

func TestProductDetail_Normal(t *testing.T) {
	path := "/api/open/v1/products/1"
	hdr := openAuthHeaders(testAccessToken, testAppSecret, "GET", path)
	resp, _ := do(t, "GET", apiURL(path), nil, hdr)
	// 200 if product exists, 404 if not — both are valid non-auth responses
	if resp.StatusCode != 200 && resp.StatusCode != 404 {
		t.Errorf("[GET /open/v1/products/{id}/normal] unexpected status: %d", resp.StatusCode)
		record("GET /open/v1/products/{id}", "normal", false, resp.StatusCode, "unexpected status")
	} else {
		record("GET /open/v1/products/{id}", "normal", true, resp.StatusCode, "")
	}
}

// =============================================================================
// OpenAPI Auth Endpoint — GET /api/open/v1/services
// =============================================================================

func TestServiceList_NoAuth(t *testing.T) {
	resp, data := do(t, "GET", apiURL("/api/open/v1/services"), nil, nil)
	assertStatus(t, resp, 401, "GET /open/v1/services", "no-auth")
	assertErrorCode(t, data, "UNAUTHORIZED", "GET /open/v1/services", "no-auth")
}

func TestServiceList_InvalidToken(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	resp, data := do(t, "GET", apiURL("/api/open/v1/services"), nil, map[string]string{
		"Authorization": "Bearer nope.token",
		"X-Timestamp":   ts,
		"X-Nonce":       "x",
		"X-Signature":   "y",
	})
	assertStatus(t, resp, 401, "GET /open/v1/services", "invalid-token")
	assertErrorCode(t, data, "UNAUTHORIZED", "GET /open/v1/services", "invalid-token")
}

func TestServiceList_MissingSignature(t *testing.T) {
	resp, data := do(t, "GET", apiURL("/api/open/v1/services"), nil, map[string]string{
		"Authorization": "Bearer " + testAccessToken,
	})
	assertStatus(t, resp, 400, "GET /open/v1/services", "missing-signature")
	assertErrorCode(t, data, "SIGNATURE_MISSING", "GET /open/v1/services", "missing-signature")
}

func TestServiceList_BadSignature(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	resp, data := do(t, "GET", apiURL("/api/open/v1/services"), nil, map[string]string{
		"Authorization": "Bearer " + testAccessToken,
		"X-Timestamp":   ts,
		"X-Nonce":       "bad-svc",
		"X-Signature":   "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
	})
	assertStatus(t, resp, 401, "GET /open/v1/services", "bad-signature")
	assertErrorCode(t, data, "SIGNATURE_INVALID", "GET /open/v1/services", "bad-signature")
}

func TestServiceList_Normal(t *testing.T) {
	path := "/api/open/v1/services"
	hdr := openAuthHeaders(testAccessToken, testAppSecret, "GET", path)
	resp, _ := do(t, "GET", apiURL(path), nil, hdr)
	assertStatus(t, resp, 200, "GET /open/v1/services", "normal")
}

// =============================================================================
// OpenAPI Auth Endpoint — GET /api/open/v1/breeds
// =============================================================================

func TestBreeds_NoAuth(t *testing.T) {
	resp, data := do(t, "GET", apiURL("/api/open/v1/breeds"), nil, nil)
	assertStatus(t, resp, 401, "GET /open/v1/breeds", "no-auth")
	assertErrorCode(t, data, "UNAUTHORIZED", "GET /open/v1/breeds", "no-auth")
}

func TestBreeds_InvalidToken(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	resp, data := do(t, "GET", apiURL("/api/open/v1/breeds"), nil, map[string]string{
		"Authorization": "Bearer garbage.token.value",
		"X-Timestamp":   ts,
		"X-Nonce":       "t",
		"X-Signature":   "x",
	})
	assertStatus(t, resp, 401, "GET /open/v1/breeds", "invalid-token")
	assertErrorCode(t, data, "UNAUTHORIZED", "GET /open/v1/breeds", "invalid-token")
}

func TestBreeds_MissingSignature(t *testing.T) {
	resp, data := do(t, "GET", apiURL("/api/open/v1/breeds"), nil, map[string]string{
		"Authorization": "Bearer " + testAccessToken,
	})
	assertStatus(t, resp, 400, "GET /open/v1/breeds", "missing-signature")
	assertErrorCode(t, data, "SIGNATURE_MISSING", "GET /open/v1/breeds", "missing-signature")
}

func TestBreeds_BadSignature(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	resp, data := do(t, "GET", apiURL("/api/open/v1/breeds"), nil, map[string]string{
		"Authorization": "Bearer " + testAccessToken,
		"X-Timestamp":   ts,
		"X-Nonce":       "bad-breeds",
		"X-Signature":   "0000000000000000000000000000000000000000000000000000000000000000",
	})
	assertStatus(t, resp, 401, "GET /open/v1/breeds", "bad-signature")
	assertErrorCode(t, data, "SIGNATURE_INVALID", "GET /open/v1/breeds", "bad-signature")
}

func TestBreeds_Normal(t *testing.T) {
	path := "/api/open/v1/breeds"
	hdr := openAuthHeaders(testAccessToken, testAppSecret, "GET", path)
	resp, _ := do(t, "GET", apiURL(path), nil, hdr)
	assertStatus(t, resp, 200, "GET /open/v1/breeds", "normal")
}

// =============================================================================
// Additional edge cases — Authorization header format
// =============================================================================

func TestOpenAPIAuth_BadAuthFormat(t *testing.T) {
	resp, data := do(t, "GET", apiURL("/api/v1/open/ping"), nil, map[string]string{
		"Authorization": "NoBearerPrefix token",
		"X-Timestamp":   strconv.FormatInt(time.Now().Unix(), 10),
		"X-Nonce":       "test",
		"X-Signature":   "sig",
	})
	assertStatus(t, resp, 401, "GET /open/ping", "bad-auth-format")
	assertErrorCode(t, data, "UNAUTHORIZED", "GET /open/ping", "bad-auth-format")
}

func TestOpenAPIAuth_NoAuthHeader(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	resp, data := do(t, "GET", apiURL("/api/v1/open/ping"), nil, map[string]string{
		"X-Timestamp": ts,
		"X-Nonce":     "test",
		"X-Signature": "sig",
	})
	assertStatus(t, resp, 401, "GET /open/ping", "no-auth-header")
	assertErrorCode(t, data, "UNAUTHORIZED", "GET /open/ping", "no-auth-header")
}

// =============================================================================
// Token exchange — wrong AppKey but correct format
// =============================================================================

func TestToken_WrongAppKey(t *testing.T) {
	resp, data := do(t, "POST", apiURL("/api/v1/open/token"), map[string]interface{}{
		"app_key":    "AK0000000000000000",
		"app_secret": testAppSecret,
	}, nil)
	assertStatus(t, resp, 401, "POST /open/token", "wrong-appkey")
	assertErrorCode(t, data, "APPKEY_INVALID", "POST /open/token", "wrong-appkey")
}

// =============================================================================
// Refresh token — normal flow
// =============================================================================

func TestRefresh_Normal(t *testing.T) {
	// Get a full token pair first.
	resp, data := do(t, "POST", apiURL("/api/v1/open/token"), map[string]interface{}{
		"app_key":    testAppKey,
		"app_secret": testAppSecret,
	}, nil)
	if resp.StatusCode != 200 {
		t.Skip("cannot get token for refresh test")
		return
	}
	var tp tokenPair
	json.Unmarshal(data, &tp)

	resp, _ = do(t, "POST", apiURL("/api/v1/open/token/refresh"), map[string]interface{}{
		"refresh_token": tp.RefreshToken,
	}, nil)
	assertStatus(t, resp, 200, "POST /open/token/refresh", "normal")
}

// =============================================================================
// Edge: wrong signature path vs actual path mismatch
// =============================================================================

func TestPing_QueryStringIgnored(t *testing.T) {
	// Query parameters are not part of r.URL.Path in Go, so signatures should still match.
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "query-string-test"
	sig := computeSig(testAppSecret, ts, nonce, "GET", "/api/v1/open/ping")
	resp, _ := do(t, "GET", apiURL("/api/v1/open/ping?extra=1"), nil, map[string]string{
		"Authorization": "Bearer " + testAccessToken,
		"X-Timestamp":   ts,
		"X-Nonce":       nonce,
		"X-Signature":   sig,
	})
	// Query strings are excluded from path by Go's http.Request.URL.Path, so this should pass.
	assertStatus(t, resp, 200, "GET /open/ping", "query-string-ok")

	// Correct path with matching nonce — should pass.
	nonce2 := "correct-path-nonce"
	sig2 := computeSig(testAppSecret, ts, nonce2, "GET", "/api/v1/open/ping")
	resp2, _ := do(t, "GET", apiURL("/api/v1/open/ping"), nil, map[string]string{
		"Authorization": "Bearer " + testAccessToken,
		"X-Timestamp":   ts,
		"X-Nonce":       nonce2,
		"X-Signature":   sig2,
	})
	assertStatus(t, resp2, 200, "GET /open/ping", "correct-path")
}

// =============================================================================
// Test: Invalid timestamp format (non-numeric)
// =============================================================================

func TestPing_InvalidTimestampFormat(t *testing.T) {
	resp, data := do(t, "GET", apiURL("/api/v1/open/ping"), nil, map[string]string{
		"Authorization": "Bearer " + testAccessToken,
		"X-Timestamp":   "not-a-number",
		"X-Nonce":       "test",
		"X-Signature":   "sig",
	})
	assertStatus(t, resp, 401, "GET /open/ping", "invalid-timestamp")
	assertErrorCode(t, data, "SIGNATURE_INVALID", "GET /open/ping", "invalid-timestamp")
}

// =============================================================================
// Test: List developers with status filter
// =============================================================================

func TestListDevs_FilterByStatus(t *testing.T) {
	hdr := map[string]string{"Authorization": "Bearer " + adminToken}
	resp, _ := do(t, "GET", apiURL("/api/v1/open/developers?status=approved"), nil, hdr)
	assertStatus(t, resp, 200, "GET /developers?status=approved", "normal")
}
