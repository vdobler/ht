// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fingerprint

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
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
	for _, file := range []string{"boat", "clock", "lena", "mandrill", "peppers"} {
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

func TestNewColorHist(t *testing.T) {
	for _, file := range []string{"boat", "clock", "lena", "mandrill", "peppers"} {
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
	}
}
