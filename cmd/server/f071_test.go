//go:build integration

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

// go test -run TestF071 -v -count=1 ./cmd/server/

func TestF071(t *testing.T) {
	baseURL = os.Getenv("TEST_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Setup: admin login → create + approve developer → get open platform token.
	fmt.Println("=== F071 SETUP ===")

	// 1. Login as admin.
	resp, data := do(t, "POST", apiURL("/api/v1/auth/login"),
		map[string]interface{}{"username": "admin", "password": "admin123"}, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("admin login failed: %d %s", resp.StatusCode, string(data))
	}
	var lr loginResp
	json.Unmarshal(data, &lr)
	adminToken = lr.AccessToken
	fmt.Println("  admin login OK")

	// 2. Create developer.
	resp, data = do(t, "POST", apiURL("/api/v1/open/developers/apply"), map[string]interface{}{
		"company_name":   "F071TestCo",
		"contact_person": "F071 Tester",
		"contact_phone":  "13900139000",
		"contact_email":  "f071@test.example.com",
		"usage_purpose":  "F071 member/pet API testing",
		"callback_url":   "https://f071.example.com/callback",
	}, nil)
	if resp.StatusCode != 201 {
		t.Fatalf("create dev failed: %d %s", resp.StatusCode, string(data))
	}
	var dev devApp
	json.Unmarshal(data, &dev)
	fmt.Printf("  developer created ID=%d\n", dev.ID)

	// 3. Approve developer.
	adminHdr := map[string]string{"Authorization": "Bearer " + adminToken}
	resp, data = do(t, "POST", apiURLf("/api/v1/open/developers/%d/approve?merchant_id=1", dev.ID), nil, adminHdr)
	if resp.StatusCode != 200 {
		t.Fatalf("approve dev failed: %d %s", resp.StatusCode, string(data))
	}
	var ar approveResp
	json.Unmarshal(data, &ar)
	appKey := ar.AppKeyResp
	appSecret := ar.AppSecretResp
	fmt.Printf("  developer approved, AppKey=%s\n", appKey)

	// 4. Get open platform token.
	resp, data = do(t, "POST", apiURL("/api/v1/open/token"), map[string]interface{}{
		"app_key": appKey, "app_secret": appSecret,
	}, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("get token failed: %d %s", resp.StatusCode, string(data))
	}
	var tp tokenPair
	json.Unmarshal(data, &tp)
	accessToken := tp.AccessToken
	fmt.Println("  open platform token acquired")

	authHdr := func(method, path string) map[string]string {
		return openAuthHeaders(accessToken, appSecret, method, path)
	}

	var memberID int64

	// =========================================================================
	// Step 1: POST /api/open/v1/members/register
	// =========================================================================
	t.Run("Step1_Register", func(t *testing.T) {
		endpoint := "POST /api/open/v1/members/register"
		path := "/api/open/v1/members/register"

		resp, data := do(t, "POST", apiURL(path), map[string]interface{}{
			"name":    "张三",
			"phone":   "13800001111",
			"gender":  "M",
			"wechat":  "zhangsan_wx",
			"address": "北京市朝阳区",
			"remark":  "测试会员",
		}, authHdr("POST", path))

		if !assertStatus(t, resp, 201, endpoint, "normal") {
			t.Fatalf("register failed: %s", string(data))
		}

		var created struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(data, &created)
		memberID = created.ID
		fmt.Printf("  member registered: ID=%d\n", memberID)
	})

	// =========================================================================
	// Step 2: GET /api/open/v1/members/{id}
	// =========================================================================
	t.Run("Step2_GetMember", func(t *testing.T) {
		path := fmt.Sprintf("/api/open/v1/members/%d", memberID)
		endpoint := "GET /api/open/v1/members/{id}"

		resp, data := do(t, "GET", apiURL(path), nil, authHdr("GET", path))
		if !assertStatus(t, resp, 200, endpoint, "normal") {
			t.Fatalf("get member failed: %s", string(data))
		}

		var respBody map[string]interface{}
		json.Unmarshal(data, &respBody)
		memberObj, ok := respBody["member"]
		if !ok {
			t.Fatalf("no 'member' field in response: %s", string(data))
		}
		m := memberObj.(map[string]interface{})
		if m["balance_cents"] == nil {
			t.Errorf("missing balance_cents")
		}
		if m["points"] == nil {
			t.Errorf("missing points")
		}
		if _, ok := respBody["level"]; !ok {
			t.Log("level field missing (no default level rule configured)")
		}
		fmt.Printf("  member: name=%s, balance_cents=%v, points=%v\n",
			m["name"], m["balance_cents"], m["points"])
	})

	// =========================================================================
	// Step 3: PUT /api/open/v1/members/{id}
	// =========================================================================
	t.Run("Step3_UpdateMember", func(t *testing.T) {
		path := fmt.Sprintf("/api/open/v1/members/%d", memberID)
		endpoint := "PUT /api/open/v1/members/{id}"

		resp, data := do(t, "PUT", apiURL(path), map[string]interface{}{
			"address": "上海市浦东新区",
		}, authHdr("PUT", path))

		if !assertStatus(t, resp, 200, endpoint, "normal") {
			t.Fatalf("update member failed: %s", string(data))
		}

		var updated map[string]interface{}
		json.Unmarshal(data, &updated)
		if addr, ok := updated["address"]; ok {
			fmt.Printf("  address updated to: %v\n", addr)
		}
	})

	// =========================================================================
	// Step 4: POST /api/open/v1/members/{id}/pets
	// =========================================================================
	var petID int64
	t.Run("Step4_CreatePet", func(t *testing.T) {
		path := fmt.Sprintf("/api/open/v1/members/%d/pets", memberID)
		endpoint := "POST /api/open/v1/members/{id}/pets"

		resp, data := do(t, "POST", apiURL(path), map[string]interface{}{
			"name":   "旺财",
			"breed":  "金毛寻回犬",
			"gender": "M",
			"age":    2,
			"weight": "25kg",
			"notes":  "很活泼",
		}, authHdr("POST", path))

		if !assertStatus(t, resp, 201, endpoint, "normal") {
			t.Fatalf("create pet failed: %s", string(data))
		}

		var created map[string]interface{}
		json.Unmarshal(data, &created)
		if pid, ok := created["id"].(float64); ok {
			petID = int64(pid)
		}
		fmt.Printf("  pet created: ID=%d\n", petID)
	})

	// =========================================================================
	// Step 5: GET /api/open/v1/members/{id}/pets
	// =========================================================================
	t.Run("Step5_ListPets", func(t *testing.T) {
		path := fmt.Sprintf("/api/open/v1/members/%d/pets", memberID)
		endpoint := "GET /api/open/v1/members/{id}/pets"

		resp, data := do(t, "GET", apiURL(path), nil, authHdr("GET", path))
		if !assertStatus(t, resp, 200, endpoint, "normal") {
			t.Fatalf("list pets failed: %s", string(data))
		}

		var respBody map[string]interface{}
		json.Unmarshal(data, &respBody)
		pets, _ := respBody["pets"].([]interface{})
		fmt.Printf("  pets count: %d\n", len(pets))

		if len(pets) == 0 {
			t.Errorf("expected at least 1 pet")
		}
	})

	// =========================================================================
	// Security: no auth should fail
	// =========================================================================
	t.Run("Security_NoAuth", func(t *testing.T) {
		path := fmt.Sprintf("/api/open/v1/members/%d", memberID)

		resp, data := do(t, "GET", apiURL(path), nil, nil)
		assertStatus(t, resp, 401, "GET /members/{id}", "no-auth")

		var e errorResp
		json.Unmarshal(data, &e)
		fmt.Printf("  no-auth → %d, code=%s\n", resp.StatusCode, e.Code)
	})

	fmt.Printf("\n=== F071 test summary: memberID=%d, petID=%d ===\n", memberID, petID)

	// Print periodic reminder header for health test.
	t.Logf("Tests completed at %s", time.Now().Format(time.RFC3339))
	fmt.Println("done")
}
