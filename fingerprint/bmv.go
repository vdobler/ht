// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Block Mean Value Fingerprinting
//
// blockMeanValue calculates the 64bit block mean value hash of img.
// It uses an algorithm according to Di Wu, Xuebing Zhou, and Xiamu Niu. 2009.
// A novel image hash algorithm resistant to print-scan. Signal Process. 89,
// 12 (December 2009), 2415-2424.  As described in Christoph Zauner:
// "Implementation and Benchmarking of Perceptual Image Hash Functions"
// DIPLOMARBEIT, FH Hagenberg, Juli 2010.
// The following algorithm is used:
//   *  The image is converted to a 8-bit gray scale image
//   *  The image is divided into 8x8 non-overlapping blocks, for each block
//      the mean gray value is calculated
//   *  The average and median of the 64 blocks is calculated.
//      If the median is >= 250 than the limit is set to the average
//      else to the median.
//   *  If the mean value of a block is higher than the limit, the
//      corresponding bit in the hash is set to 1, otherwise to 0.
// The blocks need not be aligned to pixels but may contain a fraction
// of a pixel.  This is needed as otherwise tiny resizes of an image
// may result in slight different inclusion of pixels in a block which
// can switch this blocks value. This is an artefact of integer rounding
// and not a difference in the image.  Using fractional block boundaries
// makes the hash value a much better fingerprint: It will be identical
// more often and if it misguided produces a bit difference, the Hamming
// distance is much lower. This increase in quality justifies the increased
// complexity and running time of the algorithm.
// Switching to the average value if the median is too high is necessary
// as otherwise half-white (or half-black images cannot be fingerprinted
// properly.

package fingerprint

import (
	"fmt"
	"image"
	"image/color"
	"sort"
	"strconv"
)

// BMVHash is the 64 bit block mean value hash of an image. The following
// references contain more information: Di Wu, Xuebing Zhou, and Xiamu Niu. 2009.
// A novel image hash algorithm resistant to print-scan. Signal Process. 89,
// 12 (December 2009), 2415-2424.  As described in Christoph Zauner:
// "Implementation and Benchmarking of Perceptual Image Hash Functions"
// DIPLOMARBEIT, FH Hagenberg, Juli 2010.
type BMVHash uint64

// String returns h in hexadecimal form.
func (h BMVHash) String() string {
	return fmt.Sprintf("%016x", uint64(h))
}

// Image "reconstructs" the original image through gray panels.
// The generated image has dimensions width x height. Both dimensions
// must be at least 8.
func (h BMVHash) Image(width, height int) *image.Gray {
	i := image.NewGray(image.Rect(0, 0, width, height))
	dark, light := color.Gray{0x20}, color.Gray{0xdf}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			bit := 63 - (y*8)/height*8 - (x*8)/width
			v := (h >> uint(bit)) & 1
			if v == 1 {
				i.SetGray(x, y, light)
			} else {
				i.SetGray(x, y, dark)
			}
		}
	}
	return i
}

// BMVHashFromString parses the hexadecimal number in s and panics
// if s cannot be parsed to a uint64.
func BMVHashFromString(s string) (BMVHash, error) {
	v, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		return 0, err
	}
	return BMVHash(v), nil
}

// BMVDelta returns the difference between the two block mean value hashes
// h and g.  The difference is in the range [0,1] with 0 for identical
// hashes and 1 for maximal different hashes (i.e. a Hamming distance of 64).
// The lowest significant difference in BMVHashes is 2 bits corrsponding to
// a delta of 2/64 = 0.03125.
func BMVDelta(h, g BMVHash) float64 {
	return float64(h.HammingDistance(g)) / 64
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

// produce a [0, 0xFFFFFFFF] gray value from [0, 0xFFFF] r g b data
func rgb2gray(r, g, b uint32) uint32 {
	// 0.2989 * R + 0.5870 * G + 0.1140 * B
	// 0.2989 * r / 256 = (0.2989/256) * r = r / (256/0.2989)
	return r*19588 + g*38469 + b*7471
}

// NewBMVHash computes the block mean value hash of img.
// The following degenerate case return special values:
//   * If one or both dimensions of the image are < 8 then BMV hash
//     of 0 (i.e. 64 0s) is returned.
//   * If one dimension is smaller than 16 a fingerprint of 64
//     1s is returned.
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
