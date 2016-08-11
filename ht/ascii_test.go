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
		"foo\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n13\n14\n15\nbar",
		"foo\n2\n3\n4\n5\n6\n7\n8\n\u22EE\n14\n15\nbar",
	},
	{
		strings.Repeat("x", 200),
		strings.Repeat("x", 120) + "\u2026",
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
