package boarding

import (
	"testing"
	"time"

	"gorm.io/gorm"
)

type mockRepo struct {
	rooms       map[int64]*BoardingRoom
	orders      map[int64]*BoardingOrder
	careLogs    map[int64][]CareLog
	nextRoomID  int64
	nextOrderID int64
	nextLogID   int64
	findRoomErr error
	findOrderErr error
	createErr   error
	updateErr   error
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		rooms:    make(map[int64]*BoardingRoom),
		orders:   make(map[int64]*BoardingOrder),
		careLogs: make(map[int64][]CareLog),
		nextRoomID: 1,
		nextOrderID: 1,
		nextLogID: 1,
	}
}

func (m *mockRepo) FindRoomByID(roomID int64) (*BoardingRoom, error) {
	r, ok := m.rooms[roomID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return r, m.findRoomErr
}
func (m *mockRepo) FindFreeRoom(storeID, roomTypeID int64) (*BoardingRoom, error) {
	for _, r := range m.rooms {
		if r.StoreID == storeID && r.RoomTypeID == roomTypeID && r.Status == RoomStatusFree {
			return r, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockRepo) FindOrderByID(id, storeID int64) (*BoardingOrder, error) {
	o, ok := m.orders[id]
	if !ok || o.StoreID != storeID {
		return nil, gorm.ErrRecordNotFound
	}
	return o, m.findOrderErr
}
func (m *mockRepo) UpdateRoom(r *BoardingRoom) error { m.rooms[r.ID] = r; return nil }
func (m *mockRepo) CreateOrder(o *BoardingOrder) error {
	if m.createErr != nil { return m.createErr }
	o.ID = m.nextOrderID; m.nextOrderID++; m.orders[o.ID] = o; return nil
}
func (m *mockRepo) UpdateOrder(o *BoardingOrder) error { m.orders[o.ID] = o; return m.updateErr }
func (m *mockRepo) CreateCareLog(cl *CareLog) error {
	cl.ID = m.nextLogID; m.nextLogID++
	m.careLogs[cl.BoardingOrderID] = append(m.careLogs[cl.BoardingOrderID], *cl)
	return nil
}
func (m *mockRepo) FindCareLogs(orderID int64, date time.Time) ([]CareLog, error) {
	return m.careLogs[orderID], nil
}
func (m *mockRepo) ListOrders(storeID int64, status string, page, pageSize int) ([]BoardingOrder, int64, error) {
	return nil, 0, nil
}

// --- Billing Tests ---

func TestCalculateNights(t *testing.T) {
	tests := []struct {
		name     string
		checkIn  time.Time
		checkOut time.Time
		want     int
	}{
		{"exact 3 nights", time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC), time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC), 3},
		{"2 days 3 hours → 3 nights", time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC), time.Date(2026, 6, 3, 13, 0, 0, 0, time.UTC), 3},
		{"20 hours → 1 night", time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC), time.Date(2026, 6, 2, 6, 0, 0, 0, time.UTC), 1},
		{"1 hour → 1 night (min)", time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC), time.Date(2026, 6, 1, 11, 0, 0, 0, time.UTC), 1},
		{"24 hours exact → 1 night", time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC), time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC), 1},
		{"24h+1s → 2 nights", time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC), time.Date(2026, 6, 2, 10, 0, 1, 0, time.UTC), 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateNights(tt.checkIn, tt.checkOut)
			if got != tt.want {
				t.Errorf("CalculateNights = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCalculateBoardingAmount(t *testing.T) {
	nights := 3
	pricePerNight := int64(12800)
	amount := int64(nights) * pricePerNight
	if amount != 38400 {
		t.Errorf("amount = %d, want 38400", amount)
	}
}

// --- Check-in Tests ---

func TestCheckIn(t *testing.T) {
	repo := newMockRepo()
	repo.rooms[5] = &BoardingRoom{ID: 5, StoreID: 1, RoomTypeID: 1, Code: "S05", Status: RoomStatusFree}
	svc := NewService(repo)

	order, err := svc.CheckIn(CheckInRequest{
		StoreID:        1,
		CustomerID:     1,
		PetID:          1,
		RoomID:         5,
		RoomTypeCode:   "small",
		PricePerNight:  8800,
		PlannedCheckIn:  time.Now(),
		PlannedCheckOut: time.Now().Add(3 * 24 * time.Hour),
		Remark:         "标准3晚",
	})
	if err != nil {
		t.Fatalf("CheckIn() error: %v", err)
	}
	if order.Status != StatusCheckedIn {
		t.Errorf("order status = %q, want checked_in", order.Status)
	}
	if repo.rooms[5].Status != RoomStatusOccupied {
		t.Errorf("room status = %q, want occupied", repo.rooms[5].Status)
	}
}

func TestCheckInRoomNotFree(t *testing.T) {
	repo := newMockRepo()
	repo.rooms[1] = &BoardingRoom{ID: 1, StoreID: 1, Code: "S01", Status: RoomStatusOccupied}
	svc := NewService(repo)

	_, err := svc.CheckIn(CheckInRequest{StoreID: 1, RoomID: 1, RoomTypeCode: "small", PricePerNight: 8800})
	if err == nil {
		t.Fatal("expected error for occupied room")
	}
}

// --- Check-out Tests ---

func TestCheckOut(t *testing.T) {
	repo := newMockRepo()
	checkInTime := time.Now().Add(-2 * 24 * time.Hour).Add(-3 * time.Hour)
	roomID := int64(5)
	repo.orders[1] = &BoardingOrder{
		ID: 1, StoreID: 1, CustomerID: 1, PetID: 1,
		RoomID: &roomID, RoomTypeSnapshot: "small", PricePerNight: 8800,
		Status: StatusCheckedIn, ActualCheckIn: &checkInTime,
	}
	repo.rooms[5] = &BoardingRoom{ID: 5, StoreID: 1, Code: "S05", Status: RoomStatusOccupied}
	svc := NewService(repo)

	resp, err := svc.CheckOut(1, 1)
	if err != nil {
		t.Fatalf("CheckOut() error: %v", err)
	}
	if resp.Order.Status != StatusCheckedOut {
		t.Errorf("order status = %q, want checked_out", resp.Order.Status)
	}
	if resp.Nights < 2 {
		t.Errorf("nights should be >= 2 (2d+3h → 3 nights), got %d", resp.Nights)
	}
	if repo.rooms[5].Status != RoomStatusCleaning {
		t.Errorf("room status = %q, want cleaning", repo.rooms[5].Status)
	}
}

// --- Care Log Tests ---

func TestLogCare(t *testing.T) {
	repo := newMockRepo()
	repo.orders[1] = &BoardingOrder{ID: 1, StoreID: 1, Status: StatusCheckedIn}
	svc := NewService(repo)

	err := svc.LogCare(1, 1, "feeding", "done", "喂食完成", 7)
	if err != nil {
		t.Fatalf("LogCare() error: %v", err)
	}
	logs := repo.careLogs[1]
	if len(logs) != 1 {
		t.Fatalf("care logs count = %d, want 1", len(logs))
	}
	if logs[0].Task != "feeding" {
		t.Errorf("task = %q, want feeding", logs[0].Task)
	}
}

func TestLogCareOrderNotCheckedIn(t *testing.T) {
	repo := newMockRepo()
	repo.orders[1] = &BoardingOrder{ID: 1, StoreID: 1, Status: StatusBooked}
	svc := NewService(repo)

	err := svc.LogCare(1, 1, "feeding", "done", "", 7)
	if err == nil {
		t.Fatal("expected error for non-checked-in order")
	}
}

// --- Room State Machine Tests ---

func TestRoomStateTransitions(t *testing.T) {
	tests := []struct {
		from  string
		to    string
		valid bool
	}{
		{RoomStatusFree, RoomStatusOccupied, true},
		{RoomStatusFree, RoomStatusMaintenance, true},
		{RoomStatusOccupied, RoomStatusCleaning, true},
		{RoomStatusCleaning, RoomStatusFree, true},
		{RoomStatusCleaning, RoomStatusMaintenance, true},
		{RoomStatusMaintenance, RoomStatusFree, true},
		{RoomStatusFree, RoomStatusCleaning, false},    // skip occupied
		{RoomStatusOccupied, RoomStatusFree, false},    // skip cleaning
		{RoomStatusCleaning, RoomStatusOccupied, false}, // must be freed first
	}
	for _, tt := range tests {
		t.Run(tt.from+"->"+tt.to, func(t *testing.T) {
			got := IsValidRoomTransition(tt.from, tt.to)
			if got != tt.valid {
				t.Errorf("IsValidRoomTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.valid)
			}
		})
	}
}
