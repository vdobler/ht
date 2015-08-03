// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fingerprint

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"testing"
)

func readImage(fn string) image.Image {
	file, err := os.Open(fn)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Decode the image.
	img, _, err := image.Decode(file)
	if err != nil {
		panic(err)
	}
	return img
}

func TestBinColor(t *testing.T) {
	for _, file := range []string{"boat", "clock", "lena", "baboon", "pepper"} {
		img := readImage("testdata/" + file + ".jpg")
		bounds := img.Bounds()
		bined := image.NewRGBA(bounds)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				bin := colorBin(img.At(x, y))
				mb := macbeth[bin]
				c := color.RGBA{uint8(mb[0]), uint8(mb[1]), uint8(mb[2]), 0xff}
				bined.SetRGBA(x, y, c)
			}
		}

		out, err := os.Create("testdata/" + file + ".bined.jpg")
		if err != nil {
			t.Fatal(err.Error())
		}
		err = jpeg.Encode(out, bined, nil)
		if err != nil {
			t.Fatal(err.Error())
		}
		out.Close()
	}
}

func TestUniformColorHist(t *testing.T) {
	red := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for x := 0; x < 100; x++ {
		for y := 0; y < 100; y++ {
			red.SetRGBA(x, y, color.RGBA{0xff, 0, 0, 0xff})
		}
	}
	green := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for x := 0; x < 100; x++ {
		for y := 0; y < 100; y++ {
			green.SetRGBA(x, y, color.RGBA{0, 0xff, 0, 0xff})
		}
	}
	blue := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for x := 0; x < 100; x++ {
		for y := 0; y < 50; y++ { // Only half filled!
			blue.SetRGBA(x, y, color.RGBA{0, 0, 0xff, 0xff})
		}
	}
	rh := NewColorHist(red)
	gh := NewColorHist(green)
	bh := NewColorHist(blue)

	if rh.String() != "00000000000000f000000000" ||
		gh.String() != "0000000000000f0000000000" ||
		bh.String() != "000000000000f0000000000f" {
		t.Fatalf("Got rh=%s gh=%s bh=%s", rh.String(), gh.String(), bh.String())
	}

	rb, rg, bg := rh.l1Norm(bh), rh.l1Norm(gh), bh.l1Norm(gh)

	// Two bins out of 24 differ completely
	if rg < 2.0/24-1e-6 || rg > 2.0/24+1e-6 {
		t.Errorf("Got %.6f, want 2/24=0.0833", rg)
	}

	// One bin differs completely (f vs 0), two differ half (scaling!) (0 vs f/2)
	if rb < 2.0/24-1e-6 || rb > 2.0/24+1e-6 {
		t.Errorf("Got %.6f, want 2/24=0.0833", rb)
	}
	if bg < 2.0/24-1e-6 || bg > 2.0/24+1e-6 {
		t.Errorf("Got %.6f, want 2/24=0.0833", bg)
	}
}

func TestMaxDiffColorHist(t *testing.T) {
	first := image.NewRGBA(image.Rect(0, 0, 100, 12))
	for x := 0; x < 100; x++ {
		for y := 0; y < 12; y++ {
			c := color.RGBA{uint8(macbeth[y][0]), uint8(macbeth[y][1]),
				uint8(macbeth[y][2]), 0xff}
			first.SetRGBA(x, y, c)
		}
	}
	second := image.NewRGBA(image.Rect(0, 0, 100, 12))
	for x := 0; x < 100; x++ {
		for y := 0; y < 12; y++ {
			c := color.RGBA{uint8(macbeth[y+12][0]),
				uint8(macbeth[y+12][1]), uint8(macbeth[y+12][2]), 0xff}
			second.SetRGBA(x, y, c)
		}
	}
	h := NewColorHist(first)
	g := NewColorHist(second)
	if h.l1Norm(g) < 0.99999999 {
		t.Errorf("g=%s  h=%s  delte=%f\n", h.String(), g.String(), h.l1Norm(g))
	}
}

func TestNewColorHist(t *testing.T) {
	files := []string{"boat", "clock", "lena", "lenal", "lenas", "lenat",
		"lenaf", "baboon", "pepper"}
	hists := make(map[string]ColorHist)
	bmvs := make(map[string]BMVHash)

	for _, file := range files {
		img := readImage("testdata/" + file + ".jpg")
		ch := NewColorHist(img)
		chstr := ch.String()
		ch2, err := ColorHistFromString(chstr)
		if err != nil {
			t.Errorf("%s: %s", file, err.Error())
		}
		ch2str := ch2.String()
		if chstr != ch2str {
			t.Errorf("%s %s != %s", file, chstr, ch2str)
		}
		fmt.Printf("%10s: %s\n", file, chstr)
		hists[file] = ch
		bmvs[file] = NewBMVHash(img)
	}

	fmt.Printf("           ")
	for _, a := range files {
		fmt.Printf("%-8s", a)
	}
	fmt.Println()
	for _, a := range files {
		h := hists[a]
		fmt.Printf("%9s  ", a)
		for _, b := range files {
			g := hists[b]
			fmt.Printf("%.4f  ", ColorHistDelta(h, g))
		}
		fmt.Println()
		fmt.Printf("           ")
		for _, b := range files {
			fmt.Printf("%.4f  ", BMVDelta(bmvs[a], bmvs[b]))

		}
		fmt.Println()
	}

}

func TestColorImage(t *testing.T) {
	for _, file := range []string{"boat", "clock", "lena", "baboon", "pepper"} {
		img := readImage("testdata/" + file + ".jpg")
		ch := NewColorHist(img)
		reconstructed := ch.Image(64, 64)
		out, err := os.Create("testdata/" + file + ".colrec.png")
		if err != nil {
			t.Fatal(err.Error())
		}
		err = png.Encode(out, reconstructed)
		if err != nil {
			t.Fatal(err.Error())
		}
		out.Close()
	}
}

func TestColorImageSpecial(t *testing.T) {
	ch, err := ColorHistFromString("3102000000f002e000021006")
	if err != nil {
		t.Fatalf("Ooops: %v", err)
	}

	ch.Image(64, 64)
}
