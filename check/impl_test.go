// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package check

import (
	"fmt"
	_ "image/png"
	"testing"
	"time"

	"github.com/vdobler/ht/response"
)

type TC struct {
	r response.Response
	c Check
	e error
}

var someError = fmt.Errorf("any error")
var ms = time.Millisecond

func runTest(t *testing.T, i int, tc TC) {
	got := tc.c.Okay(&tc.r)
	switch {
	case got == nil && tc.e == nil:
		return
	case got != nil && tc.e == nil:
		t.Errorf("%d. %s %v: unexpected error %v",
			i, NameOf(tc.c), tc.c, got)
	case got == nil && tc.e != nil:
		t.Errorf("%d. %s %v: missing error, want %v",
			i, NameOf(tc.c), tc.c, tc.e)
	case got != nil && tc.e != nil:
		_, malformed := got.(MalformedCheck)
		if (tc.e == someError && !malformed) ||
			(tc.e == NotFound && got == NotFound) ||
			(tc.e == FoundForbidden && got == FoundForbidden) {
			return
		}
		switch tc.e.(type) {
		case MalformedCheck:
			if !malformed {
				t.Errorf("%d. %s %v:got \"%v\" of type %T, want MalformedCheck",
					i, NameOf(tc.c), tc.c, got, got)
			}
		default:
			t.Errorf("%d. %s %v: got %T of type \"%v\", want %T",
				i, NameOf(tc.c), tc.c, got, got, tc.e)
		}
	}
}

var responseTimeTests = []TC{
	{response.Response{Duration: 10 * ms}, ResponseTime{Lower: 20 * ms}, nil},
	{response.Response{Duration: 10 * ms}, ResponseTime{Lower: 2 * ms}, someError},
	{response.Response{Duration: 10 * ms}, ResponseTime{Higher: 2 * ms}, nil},
	{response.Response{Duration: 10 * ms}, ResponseTime{Higher: 20 * ms}, someError},
	{response.Response{Duration: 10 * ms}, ResponseTime{Higher: 5 * ms, Lower: 20 * ms}, nil},
	{response.Response{Duration: 10 * ms}, ResponseTime{Higher: 15 * ms, Lower: 20 * ms}, someError},
	{response.Response{Duration: 10 * ms}, ResponseTime{Higher: 5 * ms, Lower: 8 * ms}, someError},
	{response.Response{Duration: 10 * ms}, ResponseTime{Higher: 20 * ms, Lower: 5 * ms},
		MalformedCheck{Err: someError}},
}

func TestResponseTime(t *testing.T) {
	for i, tc := range responseTimeTests {
		runTest(t, i, tc)
	}
}

var bcr = response.Response{Body: []byte("foo bar baz foo foo")}
var bodyContainsTests = []TC{
	{bcr, BodyContains{Text: "foo"}, nil},
	{bcr, BodyContains{Text: "bar"}, nil},
	{bcr, BodyContains{Text: "baz"}, nil},
	{bcr, BodyContains{Text: "foo", Count: 3}, nil},
	{bcr, BodyContains{Text: "baz", Count: 1}, nil},
	{bcr, BodyContains{Text: "wup", Count: -1}, nil},
	{bcr, BodyContains{Text: "foo bar", Count: 1}, nil},
	{bcr, BodyContains{Text: "sit"}, NotFound},
	{bcr, BodyContains{Text: "bar", Count: -1}, FoundForbidden},
	{bcr, BodyContains{Text: "bar", Count: 2}, someError}, // TODO: real error checking
}

func TestBodyContains(t *testing.T) {
	for i, tc := range bodyContainsTests {
		runTest(t, i, tc)
	}
}

var bmr = response.Response{Body: []byte("Hello World!")}
var bodyMatchTests = []TC{
	{bmr, &BodyMatch{Regexp: "ll"}, nil},
	{bmr, &BodyMatch{Regexp: "He.*ld"}, nil},
	{bmr, &BodyMatch{Regexp: "He...ld"}, NotFound},
}

func TestBodyMatch(t *testing.T) {
	for i, tc := range bodyMatchTests {
		runTest(t, i, tc)
	}
}

