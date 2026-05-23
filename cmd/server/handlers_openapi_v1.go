package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/xltxb/PetManage/internal/appointment"
	"github.com/xltxb/PetManage/internal/dictionary"
	"github.com/xltxb/PetManage/internal/employee"
	"github.com/xltxb/PetManage/internal/member"
	"github.com/xltxb/PetManage/internal/memberlevel"
	"github.com/xltxb/PetManage/internal/merchant"
	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/internal/openplatform"
	"github.com/xltxb/PetManage/internal/pet"
	"github.com/xltxb/PetManage/internal/product"
	"github.com/xltxb/PetManage/internal/schedule"
	"github.com/xltxb/PetManage/internal/servicemgmt"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// getOpenAPIMerchantID extracts the merchant_id from open platform auth claims.
// Returns 0 if the developer has no associated merchant.
func getOpenAPIMerchantID(r *http.Request) int64 {
	claims := middleware.OpenDevClaimsFromContext(r.Context())
	if claims == nil {
		return 0
	}
	return claims.MerchantID
}

// GET /api/open/v1/shop/info — returns store name, address, business hours, logo.
func makeOpenShopInfoHandler(ms *merchant.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		settings, err := ms.GetShopSettings(r.Context(), merchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get shop info", err))
			return
		}

		// Build logo full URL if logo is set.
		logoURL := settings.LogoURL
		if logoURL != "" && r.Host != "" {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			logoURL = scheme + "://" + r.Host + logoURL
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":           settings.Name,
			"address":        settings.Address,
			"contact_phone":  settings.ContactPhone,
			"contact_email":  settings.ContactEmail,
			"business_hours": settings.BusinessHours,
			"logo_url":       logoURL,
			"notice":         settings.Notice,
		})
	}
}

// GET /api/open/v1/products — product list with pagination and optional category filter.
func makeOpenProductListHandler(ps *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		q := r.URL.Query()

		// Category filter.
		var categoryID *int64
		if catStr := q.Get("category_id"); catStr != "" {
			if v, err := strconv.ParseInt(catStr, 10, 64); err == nil && v > 0 {
				categoryID = &v
			}
		}

		page := 1
		pageSize := 20
		if p := q.Get("page"); p != "" {
			if v, err := strconv.Atoi(p); err == nil && v > 0 {
				page = v
			}
		}
		if ps := q.Get("page_size"); ps != "" {
			if v, err := strconv.Atoi(ps); err == nil && v > 0 && v <= 100 {
				pageSize = v
			}
		}

		params := product.ListParams{
			Status:   "active",
			Page:     page,
			PageSize: pageSize,
		}
		_ = categoryID // category_id filter not built into ListParams; reserved for future filtering

		result, err := ps.List(r.Context(), merchantID, params)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list products", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// GET /api/open/v1/products/{id} — product detail including SKUs.
func makeOpenProductDetailHandler(ps *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		productID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || productID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid product id"))
			return
		}

		detail, err := ps.GetByIDWithSKUs(r.Context(), productID, merchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get product", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(detail)
	}
}

