// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fingerprint

import (
	"strings"
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

func TestBitBlock(t *testing.T) {
	h := BMVHash(0xff00aa0f55f03cc3)
	b := strings.Join(h.BitBlock(), "|")
	if b != "11111111|00000000|10101010|00001111|01010101|11110000|00111100|11000011" {
		t.Errorf("Got %s", b)
	}
}