var imgr = response.Response{Body: []byte(
	"\x89\x50\x4e\x47\x0d\x0a\x1a\x0a\x00\x00\x00\x0d\x49\x48\x44\x52" +
		"\x00\x00\x00\x08\x00\x00\x00\x06\x08\x06\x00\x00\x00\xfe\x05\xdf" +
		"\xfb\x00\x00\x00\x01\x73\x52\x47\x42\x00\xae\xce\x1c\xe9\x00\x00" +
		"\x00\x06\x62\x4b\x47\x44\x00\x00\x00\x00\x00\x00\xf9\x43\xbb\x7f" +
		"\x00\x00\x00\x34\x49\x44\x41\x54\x08\xd7\x85\x8e\x41\x0e\x00\x20" +
		"\x0c\xc2\x28\xff\xff\x33\x9e\x30\x6a\xa2\x72\x21\xa3\x5b\x06\x49" +
		"\xa2\x87\x2c\x49\xc0\x16\xae\xb3\xcf\x8b\xc2\xba\x57\x00\xa8\x1f" +
		"\xeb\x73\xe1\x56\xc5\xfa\x68\x00\x8c\x59\x0d\x11\x87\x39\xe4\xc3" +
		"\x00\x00\x00\x00\x49\x45\x4e\x44\xae\x42\x60\x82")}
var imageTests = []TC{
	{imgr, Image{Format: "png"}, nil},
	{imgr, Image{Format: "png", Width: 8}, nil},
	{imgr, Image{Format: "png", Height: 6}, nil},
	{imgr, Image{Format: "jpg"}, someError},
	{imgr, Image{Format: "png", Width: 12}, someError},
	{imgr, Image{Format: "png", Height: 8}, someError},
}

func TestImage(t *testing.T) {
	for i, tc := range imageTests {
		runTest(t, i, tc)
	}
}

var hcr = response.Response{Body: []byte(`<!doctype html>
<html>
<head><title>CSS Selectors</title></head>
<body>
<h1 id="mt">FooBar</h1>
<p class="X">Hello <span class="X">World</span><p>
<p class="X" id="end">Thanks!</p>
</body>
</html>
`)}
var htmlContainsTests = []TC{
	{hcr, &HTMLContains{Selector: "h1"}, nil},
	{hcr, &HTMLContains{Selector: "p.X", Count: 2}, nil},
	{hcr, &HTMLContains{Selector: "#mt", Count: 1}, nil},
	{hcr, &HTMLContains{Selector: "h2"}, NotFound},
	{hcr, &HTMLContains{Selector: "h1", Count: 2}, someError},
	{hcr, &HTMLContains{Selector: "h1", Count: -1}, FoundForbidden},
	{hcr, &HTMLContains{Selector: "p.z"}, NotFound},
	{hcr, &HTMLContains{Selector: "#nil"}, NotFound},
}

func TestHTMLContains(t *testing.T) {
	for i, tc := range htmlContainsTests {
		runTest(t, i, tc)
	}
}

var htmlContainsTextTests = []TC{
	{hcr, &HTMLContainsText{Selector: "p.X",
		Text: []string{"Hello World", "Thanks!"}}, nil},
	{hcr, &HTMLContainsText{Selector: "#mt",
		Text: []string{"FooBar"}, Complete: true}, nil},
	{hcr, &HTMLContainsText{Selector: "span",
		Text: []string{"World"}}, nil},
	{hcr, &HTMLContainsText{Selector: "span",
		Text: []string{"World"}, Complete: true}, nil},
	{hcr, &HTMLContainsText{Selector: "p.X",
		Text: []string{"Hello World", "FooBar"}}, someError},
	{hcr, &HTMLContainsText{Selector: "p.X",
		Text: []string{"Hello World"}, Complete: true}, someError},
	{hcr, &HTMLContainsText{Selector: "p.X",
		Text: []string{"Hello World", "Thanks!", "ZZZ"}}, someError},
}

func TestHTMLContainsText(t *testing.T) {
	for i, tc := range htmlContainsTextTests {
		runTest(t, i, tc)
	}
}

func TestValidHTML(t *testing.T) {
	/* TODO: find a broken HTML or fix ValidHTML
		broken := response.Response{Body: []byte(`<!doctype html>
	<html>
	<head><ta&&tatat>CS&dsdjhsdkhskdjh;S Se`)}
	*/
	for i, tc := range []TC{
		{hcr, ValidHTML{}, nil},
		// {broken, ValidHTML{}, someError},
	} {
		runTest(t, i, tc)
	}
}

var jr = response.Response{Body: []byte(`{"foo": 5, "bar": [1,2,3]}`)}
var jsonTests = []TC{
	{jr, &JSON{Expression: "(.foo == 5) && ($len(.bar)==3) && (.bar[1]==2)"}, nil},
	{jr, &JSON{Expression: ".foo == 3"}, someError},
}

func TestJSON(t *testing.T) {
	for i, tc := range jsonTests {
		runTest(t, i, tc)
	}
}
