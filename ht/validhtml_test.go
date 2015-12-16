// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"testing"
)

var brokenHtml = `<html>
<head>
    <title>CSS Selectors</title>
</head>
<body lang="de_CH">
    <p id="foo">a < b</p>
    <input type="radio" id="foo" name="radio" />
    <div class="a" class="b">
        World > Country
    </div>
    <a href="/some path">
        Link &fLTzzk;
    </a>
    <label for="other">Label:</label>
    <ul>
        <li>
            <a href="http:://example.org:3456/">Home</a>
        </li>
        <li>
            <img src="/image?n=1&p=2" />
        </div>
    </li>
</body>
</html>
`

var expectedErrorsInBrokenHtml = []string{
	"line 6: unescaped '<'",
	"line 7: duplicate id 'foo'",
	"line 8: duplicate attribute 'class'",
	"line 10: unescaped '>'",
	"line 11: URL path '/some path' should be '/some%20path'",
	"line 13: unescaped '&' or unknow entity",
	"line 17: bad URL part '://example.org:3456/'",
	"line 20: unescaped '&' or unknow entity",
	"line 21: tag 'li' closed by 'div'",
	"line 25: found 0 DOCTYPE",
	"line 25: label references unknown id 'other'",
}

var okayHtml = `<!DOCTYPE html><html>
<head>
    <title>CSS Selectors</title>
</head>
<body lang="de-CH">
    <p id="foo">a &lt; b</p>
    <input type="radio" id="waz" name="radio" />
    <div class="a b">
        World &gt; Country
    </div>
    <a href="/some%20path">
        Link &copy;
    </a>
    <label for="waz">Label:</label>
    <ul>
        <li>
            <a href="http://example.org:3456/">Home</a>
        </li>
        <li>
            <img src="/image?n=1&amp;p=2" />
        </li>
    </ul>
</body>
</html>
`

var brokenHtmlResponse = Response{
	BodyStr: brokenHtml,
}

func TestValidHTMLBroken(t *testing.T) {
	test := &Test{
		Response:  Response{BodyStr: brokenHtml},
		Verbosity: 1,
	}

	check := ValidHTML{}
	err := check.Prepare()
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	err = check.Execute(test)
	el, ok := err.(ErrorList)
	if !ok {
		t.Fatalf("Unexpected type %T of error %#v", err, err)
	}

	es := el.AsStrings()
	for i, e := range es {
		if testing.Verbose() {
			fmt.Println(e)
		}
		if i >= len(expectedErrorsInBrokenHtml) {
			t.Errorf("Unexpected extra error: %s", e)
		} else if want := expectedErrorsInBrokenHtml[i]; e != want {
			t.Errorf("Expected %s, got %s", e, want)
		}
	}
}

func TestValidHTMLOkay(t *testing.T) {
	test := &Test{
		Response:  Response{BodyStr: okayHtml},
		Verbosity: 1,
	}

	check := ValidHTML{}
	err := check.Prepare()
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	err = check.Execute(test)
	if err != nil {
		t.Fatalf("Unexpected error: %#v\n%s", err, err)
	}

}

func TestCheckHTMLEscaping(t *testing.T) {
	testcases := []struct {
		raw  string
		okay bool
	}{
		{"", true},
		{"foo", true},
		{"1 &lt; 2", true},
		{"Hund &amp; Katz", true},
		{"&copy; 2009", true},
		{"A & B", false},
		{"A < B", false},
		{"x &fZtU4w; y", false},
	}

	for i, tc := range testcases {
		state := newHtmlState(tc.raw)
		state.checkEscaping(tc.raw)
		if tc.okay && len(state.errors) > 0 {
			t.Errorf("%d. %q: Unexpected error %s", i, tc.raw, state.errors[0])
		} else if !tc.okay && len(state.errors) == 0 {
			t.Errorf("%d. %q: Missing error", i, tc.raw)
		}
		return
	}
}
