// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fingerprint provides fingerprinting of images.
package fingerprint

import (
	"fmt"
	"image"
	"image/color"
	"sort"
	"strconv"
)

// ----------------------------------------------------------------------------
// Color Histogram Fingerprinting
//
// See Marios A. Gavrielides, Elena Sikudova, Dimitris Spachos, and Ioannis Pitas
// in G. Antoniou et al. (Eds.): SETN 2006, LNAI 3955, pp. 494–497, 2006
// Springer-Verlag Berlin Heidelberg 2006
// http://poseidon.csd.auth.gr/papers/PUBLISHED/CONFERENCE/pdf/Gavrielides06a.pdf

// ColorHist is a normalized color histogram based on the
// colors from the Greta Mecbeth Color Picker.
type ColorHist [24]byte

// String produces a string representation by renormalizing the histogram
// to 16 so that it can be encoded in 24 hex digits.
func (ch ColorHist) String() string {
	buf := make([]byte, 0, 24)
	for _, n := range ch {
		v := (int64(n) + 7) >> 4 // proper rounding
		if v > 15 {
			v = 15
		}
		buf = strconv.AppendInt(buf, int64(n>>4), 16)
	}
	return string(buf)
}

// Image reconstructs the original image from the color histogram.
func (ch ColorHist) Image(width, height int) *image.RGBA {
	cbs := make(colorBinSlice, 24)
	total := 0
	for i, cnt := range ch {
		cbs[i].count = int(cnt)
		cbs[i].cidx = i
		total += int(cnt)
	}
	sort.Sort(cbs)

	for i := range cbs {
		cbs[i].fraction = float64(cbs[i].count) / float64(total)
		total -= cbs[i].count
		if total <= 0 {
			break
		}
	}

	remaining := image.Rect(0, 0, width, height)
	img := image.NewRGBA(remaining)

	left := true
	for _, e := range cbs {
		if e.fraction == 0 {
			break
		}
		if left {
			x := remaining.Min.X + int(e.fraction*float64(remaining.Dx()))
			fill := remaining
			fill.Max.X = x
			fillRect(img, fill, e.cidx)
			remaining.Min.X = x
			left = false
		} else {
			y := remaining.Min.Y + int(e.fraction*float64(remaining.Dy()))
			fill := remaining
			fill.Max.Y = y
			fillRect(img, fill, e.cidx)
			remaining.Min.Y = y
			left = true
		}
	}

	return img
}

// fill r of img with Mac Beth color no idx.
func fillRect(img *image.RGBA, r image.Rectangle, idx int) {
	mb := macbeth[idx]
	col := color.RGBA{uint8(mb[0]), uint8(mb[1]), uint8(mb[2]), 0xff}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, col)
		}
	}
}

type cBin struct {
	count    int // count of this bin
	cidx     int // the color, i.e. the index into macbeth table
	fraction float64
}

// colorBinSlice is a sortable slice of colorBins
type colorBinSlice []cBin