// GET /api/open/v1/services — service item list with price and duration.
func makeOpenServiceListHandler(svc *servicemgmt.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		q := r.URL.Query()
		page := 1
		pageSize := 20
		if p := q.Get("page"); p != "" {
			if v, err := strconv.Atoi(p); err == nil && v > 0 {
				page = v
			}
		}
		if ps := q.Get("page_size"); ps != "" {
			if v, err := strconv.Atoi(ps); err == nil && v > 0 && v <= 100 {
				pageSize = v
			}
		}

		params := servicemgmt.ListItemsParams{
			Status:   "active",
			Page:     page,
			PageSize: pageSize,
		}
		if catStr := q.Get("category_id"); catStr != "" {
			if v, err := strconv.ParseInt(catStr, 10, 64); err == nil && v > 0 {
				params.CategoryID = &v
			}
		}

		result, err := svc.ListItems(r.Context(), merchantID, params)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list services", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// GET /api/open/v1/breeds — pet breed list, optionally filtered by type.
func makeOpenBreedsHandler(ds *dictionary.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			// Breeds are platform-level data, merchant association not strictly required.
		}

		petType := r.URL.Query().Get("type")

		result, err := ds.ListBreeds(r.Context(), petType, merchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list breeds", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// OpenAPIService holds references needed by the open API handlers.
type OpenAPIService struct {
	Merchant     *merchant.Service
	Product      *product.Service
	ServiceMgmt  *servicemgmt.Service
	Dict         *dictionary.Service
	Token        *openplatform.TokenService
	Member       *member.Service
	MemberLevel  *memberlevel.Service
	Pet          *pet.Service
}

// =============================================================================
// F071: Member & Pet API handlers
// =============================================================================

// POST /api/open/v1/members/register — register a new member.
func makeOpenMemberRegisterHandler(ms *member.Service, mls *memberlevel.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		var req member.CreateMemberRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body: "+err.Error()))
			return
		}

		m, err := ms.Create(r.Context(), merchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to register member", err))
			return
		}

		// Assign default level if configured.
		_ = mls.SetDefaultLevel(r.Context(), merchantID, m.ID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(m)
	}
}

// GET /api/open/v1/members/{id} — query member info including level, balance, points.
func makeOpenMemberGetHandler(ms *member.Service, mls *memberlevel.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		memberID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		m, err := ms.GetByID(r.Context(), memberID, merchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get member", err))
			return
		}

		levelInfo, _ := mls.GetMemberLevel(r.Context(), merchantID, memberID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"member": m,
			"level":  levelInfo,
		})
	}
}

// PUT /api/open/v1/members/{id} — update member information.
func makeOpenMemberUpdateHandler(ms *member.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		memberID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		var req member.UpdateMemberRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body: "+err.Error()))
			return
		}

		m, err := ms.Update(r.Context(), memberID, merchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update member", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m)
	}
}

// POST /api/open/v1/members/{id}/pets — add a pet profile to a member.
func makeOpenPetCreateHandler(ps *pet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		memberID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		var req pet.CreatePetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body: "+err.Error()))
			return
		}

		p, err := ps.Create(r.Context(), merchantID, memberID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create pet", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(p)
	}
}

// GET /api/open/v1/members/{id}/pets — list pets for a member.
func makeOpenPetListHandler(ps *pet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		memberID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		pets, err := ps.ListByMember(r.Context(), merchantID, memberID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list pets", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pets": pets,
		})
	}
}

// =============================================================================
// F072: Booking & Technician Availability API handlers
// =============================================================================

// POST /api/open/v1/bookings — create a new appointment.
func makeOpenBookingCreateHandler(as *appointment.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		var req appointment.CreateAppointmentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body: "+err.Error()))
			return
		}

		apt, err := as.Create(r.Context(), merchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create booking", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(apt)
	}
}

// GET /api/open/v1/bookings — list bookings for a member.
func makeOpenBookingListHandler(as *appointment.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		q := r.URL.Query()
		memberIDStr := q.Get("member_id")
		if memberIDStr == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("member_id is required"))
			return
		}
		memberID, err := strconv.ParseInt(memberIDStr, 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member_id"))
			return
		}

		page := 1
		pageSize := 20
		if p := q.Get("page"); p != "" {
			if v, err := strconv.Atoi(p); err == nil && v > 0 {
				page = v
			}
		}
		if ps := q.Get("page_size"); ps != "" {
			if v, err := strconv.Atoi(ps); err == nil && v > 0 && v <= 100 {
				pageSize = v
			}
		}

		params := appointment.ListParams{
			MemberID: memberID,
			Status:   q.Get("status"),
			Page:     page,
			PageSize: pageSize,
		}

		result, err := as.List(r.Context(), merchantID, params)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list bookings", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// PUT /api/open/v1/bookings/{id} — update booking time and/or remark.
