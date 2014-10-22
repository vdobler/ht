// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fingerprint provides fingerprinting of images.
package fingerprint

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"sort"
	"strconv"
)

// BMVHash is the 64 bit block mean value hash of an image.
type BMVHash uint64

// String returns h in hexadecimal form.
func (h BMVHash) String() string {
	return fmt.Sprintf("%016x", uint64(h))
}

// NewBMVHashFromString parses the hexadecimal number in s and panics
// if s cannot be parsed to a uint64.
func NewBMVHashFromString(s string) BMVHash {
	v, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		panic(err)
	}
	return BMVHash(v)
}

// HammingDistance returns the Hammig distance between
// the bit strings of h and g.
func (h BMVHash) HammingDistance(v BMVHash) int {
	dist := 0
	val := h ^ v
	for val != 0 {
		dist++
		val &= val - 1
	}
	return dist
}

// BitBlock produces a binary representation.
func (h BMVHash) BitBlock() []string {
	block := make([]string, 8)
	s := ""
	for i := 0; i < 64; i++ {
		if h&(1<<uint(63-i)) != 0 {
			s += "1"
		} else {
			s += "0"
		}
		if i%8 == 7 {
			block[i/8] = s
			s = ""
		}
	}
	return block
}

// Delta produces a small ASCII art string where each bit difference between
// h and g is marked with "XX" and identical bits marked with "--".  The
// resulting string consists of 8 lines (\n seperated), each 16 charater
// long.
func (h BMVHash) Delta(g BMVHash) string {
	s := ""
	for i := 0; i < 64; i++ {
		var bit uint64 = (1 << uint(63-i))
		if uint64(h)&bit != uint64(g)&bit {
			s += "XX"
		} else {
			s += "--"
		}
		if i%8 == 7 {
			s += "\n"
		}
	}
	return s
}

// produce a [0, 0xFFFFFFFF] gray value from [0, 0xFFFF] r g b data
func rgb2gray(r, g, b uint32) uint32 {
	// 0.2989 * R + 0.5870 * G + 0.1140 * B
	// 0.2989 * r / 256 = (0.2989/256) * r = r / (256/0.2989)
	return r*19588 + g*38469 + b*7471
}

