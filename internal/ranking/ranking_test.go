package ranking

import (
	"github.com/malbs/UnoGoBot/internal/groups"
	"testing"
)

func TestScores(t *testing.T) {
	for _, n := range []int{2, 3, 8, 20} {
		previous := Units(1001)
		for position := 1; position <= n; position++ {
			u, err := Score(groups.Updated, n, position)
			if err != nil {
				t.Fatal(err)
			}
			if position == 1 && u != 1000 || position == n && u != 0 || u > previous {
				t.Fatalf("N=%d p=%d score=%d", n, position, u)
			}
			previous = u
			legacy, _ := Score(groups.Legacy, n, position)
			expected := Units(100)
			if position == n {
				expected = 0
			}
			if legacy != expected {
				t.Fatal("legacy bonus changed")
			}
		}
	}
	for i, want := range []Units{1000, 857, 714, 571, 429, 286, 143, 0} {
		got, _ := Score(groups.Updated, 8, i+1)
		if got != want {
			t.Fatalf("8/%d: %d", i+1, got)
		}
	}
	for i, want := range []Units{1000, 500, 0} {
		got, _ := Score(groups.Updated, 3, i+1)
		if got != want {
			t.Fatalf("3/%d: %d", i+1, got)
		}
	}
	// 1000/16=62.5 units must round up to 63, independently of binary float.
	if u, _ := Score(groups.Updated, 17, 16); u != 63 {
		t.Fatal("half-up rounding", u)
	}
	if _, err := Score(groups.Updated, 1, 1); err == nil {
		t.Fatal("undefined N=1 accepted")
	}
	if got := FormatPoints(857); got != "8,6" {
		t.Fatal(got)
	}
}
