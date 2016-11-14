// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"testing"
)

var brokenHTML = `<html>
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
    <a href="mailto:info-example_org>write us</a>
</body>
</html>
`

var expectedErrorsInBrokenHTML = []string{
	"line 6: unescaped '<'",
	"line 7: duplicate id 'foo'",
	"line 8: duplicate attribute 'class'",
	"line 10: unescaped '>'",
	"line 11: unencoded space in URL",
	"line 13: unescaped '&' or unknow entity",
	"line 17: bad URL part '://example.org:3456/'",
	"line 20: unescaped '&' or unknow entity",
	"line 21: tag 'li' closed by 'div'",
	"line 26: found 0 DOCTYPE",
	"line 26: label references unknown id 'other'",
}

var okayHTML = `<!DOCTYPE html><html>
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
    <a href="mailto:info@example.org">write us</a>
    <script>
       if(a<b && c!="") { consol.log("foo"); }
    </script>
</body>
</html>
`

var brokenHTMLResponse = Response{
	BodyStr: brokenHTML,
}

func TestValidHTMLBroken(t *testing.T) {
	test := &Test{
		Response: Response{BodyStr: brokenHTML},
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
		if *verboseTest {
			fmt.Println(e)
		}
		if i >= len(expectedErrorsInBrokenHTML) {
			t.Errorf("Unexpected extra error: %s", e)
		} else if want := expectedErrorsInBrokenHTML[i]; e != want {
			t.Errorf("Got %s, want %s", e, want)
		}
	}
}

func TestValidHTMLOkay(t *testing.T) {
	test := &Test{
		Response: Response{BodyStr: okayHTML},
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
		state := newHTMLState(tc.raw, issueIgnoreNone)
		state.checkEscaping(tc.raw)
		if tc.okay && len(state.errors) > 0 {
			t.Errorf("%d. %q: Unexpected error %s", i, tc.raw, state.errors[0])
		} else if !tc.okay && len(state.errors) == 0 {
			t.Errorf("%d. %q: Missing error", i, tc.raw)
		}
	}
}

func TestURLEscaping(t *testing.T) {
	testcases := []struct {
		href string
		err  string
	}{
		{"/all%20good", ""},
		{"http://www.example.org/foo/bar?a=12", ""},
		{"mailto:info@example.org", ""},
		{"mailto:info.example-org", "not an email address"},
		{"/with space", "unencoded space in URL"},
	}

	for i, tc := range testcases {
		test := &Test{Response: Response{
			BodyStr: fmt.Sprintf(`<!DOCTYPE html><html><body><a href="%s" /></body></html>`, tc.href),
		}}

		check := ValidHTML{}
		err := check.Prepare()
		if err != nil {
			t.Fatalf("Unexpected error: %#v", err)
		}

		err = check.Execute(test)
		if err == nil && tc.err != "" {
			t.Errorf("%d. %q: want error %s", i, tc.href, tc.err)
		} else if err != nil {
			if tc.err == "" {
				t.Errorf("%d. %q: unexpected error %s", i, tc.href, err)
			} else if got := err.(ErrorList)[0].Error(); got != "line 1: "+tc.err {
				t.Errorf("%d. %q: got %q; want %q", i, tc.href, got, tc.err)
			}
		}
	}
}