func (p colorBinSlice) Len() int           { return len(p) }
func (p colorBinSlice) Less(i, j int) bool { return p[i].count > p[j].count }
func (p colorBinSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// ColorHistDelta returns the difference between the two color histograms
// h and g.  The difference is in the range [0,1] with 0 for identical
// color histograms and 1 for maximal different histograms.
func ColorHistDelta(h, g ColorHist) float64 {
	return h.l1Norm(g)
}

func (h ColorHist) l1Norm(g ColorHist) float64 {
	// The histograms do not contain absolute counts but are scaled
	// the fullest bin equaling 255. Rescaling so that both images
	// contain the same number of pixels.
	nh, ng := 0, 0
	for i := 0; i < 24; i++ {
		nh += int(h[i])
		ng += int(g[i])
	}
	rh, rg := 1.0, 1.0
	if nh > ng {
		rg = float64(nh) / float64(ng)
	} else {
		rh = float64(ng) / float64(nh)
	}
	// 	fmt.Printf("%d  %d  rh=%.4f  rg=%.4f\n", nh, ng, rh, rg)
	sum := 0.0
	for i := 0; i < 24; i++ {
		d := (rh*float64(h[i]) - rg*float64(g[i])) / 255
		// fmt.Printf("  %2d (%3d,%3d) [%.4f,%.4f] %.4f\n", i, h[i], g[i],
		//	rh*float64(h[i]), rg*float64(g[i]), d)
		if d >= 0 {
			sum += d
		} else {
			sum -= d
		}
	}

	return sum / (24 * rg * rh)
}

// ColorHistFromString converts 24 hex digits to a ColorHist.
func ColorHistFromString(s string) (ColorHist, error) {
	ch := ColorHist{}
	if len(s) != 24 {
		return ch, fmt.Errorf("fingerprint: Bad format for ColorHist string %q", s)
	}

	a, err := strconv.ParseUint(s[0:16], 16, 64)
	if err != nil {
		return ch, err
	}
	b, err := strconv.ParseUint(s[16:24], 16, 64)
	if err != nil {
		return ch, err
	}

	mask := uint64(0xfffffffffffffff)
	shift := uint(60) // TODO: combine mark and shift
	for i := 0; i < 16; i++ {
		h := a >> shift
		t := h<<4 | h
		a &= mask
		mask >>= 4
		shift -= 4
		ch[i] = byte(t)
	}
	mask = uint64(0xfffffff)
	shift = 28 // TODO: combine mark and shift
	for i := 16; i < 24; i++ {
		h := b >> shift
		t := h<<4 | h
		b &= mask
		mask >>= 4
		shift -= 4
		ch[i] = byte(t)
	}

	return ch, nil
}

// NewColorHist computest the color histogram of img.
func NewColorHist(img image.Image) ColorHist {
	bounds := img.Bounds()

	hist := [24]int{}
	max := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			bin := colorBin(img.At(x, y))
			hist[bin]++
			if hist[bin] > max {
				max = hist[bin]
			}
		}
	}

	ch := ColorHist{}
	for bin := 0; bin < 24; bin++ {
		ch[bin] = byte(hist[bin] * 255 / max)
	}

	return ch
}

// colorBin returns the index of the nearest color in macbeth.
// Using an euclidean distance in RGB space because I have not the slightest
// understanding of color spaces and/or color perception.
func colorBin(c color.Color) int {
	rr, gg, bb, _ := c.RGBA()
	r := int(rr >> 8)
	g := int(gg >> 8)
	b := int(bb >> 8)

	min, bin := 200000, -1 // 200000 > 196608 = 3 * 256²
	for i, mb := range macbeth {
		rd, gd, bd := r-mb[0], g-mb[1], b-mb[2]
		d := rd*rd + gd*gd + bd*bd
		if d < min {
			min, bin = d, i
		}
	}
	return bin
}

// The 24 Macbeth colors from the ColorChecker as 8bit RGB values, taken from
// http://en.wikipedia.org/wiki/ColorChecker.
var macbeth = [][3]int{
	// Natural colors
	{0x73, 0x52, 0x44},
	{0xc2, 0x96, 0x82},
	{0x62, 0x7a, 0x9d},
	{0x57, 0x6c, 0x43},
	{0x85, 0x80, 0xb1},
	{0x67, 0xbd, 0xaa},

	// Miscellaneous colors
	{0xd6, 0x7e, 0x2c},
	{0x50, 0x5b, 0xa6},
	{0xc1, 0x5a, 0x63},
	{0x5e, 0x3c, 0x6c},
	{0x9d, 0xbc, 0x40},
	{0xe0, 0xa3, 0x2e},

	// Primary and secondary colors
	{0x38, 0x3d, 0x96},
	{0x46, 0x94, 0x49},
	{0xaf, 0x36, 0x3c},
	{0xe7, 0xc7, 0x1f},
	{0xbb, 0x56, 0x95},
	{0x08, 0x85, 0xa1},

	// Grayscale colors
	{0xf3, 0xf3, 0xf2},
	{0xc8, 0xc8, 0xc8},
	{0xa0, 0xa0, 0xa0},
	{0x7a, 0x7a, 0x79},
	{0x55, 0x55, 0x55},
	{0x34, 0x34, 0x34},
}
