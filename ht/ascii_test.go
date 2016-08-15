// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"strings"
	"testing"
)

var summaryTests = []struct{ in, want string }{
	{
		"foo",
		"foo",
	},
	{
		strings.Repeat("A\n2\nB\n4\n", 50),
		strings.Repeat("A\n2\nB\n4\n", 6) + "A\n2\nB\n\u22EE\n" + strings.Repeat("A\n2\nB\n4\n", 3),
	},
	{
		strings.Repeat("x", 200),
		strings.Repeat("x", 150) + "\u2026",
	},
	{
		`{"foo":[1,2,3],"bar":"waz"}`,
		`{
    "bar": "waz",
    "foo": [
        1,
        2,
        3
    ]
}`,
	},
	{
		"\n\n\t <html><body><p>Hello</p></body></html>",
		"<html><body><p>Hello</p></body></html>",
	},
	{
		"%PDF-1.4\n1 0 obj\n\x00\xff\xf5",
		typeIndicator("PDF"),
	},
	{
		"\x89\x50\x4E\x47\x0D\x0A\x1A\x0Adsklfjsdklfjsdfdfds",
		typeIndicator("PNG"),
	},
}

func TestSummary(t *testing.T) {
	for i, tc := range summaryTests {
		if got := Summary(tc.in); got != tc.want {
			t.Errorf("%d. Got\n%s\nWant\n%s", i, got, tc.want)
		}
	}
}
