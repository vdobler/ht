// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kr/pretty"
	"github.com/vdobler/ht/internal/json5"
)

var exampleTest = Test{
	Name:        "Name",
	Description: "Description",
	Request: Request{
		Method: "GET",
		URL:    "http://example.org",
		Params: URLValues{
			"param": {"val1", "val2"},
		},
		ParamsAs: "multipart",
		Header: http.Header{
			"header": {"head1", "head2"},
		},
		Cookies: []Cookie{
			{Name: "cname", Value: "cvalue"},
		},
		Body:            "body",
		FollowRedirects: true,
	},
	Checks: CheckList{},
	VarEx: map[string]Extractor{
		"extract": Extractor{
			HTMLElementSelector:  "elemSel",
			HTMLElementAttribute: "elemAttr",
		},
	},
	Poll: Poll{
		Max:   106,
		Sleep: Duration(107),
	},
	Timeout:    Duration(101),
	Verbosity:  102,
	PreSleep:   Duration(103),
	InterSleep: Duration(104),
	PostSleep:  Duration(105),
}

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
	r, err := Repeat(test, nrep, variables)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if len(r) != 6 {
		t.Fatalf("Got %d repetitions, want 6: %#v", len(r), r)
	}

	for i, want := range []string{"q=foo c=1 f=fix", "q=bar c=2 f=fix", "q=foo c=3 f=fix",
		"q=bar c=1 f=fix", "q=foo c=2 f=fix", "q=bar c=3 f=fix"} {
		if !strings.HasPrefix(r[i].Description, want) {
			t.Errorf("%d got %q, want %q", i, r[i].Description, want)
		}
	}
}

func TestRepeatNoSubst(t *testing.T) {
	orig := exampleTest
	reps, err := Repeat(&orig, 3, nil)

	if len(reps) != 3 || err != nil {
		t.Fatalf("len(resp)=%d, err=%v", len(reps), err)
	}

	for i := 0; i < 3; i++ {
		if !reflect.DeepEqual(orig, *(reps[i])) {
			origpp, _ := json5.MarshalIndent(orig, "", "  ")
			copypp, _ := json5.MarshalIndent(*(reps[i]), "", "  ")
			t.Errorf("Original:\n%s\nCopy %d\n%s", origpp, i, copypp)
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

func TestSubstituteTestVariables(t *testing.T) {
	test := Test{
		Name:        "Name={{x}}",
		Description: "Desc={{x}}",
		Request: Request{
			Method: "GET",
			URL:    "url={{x}}",
			Params: URLValues{
				"pn={{x}}": []string{"pv={{x}}"},
			},
			Header: http.Header{
				"{{x}}head": []string{"{{x}}val"},
			},
			FollowRedirects: true,
		},
		Checks: []Check{
			&Body{Contains: "bctext={{x}}", Count: 1},
			&Header{Header: "Location{{x}}", Condition: Condition{Suffix: "foo{{x}}bar"}},
		},
	}

	repl := strings.NewReplacer("{{x}}", "Y")
	rt := test.substituteVariables(replacer{str: repl, fn: nil})
	if rt.Name != "Name=Y" || rt.Description != "Desc=Y" ||
		rt.Request.URL != "url=Y" ||
		rt.Request.Params["pn={{x}}"][0] != "pv=Y" || // TODO: names too?
		rt.Request.Header["{{x}}head"][0] != "Yval" || // TODO: header keys too?
		rt.Checks[0].(*Body).Contains != "bctext=Y" ||
		rt.Checks[1].(*Header).Header != "LocationY" ||
		rt.Checks[1].(*Header).Suffix != "fooYbar" {
		t.Errorf("%s", pretty.Sprintf("%# v\n", test))
	}

}

func TestNewReplacer(t *testing.T) {
	vm := map[string]string{
		"HOST":  "example.test",
		"user":  "JohnDoe",
		"#9991": "401",
		"#9992": "-3",
	}

	repl, err := newReplacer(vm)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	s := repl.str.Replace("http://{{HOST}}/path?u={{user}}")
	if s != "http://example.test/path?u=JohnDoe" {
		t.Errorf("Got %q", s)
	}

	a, b := repl.fn[9991], repl.fn[9992]
	if len(repl.fn) != 2 || a != 401 || b != -3 {
		t.Errorf("Got %+v", repl.fn)
	}
}

func TestFindNow(t *testing.T) {
	test := Test{
		Name:        "now == {{NOW}}",
		Description: "now+1 == {{NOW+1s}}",
		Request: Request{
			Method: "GET",
			URL:    "now+2 == {{NOW + 2s}}",
			Params: URLValues{
				"text": []string{`now+3 == {{NOW+3s | "2006-Jan-02"}}`}},
			Header:          http.Header{"header": []string{"now+4 == {{NOW+4s}}"}},
			FollowRedirects: false,
		},
		Checks: []Check{
			&Body{Contains: "now+5 == {{NOW + 15m}}"},
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
