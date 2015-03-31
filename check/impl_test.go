// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package check

import (
	_ "image/jpeg"
	_ "image/png"
	"testing"

	"github.com/vdobler/ht/response"
)

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