// blockMeanValue calculates the 64bit block mean value hash of img.
// It uses an algorithm accoring to Di Wu, Xuebing Zhou, and Xiamu Niu. 2009.
// A novel image hash algorithm resistant to print-scan. Signal Process. 89,
// 12 (December 2009), 2415-2424.  As described in Christoph Zauner:
// "Implementation and Benchmarking of Perceptual Image Hash Functions"
// DIPLOMARBEIT, FH Hagenberg, Juli 2010.
// The following algorithm is used:
//   *  The image is converted to a 8-bit gray scale image
//   *  The image is devided into 8x8 non-overlapping blocks, for each block
//      the mean gray value is calculated
//   *  The average and medain of the 64 blocks is calculated.
//      If the median is >= 250 than the limit is set to the average
//      else to the median.
//   *  If the mean value of a block is higher than the limit, the
//      corresponding bit in the hash is set to 1, otherwise to 0.
// The blocks need not be aligned to pixels but may contain a fraction
// of a pixel.  This is needed as otherwise tiny resises of an image
// may result in slight different inclusion of pixels in a block which
// can switch this blocks value. This is an artefact of integer rounding
// and not a difference in the image.  Using fractional block boundaries
// make the hash value a much better fingerprint: It will be identical
// more often and if it misguided produces a bit difference, the Hamming
// distance is much lower. This increase in quality justifies the increased
// complexity and running time of the algorithm.
// Switching to the average value if the median is too high is necessary
// as otherwise half-white (or half-blackI images cannot be fingerprinted
// properly.
//
// The returned fingerprint hash is 0 for all images with at least one
// dimension smaller than 8 pixel.
// Images which are smaller than 16x16 pixel have a fingerprint of just
// 1s.
func NewBMVHash(img image.Image) BMVHash {
	bounds := img.Bounds()

	// handle too small images first
	if bounds.Dx() < 8 || bounds.Dy() < 8 { // degenerate case
		return BMVHash(0)
	}
	if bounds.Dx() < 16 && bounds.Dy() < 16 { // second degenerate case
		return BMVHash(0xFFFFFFFFFFFFFFFF)
	}

	sum := make([]uint64, 64) // running sum of each of the 64 blocks
	dw, dh := float64(bounds.Dx())/8, float64(bounds.Dy())/8

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		ty, by := int(float64(y-bounds.Min.Y)/dh), int(float64(y-bounds.Min.Y+1)/dh)
		for xx := bounds.Min.X; xx < bounds.Max.X; xx++ {
			r, g, b, _ := img.At(xx, y).RGBA()
			gv := rgb2gray(r, g, b) >> 24

			x := xx - bounds.Min.X
			lx, rx := int(float64(x)/dw), int(float64(x+1)/dw)

			if lx == rx {
				if ty == by {
					box := lx + 8*ty
					sum[box] = sum[box] + uint64(gv)
				} else {
					fy := dh*float64(by) - float64(y)
					if fy > 0.01 {
						box := lx + 8*ty
						sum[box] = sum[box] + uint64(float64(gv)*fy)
					}
					if fy < 0.99 {
						box := lx + 8*by
						sum[box] = sum[box] + uint64(float64(gv)*(1-fy))
					}
				}
			} else { // lx != rx
				fx := dw*float64(rx) - float64(x)
				if ty == by {
					if fx > 0.01 {
						box := lx + 8*ty
						sum[box] = sum[box] + uint64(float64(gv)*fx)
					}
					if fx < 0.99 {
						box := rx + 8*ty
						sum[box] = sum[box] + uint64(float64(gv)*(1-fx))
					}
				} else {
					// lx!=rx && ty!=by
					fy := dh*float64(by) - float64(y)
					if fx*fy > 0.01 {
						box := lx + 8*ty
						sum[box] = sum[box] + uint64(float64(gv)*fx*fy)
					}
					if (1-fx)*fy > 0.01 {
						box := rx + 8*ty
						sum[box] = sum[box] + uint64(float64(gv)*(1-fx)*fy)
					}
					if fx*(1-fy) > 0.01 {
						box := lx + 8*by
						sum[box] = sum[box] + uint64(float64(gv)*fx*(1-fy))
					}
					if (1-fx)*(1-fy) > 0.01 {
						box := rx + 8*by
						sum[box] = sum[box] + uint64(float64(gv)*(1-fx)*(1-fy))
					}
				}
			}
		}
	}

	// Calculate mean value per block and total average from sum
	area := dw * dh // of one block
	means := make([]int, 64)
	average := 0.0
	for i := range sum {
		v := float64(sum[i]) / area
		means[i] = int(v)
		average += v
	}
	average /= 64

	// calculate median value
	med := make([]int, 64)
	copy(med, means)
	sort.Ints(med)
	median := (med[31] + med[32]) / 2

	// calculate the bit hash
	limit := median
	if median >= 250 /* || median <= 5 */ {
		// medain value is too extrem (image contains lot of
		// large homegenious white or black areas): use
		// average in this case (empirically better)
		limit = int(average)
	}
	var hash BMVHash
	for i, v := range means {
		if v > limit {
			hash |= 1 << uint(63-i)
		}
	}

	return hash
}

// ColorHist is a normalized color histogram based on the
// colors from the Greta Mecbeth Color Picker.
type ColorHist [24]byte

// String produces a string representation by renormalizing the histogram
// to 16 so that it can be encoded in 24 hex digits.
func (ch ColorHist) String() string {
	buf := make([]byte, 0, 24)
	for _, n := range ch {
		buf = strconv.AppendInt(buf, int64(n>>4), 16)
	}
	return string(buf)
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

// ColorHist computest the color histogram of img.
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
var macbeth [][3]int = [][3]int{
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
