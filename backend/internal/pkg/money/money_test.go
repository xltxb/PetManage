package money

import (
	"testing"
)

func TestToYuan(t *testing.T) {
	tests := []struct {
		name   string
		cents  int64
		expect string
	}{
		{"zero", 0, "0.00"},
		{"one yuan", 100, "1.00"},
		{"268 yuan", 26800, "268.00"},
		{"negative", -100, "-1.00"},
		{"fifty cents", 50, "0.50"},
		{"round number", 10000, "100.00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToYuan(tt.cents)
			if got != tt.expect {
				t.Errorf("ToYuan(%d) = %q, want %q", tt.cents, got, tt.expect)
			}
		})
	}
}

func TestToCents(t *testing.T) {
	tests := []struct {
		name    string
		yuan    string
		expect  int64
		wantErr bool
	}{
		{"zero", "0.00", 0, false},
		{"one yuan", "1.00", 100, false},
		{"268 yuan", "268.00", 26800, false},
		{"with decimal", "12.50", 1250, false},
		{"invalid format", "abc", 0, true},
		{"empty", "", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToCents(tt.yuan)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ToCents(%q) expected error, got nil", tt.yuan)
				}
				return
			}
			if err != nil {
				t.Errorf("ToCents(%q) unexpected error: %v", tt.yuan, err)
			}
			if got != tt.expect {
				t.Errorf("ToCents(%q) = %d, want %d", tt.yuan, got, tt.expect)
			}
		})
	}
}

func TestToYuanInt(t *testing.T) {
	tests := []struct {
		name   string
		cents  int64
		expect string
	}{
		{"even", 200, "2.00"},
		{"round up", 201, "2.01"},
		{"round down", 199, "1.99"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToYuan(tt.cents)
			if got != tt.expect {
				t.Errorf("ToYuan(%d) = %q, want %q", tt.cents, got, tt.expect)
			}
		})
	}
}
