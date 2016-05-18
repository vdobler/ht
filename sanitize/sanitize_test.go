// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sanitize

import "testing"

var sfTests = []struct {
	name, want string
}{
	{"", ""},
	{"foo", "foo"},
	{"*", "_"},
	{":#", "_"},
	{"foo&bar", "foo_and_bar"},
	{"-foo-", "foo"},
	{"-", ""},
	{"Hütte", "Huette"},
	{"© 2015", "_2015"},
	{"bis-¼-voll", "bis-_-voll"},
	{"le été garçon ŷ", "le_ete_garcon_y"},
}

func TestFilename(t *testing.T) {
	for i, tc := range sfTests {
		if got := Filename(tc.name); got != tc.want {
			t.Errorf("%d: SanitizeFilename(%q) = %q, want %q",
				i, tc.name, got, tc.want)
		}

	}
}

var benchmarkFilename string

func BenchmarkFilename(b *testing.B) {
	const have = `le été garçon ŷ-`
	const want = `le_ete_garcon_y`
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchmarkFilename = Filename(have)
		if benchmarkFilename != want {
			b.Errorf("%d: SanitizeFilename(%q) = %q, want %q",
				i, have, benchmarkFilename, want)
		}
	}
}
