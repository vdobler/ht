// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fingerprint

import (
	"image/png"
	"os"
	"testing"
)

func TestHammingDistance(t *testing.T) {
	a := BMVHash(0x99) // 10011001
	b := BMVHash(0x9a) // 10011010
	c := BMVHash(0x9b) // 10011011
	d := BMVHash(0x33) // 00110011

	if a.HammingDistance(a) != 0 {
		t.Fail()
	}
	if a.HammingDistance(b) != 2 {
		t.Fail()
	}
	if a.HammingDistance(c) != 1 {
		t.Fail()
	}
	if a.HammingDistance(d) != 4 {
		t.Fail()
	}
	if c.HammingDistance(d) != 3 {
		t.Fail()
	}
}

func TestBMVImage(t *testing.T) {
	for _, file := range []string{"boat", "clock", "lena", "baboon", "pepper"} {
		img := readImage("testdata/" + file + ".jpg")
		h := NewBMVHash(img)
		reconstructed := h.Image(64, 64)
		out, err := os.Create("testdata/" + file + ".bmvrec.png")
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

func TestBMVImageSpecial(t *testing.T) {
	bmv, err := BMVHashFromString("0f0f0f0f0f0f0f0f")
	if err != nil {
		t.Fatalf("Ooops: %v", err)
	}

	bmv.Image(8, 8)
}
