// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gui

import (
	"testing"
)

func TestBinaryString(t *testing.T) {
	for i, tc := range []struct {
		in  string
		bin bool
	}{
		{"", false},
		{"Hello World", false},
		{`{"some":"JSON"}`, false},
		{"\xEF\xBB\xBFHello World", true},
		{"\x00\x00", true},
		{"Hello\x04blob", true},
		{"\xbd\xb2\x3d\xbc\x20\xe2\x8c\x98", true},
	} {
		if got := binaryString(tc.in); (got != nil) != tc.bin {
			t.Errorf("%d: binaryString(%q)=%t, want %t",
				i, tc.in, (got == nil), tc.bin)
		}
	}

}
