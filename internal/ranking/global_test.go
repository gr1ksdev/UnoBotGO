package ranking

import (
	"math"
	"testing"
)

func TestMaskedIDs(t *testing.T) {
	for id, want := range map[int64]string{-100123456789: "••••6789", 12: "••••12", math.MinInt64: "••••5808", 12345: "••••2345"} {
		if got := MaskID(id); got != want {
			t.Fatal(id, got, want)
		}
	}
}