func makeOpenBookingUpdateHandler(as *appointment.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		bookingID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || bookingID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid booking id"))
			return
		}

		var body struct {
			NewTime string `json:"new_time"`
			Remark  string `json:"remark"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body: "+err.Error()))
			return
		}

		// If new_time is provided, reschedule.
		if body.NewTime != "" {
			newTime, err := time.Parse(time.RFC3339, body.NewTime)
			if err != nil {
				apperrors.WriteError(w, r, apperrors.NewValidationError("invalid new_time format, use RFC3339"))
				return
			}
			if newTime.Before(time.Now()) {
				apperrors.WriteError(w, r, apperrors.NewValidationError("new_time must be in the future"))
				return
			}

			_, err = as.Reschedule(r.Context(), merchantID, bookingID, appointment.RescheduleRequest{
				NewTime: body.NewTime,
				Reason:  body.Remark,
			})
			if err != nil {
				if appErr, ok := err.(*apperrors.AppError); ok {
					apperrors.WriteError(w, r, appErr)
					return
				}
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to reschedule booking", err))
				return
			}
		}

		// Return the updated booking.
		apt, err := as.GetByID(r.Context(), merchantID, bookingID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get updated booking", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(apt)
	}
}

// DELETE /api/open/v1/bookings/{id} — cancel a booking.
func makeOpenBookingCancelHandler(as *appointment.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		bookingID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || bookingID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid booking id"))
			return
		}

		var body struct {
			Reason string `json:"reason"`
		}
		json.NewDecoder(r.Body).Decode(&body)

		apt, err := as.Cancel(r.Context(), merchantID, bookingID, appointment.CancelRequest{
			Reason: body.Reason,
		})
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to cancel booking", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "booking cancelled",
			"booking": apt,
		})
	}
}

// GET /api/open/v1/technicians/{id}/availability — query technician availability for a given date.
func makeOpenTechnicianAvailabilityHandler(es *employee.Service, ss *schedule.Service, as *appointment.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantID := getOpenAPIMerchantID(r)
		if merchantID == 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("no merchant associated with this developer"))
			return
		}

		techID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || techID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid technician id"))
			return
		}

		// Verify technician exists and is active.
		emp, err := es.GetByID(r.Context(), merchantID, techID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get technician", err))
			return
		}
		if emp.Status != "active" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("technician is not active"))
			return
		}

		// Date parameter, default to today.
		dateStr := r.URL.Query().Get("date")
		if dateStr == "" {
			dateStr = time.Now().Format("2006-01-02")
		}

		// Get the technician's schedule for this date.
		schedules, err := ss.List(r.Context(), merchantID, techID, dateStr, dateStr)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get schedule", err))
			return
		}

		shiftType := "rest"
		shiftHours := map[string]string{"start": "", "end": ""}
		if len(schedules) > 0 && schedules[0].ShiftType != "rest" {
			shiftType = schedules[0].ShiftType
			hours, ok := schedule.ShiftHours[shiftType]
			if ok {
				shiftHours["start"] = hours.Start
				shiftHours["end"] = hours.End
			}
		}

		// Get booked slots for this technician on this date.
		bookedSlots, err := as.GetBookedSlots(r.Context(), merchantID, techID, dateStr)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get booked slots", err))
			return
		}

		// Compute available time slots in 30-minute increments.
		type Slot struct {
			Time     string `json:"time"`
			Booked   bool   `json:"booked"`
			BookingID int64  `json:"booking_id,omitempty"`
		}

		var slots []Slot
		if shiftType != "rest" {
			start, _ := time.Parse("15:04", shiftHours["start"])
			end, _ := time.Parse("15:04", shiftHours["end"])

			// Build a set of booked times for quick lookup.
			bookedSet := make(map[string]BookedSlotInfo)
			for _, bs := range bookedSlots {
				t := bs.AppointmentTime.Format("15:04")
				bookedSet[t] = BookedSlotInfo{Booked: true, BookingID: bs.AppointmentID}
			}

			for t := start; t.Before(end); t = t.Add(30 * time.Minute) {
				timeStr := t.Format("15:04")
				if info, booked := bookedSet[timeStr]; booked {
					slots = append(slots, Slot{Time: timeStr, Booked: true, BookingID: info.BookingID})
				} else {
					slots = append(slots, Slot{Time: timeStr, Booked: false})
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"technician": map[string]interface{}{
				"id":       emp.ID,
				"name":     emp.Name,
				"position": emp.Position,
			},
			"date":       dateStr,
			"shift_type": shiftType,
			"shift_hours": map[string]string{
				"start": shiftHours["start"],
				"end":   shiftHours["end"],
			},
			"booked_slots": bookedSlots,
			"slots":        slots,
		})
	}
}

// BookedSlotInfo is used for the availability map lookup.
type BookedSlotInfo struct {
	Booked    bool
	BookingID int64
}
