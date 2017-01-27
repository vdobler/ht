// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package loghist provides integer histograms with exponentially growing bucket sizes.
package loghist

import (
	"fmt"
	"strings"
)

// Hist is a histogram of non-negative integer values whose bin size
// increases exponentially and covers the intervall [0,Max].
// The bins are grouped into blocks of 1<<bist equal-sized bins. The first block has
// a bin size of 1; in each consecutive block the bin size is doubled.
// The resolution of the histogram is 1/(1<<bits).
type Hist struct {
	Overflow  int // Number of added values > Max
	Underflow int // Number of added values < 0

	n     int   // Number of equal sized bins before binsize doubles. A power of two.
	max   int   // Max is the last value which can be counted.
	count []int // Count contains the counts for each bucket.

}

// New returns a new Hist capable of storing values from 0 to (at least) max
// with a resolution of bits. N will be 1<<bits.
func New(bits uint, max int) *Hist {
	n := 1 << bits
	h := &Hist{
		n: n,
	}
	lastBucket := h.bucket(max)
	_, lastValue := h.cover(lastBucket)
	h.count = make([]int, lastValue+1)
	h.max = lastValue
	return h
}

// Parameters returns the number of bits resolution of the histogram and
// the maximum value which can be stored without overflowing.
func (h Hist) Parameters() (bits int, max int) {
	bits = -1
	for h.n > 0 {
		bits++
		h.n /= 2
	}
	return bits, h.max

}

// bucket returns the bucket index the value v belongs to. The value v must
// be in the range [0,h.Max].
func (h *Hist) bucket(v int) int {
	// block p covers the value range of [n * 2^p - n , n * 2^(p+1) - n)
	n := h.n
	if v < n {
		return v
	}

	p := uint(0)
	low := n*(1<<p) - n
	for low <= v {
		p++
		low = n*(1<<p) - n
	}
	p--
	low = n*(1<<p) - n
	bucketsize := 1 << p
	return n*int(p) + (v-low)/bucketsize
}

// cover returns the value intervall [a,b) which is covered by bucket.
func (h *Hist) cover(bucket int) (a int, b int) {
	// The bucket z covers the value range [a,b).
	// The bucket z is bin u = z%n in block p = z/n and is w = 1<<p values wide.
	// The bucket starts at a = n*2^p - n + u*w
	n := h.n
	u, p := bucket%n, uint(bucket/n)
	w := 1 << p
	a = n*(1<<p) - n + u*w
	return a, a + w
}

// Add counts the value v. Values below 0 or larger than h.Max are not stored
// in the histogram but counted in Underflow and Overflow.
func (h *Hist) Add(v int) {
	if v < 0 {
		h.Underflow++
		return
	}
	if v >= h.max {
		h.Overflow++
		return
	}
	b := h.bucket(v)
	h.count[b]++
}

// MustAdd works like Add but will panic if v under- or overflows the
// allowed range of [0,h.Max].
func (h *Hist) MustAdd(v int) {
	h.count[h.bucket(v)]++
}

// Integral returns the running sum of all buckets. The resulting slice
// is trimmed at the right end once all values have been summed.
func (h *Hist) integral() []uint64 {
	sum := make([]uint64, len(h.count))
	s := uint64(0)
	lastNonZero := 0
	for i, c := range h.count {
		s += +uint64(c)
		sum[i] = s
		if c > 0 {
			lastNonZero = i
		}
	}
	return sum[:lastNonZero+1]
}

// Quantiles returns an approximation to the sample quantiles for the given
// probabilities p which must be a sorted list of increasing float64s from
// the intervall [0,1].
// Quantiles ignores overflowing and underflowing values in h and can compute
// only an approximation if the bin sizes become large: The approximation will
// overestimate the real sample quantile.
func (h *Hist) Quantiles(p []float64) []int {
	sum := h.integral()
	psum := make([]float64, len(sum))
	n := float64(sum[len(sum)-1])
	for i, s := range sum {
		psum[i] = float64(s) / n
	}
	psum = append(psum, 2) // sentinel value

	v := make([]int, len(p))
	bucket := 0
	for i, x := range p {
		if x == 0 {
			for bucket < len(sum)-1 && psum[bucket+1] == 0 {
				bucket++
			}
			bucket++
		} else {
			for bucket < len(sum) && psum[bucket] <= x {
				bucket++
			}
		}
		a, b := h.cover(bucket)
		if psum[bucket] == 2 { // sentinel value
			v[i] = a - 1
		} else {
			xa, xb := psum[bucket-1], psum[bucket]
			f := (x - xa) / (xb - xa)
			v[i] = a + int(float64(b-a)*f)
		}
	}
	return v
}

// Average returns an approximation to the average of all non-over/underflowing
// values added to h.
func (h *Hist) Average() int {
	sum := uint64(0)
	n := uint64(0)
	for b, c := range h.count {
		left, right := h.cover(b)
		d := uint64(left + right - 1)
		sum += d * uint64(c/2)
		if c%2 == 1 {
			sum += d / 2
		}
		n += uint64(c)
	}
	if n == 0 {
		return 0
	}
	return int(sum / n)
}

// Bucket describes a bucket in the histogram for values [left, right]
// containing Count many values.
type Bucket struct {
	Left  int
	Right int
	Count int
}

// Data returns all non-empty buckets of h.
func (h *Hist) Data() []Bucket {
	b := []Bucket{}
	for i, c := range h.count {
		if c == 0 {
			continue
		}
		left, right := h.cover(i)
		b = append(b, Bucket{left, right, c})
	}
	return b
}

// String prints the histogram as a sequence of bucket ranges,
// count per bucket and two graphical representations: The '#' show
// the raw count while the '*' show counts scaled by bucket width.
func (h *Hist) String() string {
	buckets := h.Data()
	max, rmax := 0, 0.0
	for _, b := range buckets {
		if b.Count > max {
			max = b.Count
		}
		r := float64(b.Count) / float64(b.Right-b.Left)
		if r > rmax {
			rmax = r
		}
	}

	s := ""
	for _, b := range buckets {
		bar := strings.Repeat("#", (b.Count*30)/max)
		t := float64(b.Count) / (rmax * float64(b.Right-b.Left))
		rbar := strings.Repeat("*", int(t*30))
		s += fmt.Sprintf("%5d- %5d: %5d %-30s %-30s\n",
			b.Left, b.Right, b.Count, bar, rbar)
	}
	return s
}
