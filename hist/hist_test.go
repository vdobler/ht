package hist

import (
	"os"
	"testing"
)

func TestRoundUp(t *testing.T) {
	for _, tc := range []struct {
		in, want uint32
	}{
		{19, 20},
		{21, 30},
		{29, 30},
		{31, 50},
		{301, 500},
		{501, 1000},
		{1015, 2000},
		{2005, 3000},
	} {
		if got := roundUp(tc.in); got != tc.want {
			t.Errorf("roundUp(%d)=%d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestRoundDown(t *testing.T) {
	for _, tc := range []struct {
		in, want uint32
	}{
		{12, 10},
		{19, 10},
		{21, 20},
		{29, 20},
		{51, 50},
		{301, 300},
		{501, 500},
		{1001, 1000},
		{2001, 2000},
	} {
		if got := roundDown(tc.in); got != tc.want {
			t.Errorf("roundDown(%d)=%d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestPrintHistograms(t *testing.T) {
	h1 := Histogram{
		Name: "Hist A",
		Data: []uint32{50, 55, 60, 65, 70, 80, 90, 55, 65, 70, 80, 65, 70, 65},
	}

	h2 := Histogram{
		Name: "Hist B",
		Data: []uint32{1000, 1200, 1300, 1000, 1200, 1200, 1000, 110, 2700, 2800, 2700, 2900, 2800},
	}

	h3 := Histogram{
		Name: "Hist C",
		Data: []uint32{90, 80, 90, 75, 85, 90, 100, 150, 90, 140, 85, 250, 230, 240, 230,
			240, 230, 75, 90},
	}

	h4 := Histogram{
		Name: "Hist D",
		Data: []uint32{6, 7, 8, 9, 10, 11, 12, 7, 8, 9, 10, 11, 8, 9, 10, 8, 9},
	}
	hists := []Histogram{
		h1, h2, h3, h4,
	}

	PrintLogHistograms(os.Stdout, hists)
}
