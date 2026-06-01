package cobo

import (
	"testing"
	"time"
)

func TestGetTripleCouponRate_PhasedRates(t *testing.T) {
	tests := []struct {
		name     string
		nodeType int
		at       time.Time
		want     float64
	}{
		{
			name:     "before first cutoff uses legacy rate",
			nodeType: 1,
			at:       time.Date(2026, 4, 19, 23, 59, 59, 0, asiaShanghaiLoc),
			want:     0.10,
		},
		{
			name:     "first cutoff applies on boundary",
			nodeType: 2,
			at:       time.Date(2026, 4, 20, 0, 0, 0, 0, asiaShanghaiLoc),
			want:     0.105,
		},
		{
			name:     "second cutoff applies on boundary",
			nodeType: 3,
			at:       time.Date(2026, 5, 1, 0, 0, 0, 0, asiaShanghaiLoc),
			want:     0.072,
		},
		{
			name:     "third cutoff stops all grants",
			nodeType: 4,
			at:       time.Date(2026, 5, 11, 0, 0, 0, 0, asiaShanghaiLoc),
			want:     0.0,
		},
		{
			name:     "unknown node type returns zero",
			nodeType: 99,
			at:       time.Date(2026, 4, 19, 12, 0, 0, 0, asiaShanghaiLoc),
			want:     0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := GetTripleCouponRate(tc.nodeType, tc.at)
			if got != tc.want {
				t.Fatalf("GetTripleCouponRate(%d, %s)=%v, want=%v", tc.nodeType, tc.at.Format(time.RFC3339), got, tc.want)
			}
		})
	}
}

func TestGetTripleCouponRate_UsesAbsoluteTimeAcrossTimezones(t *testing.T) {
	utcBoundary := time.Date(2026, 4, 19, 16, 0, 0, 0, time.UTC)

	got := GetTripleCouponRate(1, utcBoundary)
	if got != 0.07 {
		t.Fatalf("GetTripleCouponRate at UTC boundary=%v, want=0.07", got)
	}
}

func TestGetTripleCouponRate_MillisecondBoundary(t *testing.T) {
	before := time.Date(2026, 4, 19, 23, 59, 59, int(999*time.Millisecond), asiaShanghaiLoc)
	at := time.Date(2026, 4, 20, 0, 0, 0, 0, asiaShanghaiLoc)

	gotBefore := GetTripleCouponRate(1, before)
	if gotBefore != 0.10 {
		t.Fatalf("GetTripleCouponRate before boundary=%v, want=0.10", gotBefore)
	}

	gotAt := GetTripleCouponRate(1, at)
	if gotAt != 0.07 {
		t.Fatalf("GetTripleCouponRate at boundary=%v, want=0.07", gotAt)
	}
}
