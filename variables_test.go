// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kr/pretty"
	"github.com/vdobler/ht/check"
)

func TestRepeat(t *testing.T) {
	test := &Test{Description: "q={{query}} c={{count}} f={{f}}"}

	variables := map[string][]string{
		"query": []string{"foo", "bar"},
		"count": []string{"1", "2", "3"},
		"f":     []string{"fix"},
	}

	nrep := lcmOf(variables)
	if nrep != 6 {
		t.Errorf("Got %d as lcmOf, wnat 6", nrep)
	}
	r := Repeat(test, nrep, variables)
	if len(r) != 6 {
		t.Fatalf("Got %d repetitions, want 6: %#v", len(r), r)
	}

	for i, want := range []string{"q=foo c=1 f=fix", "q=bar c=2 f=fix", "q=foo c=3 f=fix",
		"q=bar c=1 f=fix", "q=foo c=2 f=fix", "q=bar c=3 f=fix"} {
		if r[i].Description != want {
			t.Errorf("%d got %q, want %q", i, r[i].Description, want)
		}
	}
}

func TestLCM(t *testing.T) {
	for i, tc := range []struct{ n, m, e int }{
		{1, 1, 1},
		{2, 3, 6},
		{12, 4, 12},
		{2 * 2 * 3 * 5 * 5, 2 * 3 * 3 * 5 * 7, 2 * 2 * 3 * 3 * 5 * 5 * 7},
	} {
		if got := lcm(tc.n, tc.m); got != tc.e {
			t.Errorf("%d: lcm(%d,%d)=%d want %d", i, tc.n, tc.m, got, tc.e)
		}

	}
}

func TestSubstituteVariables(t *testing.T) {
	test := Test{
		Name:        "Name={{x}}",
		Description: "Desc={{x}}",
		Request: Request{
			Method: "GET",
			URL:    "url={{x}}",
			Params: url.Values{
				"pn={{x}}": []string{"pv={{x}}"},
			},
			Header: http.Header{
				"{{x}}head": []string{"{{x}}val"},
			},
			FollowRedirects: true,
		},
		Checks: []check.Check{
			check.BodyContains{Text: "bctext={{x}}", Count: 1},
		},
	}

	repl := strings.NewReplacer("{{x}}", "Y")
	rt := test.substituteVariables(repl)
	if rt.Name != "Name=Y" || rt.Description != "Desc=Y" ||
		rt.Request.URL != "url=Y" ||
		rt.Request.Params["pn={{x}}"][0] != "pv=Y" || // TODO: names too?
		rt.Request.Header["{{x}}head"][0] != "Yval" || // TODO: header keys too?
		rt.Checks[0].(check.BodyContains).Text != "bctext=Y" {
		t.Errorf("%s", pretty.Sprintf("%# v\n", test))
	}

}

func TestNewReplacer(t *testing.T) {
	vm := map[string]string{
		"HOST":   "www.google.com",
		"NOW":    "Foo {{NOW}} Bar",
		"NOW+7":  "{{NOW +7m}}",
		"FUTURE": `{{NOW + 8d | "Jan 2006"}}`,
		"JETZT":  `{{NOW | "02.Jan.2006 15:04h"}}`,
	}

	r := newReplacer(vm)
	for k, _ := range vm {
		t.Logf("%s --> %s", k, r.Replace("{{"+k+"}}"))
	}
}

func TestFindNow(t *testing.T) {
	test := Test{
		Name:        "now == {{NOW}}",
		Description: "now+1 == {{NOW+1s}}",
		Request: Request{
			Method: "GET",
			URL:    "now+2 == {{NOW + 2s}}",
			Params: url.Values{
				"text": []string{`now+3 == {{NOW+3s | "2006-Jan-02"}}`}},
			Header:          http.Header{"header": []string{"now+4 == {{NOW+4s}}"}},
			FollowRedirects: false,
		},
		Checks: []check.Check{
			check.BodyContains{Text: "now+5 == {{NOW + 15m}}"},
		},
	}
	nv := test.findNowVariables()
	if len(nv) != 6 {
		fmt.Printf("Got %v\n", nv)
	}
	want := []string{`{{NOW}}`, `{{NOW+1s}}`, `{{NOW + 2s}}`,
		`{{NOW+3s | "2006-Jan-02"}}`, `{{NOW+4s}}`, `{{NOW + 15m}}`}
	for i, got := range nv {
		if got != want[i] {
			t.Errorf("%d got %q, want %q", i, got, want[i])
		}
	}

	if testing.Verbose() {
		fmt.Printf("%#v\n", test.nowVariables(time.Now()))
	}
}
