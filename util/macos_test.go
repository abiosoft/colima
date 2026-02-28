package util

import (
	"fmt"
	"testing"
)

type fakeDetector struct {
	v string
	e error
}

func (f fakeDetector) GetChipType() (string, error) { return f.v, f.e }

func TestParseMNumber(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"M3", 3, true},
		{"APPLE M1", 1, true},
		{"M10 Pro", 10, true},
		{"apple m3 pro", 3, true},
		{"Apple M1", 1, true},
		{"No M here", 0, false},
		{"", 0, false},
		{"ARM64", 0, false},
	}

	for _, c := range cases {
		n, ok := parseMNumber(c.in)
		if ok != c.ok || n != c.want {
			t.Fatalf("parseMNumber(%q) = (%d, %v), want (%d, %v)", c.in, n, ok, c.want, c.ok)
		}
	}
}

func TestIsMxOrNewer(t *testing.T) {
	cases := []struct {
		name    string
		chip    string
		chipErr error
		min     int
		want    bool
	}{
		{"m3 satisfies min=3", "Apple M3 Pro", nil, 3, true},
		{"m3 satisfies min=1", "Apple M3 Pro", nil, 1, true},
		{"m3 does not satisfy min=4", "Apple M3 Pro", nil, 4, false},
		{"m1 satisfies min=1", "Apple M1", nil, 1, true},
		{"m1 does not satisfy min=2", "Apple M1", nil, 2, false},
		{"m10 satisfies min=10", "Apple M10", nil, 10, true},
		{"m10 satisfies min=3", "Apple M10", nil, 3, true},
		{"chip fetch error returns false", "", fmt.Errorf("not mac"), 3, false},
		{"non-apple chip returns false", "INTEL CORE I9", nil, 1, false},
		{"empty chip returns false", "", nil, 1, false},
	}

	orig := chipDetector
	defer func() { chipDetector = orig }()

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			chipDetector = fakeDetector{v: c.chip, e: c.chipErr}
			got := IsMxOrNewer(c.min)
			if got != c.want {
				t.Fatalf("IsMxOrNewer(%d) = %v, want %v (chip=%q)", c.min, got, c.want, c.chip)
			}
		})
	}
}
