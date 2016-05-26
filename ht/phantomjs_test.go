// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"testing"
)

var geometryTests = []struct {
	s             string
	width, height int
	left, top     int
	zoom          int
	ok            bool
}{
	{"1x2+3+4*5", 1, 2, 3, 4, 5, true},
	{"1x2+3+4", 1, 2, 3, 4, 0, true},
	{"1x2", 1, 2, 0, 0, 0, true},
	{"1x2*5", 1, 2, 0, 0, 5, true},
	{"hubbabubba", 0, 0, 0, 0, 0, false},
	{"1x2*5*6", 1, 2, 0, 0, 5, false},
	{"1x2+3+4+9*5", 1, 2, 3, 4, 5, false},
	{"axb+c+d*e", 0, 0, 0, 0, 0, false},
}

func TestParseGeometry(t *testing.T) {
	for i, tc := range geometryTests {
		g, err := parseGeometry(tc.s)
		if err != nil {
			if tc.ok {
				t.Errorf("%d. %q: Unexpected error %s", i, tc.s, err)
			}
			continue
		}
		if err == nil && !tc.ok {
			t.Errorf("%d. %q: Missing error", i, tc.s)
			continue
		}
		if g.width != tc.width || g.height != tc.height {
			t.Errorf("%d. %q: Wrong size, got %dx%d", i, tc.s, g.width, g.height)
		}
		if g.top != tc.top || g.left != tc.left {
			t.Errorf("%d. %q: Wrong offset, got +%d+%d", i, tc.s, g.left, g.top)
		}
		if g.zoom != tc.zoom {
			t.Errorf("%d. %q: Wrong zoom, go %d", i, tc.s, g.zoom)
		}

	}
}

func TestDeltaImage(t *testing.T) {
	t.Skip("not ready jet")
	a, err := readImage("A.png")
	if err != nil {
		panic(err)
	}
	b, err := readImage("B.png")
	if err != nil {
		panic(err)
	}

	ignore := []image.Rectangle{
		image.Rect(500, 200, 1000, 400),
	}

	delta, low, high := imageDelta(a, b, ignore)
	deltaFile, err := os.Create("D.png")
	if err != nil {
		panic(err)
	}
	defer deltaFile.Close()
	png.Encode(deltaFile, delta)

	r := delta.Bounds()
	N := r.Dx() * r.Dy()
	fmt.Println(N, low, high)
	fmt.Printf("Low %.2f%%   High %.2f%%\n",
		float64(100*low)/float64(N), float64(100*high)/float64(N))
}
