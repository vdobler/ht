// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package loghist

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

func TestLogHist(t *testing.T) {
	for _, bits := range []uint{2, 3, 4, 5, 6, 7, 8, 9, 10} {
		for _, max := range []int{3, 30, 300, 3000, 30000} {
			h := New(bits, max)
			n := 1 << bits

			// Checking max. TODO: corner cases
			if h.max < max {
				t.Errorf("bits=%d Max=%d, want>=%d", bits, h.max, max)
			}

			// Check buckets being continuous and of proper size.
			lastBucket := 0
			bs := 1
			blockstart := 0
			for v := 1; v <= h.max; v++ {
				bucket := h.bucket(v)

				// Bucket increases continuousely and monotonic.
				if bucket != lastBucket && bucket != lastBucket+1 {
					t.Errorf("bits=%d max=%d v=%d: bucket jumped from %d to %d",
						bits, max, v, lastBucket, bucket)
				}

				// Bucket steps are equally spaced in v.
				if (v-blockstart)%bs == 0 {
					if bucket != lastBucket+1 {
						t.Errorf("bits=%d max=%d v=%d: bucket %d, want %d",
							bits, max, v, bucket, lastBucket+1)
					}
				} else {
					if bucket != lastBucket {
						t.Errorf("bits=%d max=%d v=%d: bucket %d, want %d",
							bits, max, v, bucket, lastBucket)
					}
				}

				if blockstart+n*bs == v {
					// Start the next block.
					bs *= 2
					blockstart = v
				}

				lastBucket = bucket
			}

			// Check cover.
			lb := h.bucket(h.max)
			lastA := 0
			for bucket := 0; bucket <= lb; bucket++ {
				a, b := h.cover(bucket)
				if a != lastA {
					t.Errorf("bits=%d max=%d bucket=%d: a=%d want %d",
						bits, max, bucket, a, lastA)
				}
				for v := a; v < b; v++ {
					b := h.bucket(v)
					if b != bucket {
						t.Errorf("bits=%d max=%d bucket=%d: bucket(%d)=%d, want %d",
							bits, max, bucket, v, b, bucket)
					}
				}
				lastA = b
			}
		}
	}
}

func TestParameters(t *testing.T) {
	for i, tc := range []struct {
		bits, max, rmax int
	}{
		{8, 100, 101},
		{4, 100, 104},
		{3, 1000, 1016},
	} {
		h := New(uint(tc.bits), tc.max)
		if gb, gm := h.Parameters(); gb != tc.bits || gm != tc.rmax {
			t.Errorf("%d: (%d,%d), got %d,%d want %d,%d",
				i, tc.bits, tc.max, gb, gm, tc.bits, tc.rmax)
		}
	}

}

func TestAverage(t *testing.T) {
	type avgt struct {
		v    []int
		want int
	}
	for i, tc := range []avgt{
		{[]int{15}, 15},
		{[]int{10, 20}, 15},
		{[]int{40, 41}, 40},
		{[]int{40, 41, 40}, 40},
		{[]int{40, 41, 40, 20}, 35},
		{[]int{128}, 129},
		{[]int{128, 129}, 129},
		{[]int{128, 129, 130}, 129},
		{[]int{128, 129, 130, 131}, 129}, // all are covered by bucket 72 [128,132)
	} {
		h := New(5, 200)
		for _, v := range tc.v {
			h.Add(v)
		}
		if got := h.Average(); got != tc.want {
			t.Errorf("%d: Average(%v)=%d, want %d", i, tc.v, got, tc.want)
		}
	}
}

func TestAverage2(t *testing.T) {
	h := New(7, 1000)
	for i := 0; i <= 1000; i++ {
		h.Add(i)
	}
	if got := h.Average(); got != 500 {
		t.Errorf("Average(0..1000)=%d, want 500", got)
	}

	h = New(7, 1000)
	for i := 0; i < 100000; i++ {
		v := int(rand.NormFloat64()*100 + 500)
		h.Add(v)
	}
	if got := h.Average(); got < 495 || got > 505 {
		t.Errorf("Average=%d, want 500", got)
	}
}

func TestPercentils(t *testing.T) {
	h := New(7, 1000)

	min, max := 99999999, -999999999
	for i := 0; i < 100000; i++ {
		v := int(rand.NormFloat64()*100 + 300)
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		h.Add(v)
	}

	for i := 0; i < 100000; i++ {
		v := int(rand.NormFloat64()*100 + 700)
		h.Add(v)
	}

	ps := []float64{0.25, 0.5, 0.75}
	u := h.Quantiles(ps)
	if len(u) != 3 {
		t.Errorf("Got %d values back", len(u))
	}

	if u[0] < 295 || u[0] > 305 ||
		u[1] < 495 || u[1] > 505 ||
		u[2] < 695 || u[2] > 705 {
		t.Errorf("Either math/rand is buggy or we had exceptional "+
			"bad luck or something is brocken:\nu = %v", u)
	}

	fmt.Println(h)
}

func TestFivenum(t *testing.T) {
	h := New(7, 100)

	for i := 26; i <= 74; i++ {
		h.Add(i)
		h.Add(i)
		h.Add(i)
		h.Add(i)
		h.Add(i)
	}

	// According to R:
	//     > quantile(seq(26,74), c(0, 0.25, 0.5, 0.75, 1));
	//       0%  25%  50%  75% 100%
	//       26   38   50   62   74
	ps := []float64{0, 0.25, 0.5, 0.75, 1}
	got := h.Quantiles(ps)
	want := []int{26, 38, 50, 62, 74}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Wrong percentils:\ngot  = %v\nwant = %v", got, want)
	}
}

func TestCoarsePercentils(t *testing.T) {
	h := New(4, 20000) // only 4bit resolution

	for i := 0; i <= 20000; i++ {
		h.Add(i)
		h.Add(i)
		h.Add(i)
		h.Add(i)
		h.Add(i)
	}

	// According to R:
	//    0%    5%   25%   50%   75%   95%  100%
	//     0  1000  5000 10000 15000 19000 20000
	// but our 4-bit histogram fills the maximum of 20000 into a large
	// bucket [19440,20464) and (properly) reports the maximal value
	// in this bucket as the 1.0 quantile.
	ps := []float64{0, 0.05, 0.25, 0.5, 0.75, 0.95, 1}
	got := h.Quantiles(ps)
	want := []int{0, 1000, 5000, 10000, 15000, 19000, 20463}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Wrong percentils:\ngot  = %v\nwant = %v", got, want)
	}
}

func BenchmarkHist(b *testing.B) {
	for i := 0; i < b.N; i++ {
		h := New(7, 100000)
		for v := 0; v < 100000; v++ {
			h.Add(v)
		}
	}
}
