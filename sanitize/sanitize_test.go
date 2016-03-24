// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sanitize

import "testing"

var sfTests = []struct {
	name, want string
}{
	{"foo", "foo"},
	{"*", "_"},
	{":#", "_"},
	{"foo&bar", "foo_and_bar"},
	{"-foo-", "foo"},
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
