package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

// go test -run TestF072 -v -count=1 ./cmd/server/

func TestF072(t *testing.T) {
	baseURL = os.Getenv("TEST_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	fmt.Println("=== F072 SETUP ===")

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
		"company_name":   "F072TestCo",
		"contact_person": "F072 Tester",
		"contact_phone":  "13900139002",
		"contact_email":  "f072@test.example.com",
		"usage_purpose":  "F072 booking API testing",
		"callback_url":   "https://f072.example.com/callback",
	}, nil)
	if resp.StatusCode != 201 {
		t.Fatalf("create dev failed: %d %s", resp.StatusCode, string(data))
	}
	var dev devApp
	json.Unmarshal(data, &dev)
	fmt.Printf("  developer created ID=%d\n", dev.ID)

	// 3. Approve developer with merchant_id=39 (POS测试店2).
	adminHdr := map[string]string{"Authorization": "Bearer " + adminToken}
	resp, data = do(t, "POST", apiURLf("/api/v1/open/developers/%d/approve?merchant_id=39", dev.ID), nil, adminHdr)
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

	// Register a member for booking tests.
	resp, data = do(t, "POST", apiURL("/api/open/v1/members/register"), map[string]interface{}{
		"name":    "李四",
		"phone":   "13800002222",
		"gender":  "M",
		"wechat":  "lisi_wx",
		"address": "北京市海淀区",
		"remark":  "F072测试会员",
	}, authHdr("POST", "/api/open/v1/members/register"))
	if resp.StatusCode != 201 {
		t.Fatalf("register member failed: %d %s", resp.StatusCode, string(data))
	}
	var member struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(data, &member)
	memberID := member.ID
	fmt.Printf("  member registered: ID=%d\n", memberID)

	// Create a pet for the member.
	resp, data = do(t, "POST", apiURLf("/api/open/v1/members/%d/pets", memberID), map[string]interface{}{
		"name":   "小白",
		"breed":  "泰迪",
		"gender": "F",
		"age":    1,
		"weight": "5kg",
		"notes":  "很乖",
	}, authHdr("POST", fmt.Sprintf("/api/open/v1/members/%d/pets", memberID)))
	if resp.StatusCode != 201 {
		t.Fatalf("create pet failed: %d %s", resp.StatusCode, string(data))
	}
	var pet struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(data, &pet)
	petID := pet.ID
	fmt.Printf("  pet created: ID=%d\n", petID)

	// Use merchant_id=39 resources: employee 3 (张三收银), service item 2 (标准洗澡-小型犬).
	// Appointment time: today at 10:00 (within morning shift 09:00-17:00).
	employeeID := int64(3)
	serviceItemID := int64(2)
	now := time.Now()
	apptTime := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, now.Location()).Format(time.RFC3339)
	var bookingID int64

	// =========================================================================
	// Step 1: POST /api/open/v1/bookings — create booking
	// =========================================================================
	t.Run("Step1_CreateBooking", func(t *testing.T) {
		path := "/api/open/v1/bookings"
		endpoint := "POST /api/open/v1/bookings"

		resp, data := do(t, "POST", apiURL(path), map[string]interface{}{
			"member_id":        memberID,
			"pet_id":           petID,
			"service_item_id":  serviceItemID,
			"employee_id":      employeeID,
			"appointment_time": apptTime,
			"remark":           "第一次预约",
		}, authHdr("POST", path))

		if !assertStatus(t, resp, 201, endpoint, "normal") {
			t.Fatalf("create booking failed: %s", string(data))
		}

		var created struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(data, &created)
		bookingID = created.ID
		fmt.Printf("  booking created: ID=%d\n", bookingID)
	})

	// =========================================================================
	// Step 2: GET /api/open/v1/bookings?member_id=X — list member bookings
	// =========================================================================
	t.Run("Step2_ListBookings", func(t *testing.T) {
		path := fmt.Sprintf("/api/open/v1/bookings?member_id=%d", memberID)
		endpoint := "GET /api/open/v1/bookings"

		resp, data := do(t, "GET", apiURL(path), nil, authHdr("GET", "/api/open/v1/bookings"))
		if !assertStatus(t, resp, 200, endpoint, "normal") {
			t.Fatalf("list bookings failed: %s", string(data))
		}

		var result struct {
			Appointments []map[string]interface{} `json:"appointments"`
			Total        int                       `json:"total"`
		}
		json.Unmarshal(data, &result)
		fmt.Printf("  bookings: total=%d, count=%d\n", result.Total, len(result.Appointments))

		if result.Total < 1 {
			t.Errorf("expected at least 1 booking for member %d, got %d", memberID, result.Total)
		}
	})

	// =========================================================================
	// Step 3: PUT /api/open/v1/bookings/{id} — update booking (reschedule)
	// =========================================================================
	t.Run("Step3_UpdateBooking", func(t *testing.T) {
		path := fmt.Sprintf("/api/open/v1/bookings/%d", bookingID)
		endpoint := "PUT /api/open/v1/bookings/{id}"

		// Reschedule to today 14:00 (still within morning shift 09:00-17:00).
		now := time.Now()
		newTime := time.Date(now.Year(), now.Month(), now.Day(), 14, 0, 0, 0, now.Location()).Format(time.RFC3339)
		resp, data := do(t, "PUT", apiURL(path), map[string]interface{}{
			"new_time": newTime,
			"remark":   "改到4小时后",
		}, authHdr("PUT", fmt.Sprintf("/api/open/v1/bookings/%d", bookingID)))

		if !assertStatus(t, resp, 200, endpoint, "normal") {
			t.Fatalf("update booking failed: %s", string(data))
		}

		var updated map[string]interface{}
		json.Unmarshal(data, &updated)
		fmt.Printf("  booking updated: status=%v, remark=%v\n", updated["status"], updated["remark"])
	})

	// =========================================================================
	// Step 4: DELETE /api/open/v1/bookings/{id} — cancel booking
	// =========================================================================
	t.Run("Step4_CancelBooking", func(t *testing.T) {
		path := fmt.Sprintf("/api/open/v1/bookings/%d", bookingID)
		endpoint := "DELETE /api/open/v1/bookings/{id}"

		resp, data := do(t, "DELETE", apiURL(path), map[string]interface{}{
			"reason": "临时有事",
		}, authHdr("DELETE", fmt.Sprintf("/api/open/v1/bookings/%d", bookingID)))

		if !assertStatus(t, resp, 200, endpoint, "normal") {
			t.Fatalf("cancel booking failed: %s", string(data))
		}

		var result map[string]interface{}
		json.Unmarshal(data, &result)
		if msg, ok := result["message"]; ok {
			fmt.Printf("  cancel result: %v\n", msg)
		}
	})

	// =========================================================================
	// Step 5: GET /api/open/v1/technicians/{id}/availability — technician availability
	// =========================================================================
	t.Run("Step5_TechnicianAvailability", func(t *testing.T) {
		today := time.Now().Format("2006-01-02")
		path := fmt.Sprintf("/api/open/v1/technicians/%d/availability?date=%s", employeeID, today)
		endpoint := "GET /api/open/v1/technicians/{id}/availability"

		resp, data := do(t, "GET", apiURL(path), nil, authHdr("GET", fmt.Sprintf("/api/open/v1/technicians/%d/availability", employeeID)))

		if !assertStatus(t, resp, 200, endpoint, "normal") {
			t.Fatalf("availability query failed: %s", string(data))
		}

		var result map[string]interface{}
		json.Unmarshal(data, &result)

		tech, _ := result["technician"].(map[string]interface{})
		shiftType, _ := result["shift_type"].(string)
		slots, _ := result["slots"].([]interface{})

		fmt.Printf("  technician: %v, shift: %s, slots: %d\n", tech["name"], shiftType, len(slots))

		if len(slots) == 0 && shiftType != "rest" {
			t.Errorf("expected non-empty slots for non-rest shift, got %d", len(slots))
		}
	})

	// =========================================================================
	// Security: no auth should fail
	// =========================================================================
	t.Run("Security_NoAuth", func(t *testing.T) {
		path := "/api/open/v1/bookings?member_id=1"
		resp, data := do(t, "GET", apiURL(path), nil, nil)
		assertStatus(t, resp, 401, "GET /bookings", "no-auth")

		var e errorResp
		json.Unmarshal(data, &e)
		fmt.Printf("  no-auth → %d, code=%s\n", resp.StatusCode, e.Code)
	})

	fmt.Printf("\n=== F072 test summary: bookingID=%d, memberID=%d, petID=%d ===\n", bookingID, memberID, petID)
	t.Logf("Tests completed at %s", time.Now().Format(time.RFC3339))
	fmt.Println("done")
}
