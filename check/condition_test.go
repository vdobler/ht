// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package check

import (
	"regexp"
	"testing"
)

var fullfilledTests = []struct {
	s string
	c Condition
	w string
}{
	// Prefix and Suffix
	{"foobar", Condition{Prefix: "foo"}, ``},
	{"foobar", Condition{Prefix: "waz"}, `Bad prefix, got "foo"`},
	{"foobar", Condition{Prefix: "wazwazwaz"}, `Bad prefix, got "foobar"`},
	{"foobar", Condition{Prefix: "foobarbar"}, `Bad prefix, got "foobar"`},
	{"foobar", Condition{Suffix: "bar"}, ``},
	{"foobar", Condition{Suffix: "waz"}, `Bad suffix, got "bar"`},
	{"foobar", Condition{Suffix: "wazwazwaz"}, `Bad suffix, got "foobar"`},
	{"foobar", Condition{Suffix: "foofoobar"}, `Bad suffix, got "foobar"`},
	{"foobar", Condition{Prefix: "foo", Suffix: "bar"}, ``},
	{"foobar", Condition{Prefix: "waz", Suffix: "bar"}, `Bad prefix, got "foo"`},
	{"foobar", Condition{Prefix: "foo", Suffix: "waz"}, `Bad suffix, got "bar"`},
	{"foobar", Condition{Prefix: "waz", Suffix: "waz"}, `Bad prefix, got "foo"`},
	// Contains
	{"foobarfoobar", Condition{Contains: "oo"}, ``},
	{"foobarfoobar", Condition{Contains: "waz"}, `Missing text`},
	{"foobarfoobar", Condition{Contains: "waz", Count: -1}, ``},
	{"foobarfoobar", Condition{Contains: "oo", Count: -1}, `Forbidden text`},
	{"foobarfoobar", Condition{Contains: "oo", Count: 2}, ``},
	{"foobarfoobar", Condition{Contains: "obarf", Count: 1}, ``},
	{"foobarfoobar", Condition{Contains: "o", Count: 4}, ``},
	{"foobarfoobar", Condition{Contains: "foo", Count: 1}, `Found 2 occurences`},
	{"foobarfoobar", Condition{Contains: "foo", Count: 3}, `Found 2 occurences`},
	// Regexp
	{"foobarwu", Condition{Regexp: "[aeiou]."}, ``},
	{"foobarwu", Condition{Regexp: "[aeiou].", Count: 2}, ``},
	{"foobarwu", Condition{Regexp: "[aeiou].", Count: 3}, `Found 2 matches`},
	{"foobarwu", Condition{Regexp: "[aeiou].", Count: -1}, `Forbidden match`},
	{"frtgbwu", Condition{Regexp: "[aeiou]."}, `Missing match`},
	// Min and Max
	{"foobar", Condition{Min: 2}, ``},
	{"foobar", Condition{Min: 20}, `Too short, was 6`},
	{"foobar", Condition{Max: 30}, ``},
	{"foobar", Condition{Max: 3}, `Too long, was 6`},
}

func TestFullfilled(t *testing.T) {
	for i, tc := range fullfilledTests {
		if tc.c.Regexp != "" {
			tc.c.re = regexp.MustCompile(tc.c.Regexp)
		}
		err := tc.c.Fullfilled(tc.s)
		switch {
		case tc.w == "" && err != nil:
			t.Errorf("%d. %s, unexpected error %s", i, tc.s, err)
		case tc.w != "" && err == nil:
			t.Errorf("%d. %s, missing error", i, tc.s)
		case tc.w != "" && err != nil && err.Error() != tc.w:
			t.Errorf("%d. %s, wrong error %q, want %q", i, tc.s, err, tc.w)
		}

	}
}
