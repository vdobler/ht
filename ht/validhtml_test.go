// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"strings"
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
    <a href="mailto:info-example_org">write us</a>
    <a href="tel:+(0012)-345 67/89">call us</a>
    <p data-selector="h1 > span"></p>
    <p title="A&glubs;B"></p>
</body>
</html>
`

var expectedErrorsInBrokenHTML = []struct {
	typ, err string
}{
	{"escaping", "Line 6: Unescaped '<'"},
	{"uniqueids", "Line 7: Duplicate id 'foo'"},
	{"attr", "Line 8: Duplicate attribute 'class'"},
	{"escaping", "Line 10: Unescaped '>'"},
	{"url", "Line 11: Unencoded space in URL"},
	{"escaping", "Line 13: Unescaped '&' or unknow entity in &fLTzzk"},
	{"url", "Line 17: Bad URL part '://example.org:3456/'"},
	{"structure", "Line 21: Tag 'li' closed by 'div'"},
	{"url", "Line 23: Not an email address"},
	{"url", "Line 24: Not a telephone number"},
	{"escaping", "Line 26: Unknown entity &glubs;"},
	{"doctype", "Line 29: Found 0 DOCTYPE"},
	{"label", "Line 29: Label references unknown id 'other'"},
}

func TestValidHTMLBroken(t *testing.T) {
	test := &Test{
		Response: Response{BodyStr: brokenHTML},
	}

	for _, ignore := range []string{"", "doctype", "structure", "uniqueids", "lang", "attr", "escaping", "label", "url"} {
		t.Run("ignore="+ignore, func(t *testing.T) {
			check := ValidHTML{Ignore: ignore}
			err := check.Prepare(test)
			if err != nil {
				t.Fatalf("Unexpected error: %#v", err)
			}

			err = check.Execute(test)
			el, ok := err.(ErrorList)
			if !ok {
				t.Fatalf("Unexpected type %T of error %#v", err, err)
			}

			es := el.AsStrings()
			var got string
			isIgnored := func(t string) bool {
				if ignore == "" {
					return false
				}
				for _, toIgnore := range strings.Split(t, " ") {
					if toIgnore == ignore {
						return true
					}
				}
				return false
			}
			for _, expect := range expectedErrorsInBrokenHTML {
				if isIgnored(expect.typ) {
					continue
				}
				if len(es) == 0 {
					t.Errorf("Ignoring %s: missing error %s", ignore, expect.err)
					continue
				}
				got, es = es[0], es[1:]
				if got != expect.err {
					t.Errorf("Ignore %s: Got %q, want %q", ignore, got, expect.err)
				}
			}
			if len(es) != 0 {
				t.Errorf("Ignore %s: unexpected errors %v", ignore, es)
			}
		})
	}
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
    <span data-css: "h1 &gt; span"></span>
    <a href="mailto:info@example.org">write us</a>
    <a href="tel:+0012-345-6789">call us</a>
    <script>
       if(a<b && c!="") { consol.log("foo"); }
    </script>
</body>
</html>
`

func TestValidHTMLOkay(t *testing.T) {
	test := &Test{
		Response: Response{BodyStr: okayHTML},
	}

	check := ValidHTML{}
	err := check.Prepare(test)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	err = check.Execute(test)
	if err != nil {
		if el, ok := err.(ErrorList); !ok {
			t.Fatalf("Unexpected error: %#v\n%s", err, err)
		} else {
			t.Fatalf("Unexpected error: %ss", el.AsStrings())
		}
	}

}

func TestCheckAttributeEscaping(t *testing.T) {
	for _, tc := range []struct {
		name, in, msg string
	}{
		{"no-amp", `p class="foo"`, ""},
		{"trailing", `p class="foo&"`, ""},
		{"no-ascii", `p class="foo&--;"`, ""},
		{"no-semicolon", `p class="foo&circ"`, ""},
		{"several", `p class="foo&circ&circ-a;&t=a;"`, ""},
		{"proper-amp", `p title="foo&amp;bar"`, ""},
		{"proper-circ", `p title="e&circ;2pi"`, ""},
		{"bad1", `a href="/foo&circa;bar"`, "Unknown entity &circa;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &htmlState{}
			s.checkAttributeEscaping(tc.in)
			if len(s.errors) > 1 {
				t.Fatalf("Too many errors %#v", s.errors)
			}
			got := ""
			if len(s.errors) > 0 {
				got = s.errors[0].Error()
			}
			if got != tc.msg {
				t.Errorf("Got %q, want %q", got, tc.msg)
			}
		})
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
		{"mailto:info.example-org", "Not an email address"},
		{"/with space", "Unencoded space in URL"},
		{"tel:+41123456778", ""},
		{"tel:+41-12-345-67-78", ""},
		{"tel:004112345678", "Telephone numbers must start with +"},
		{"tel:+", "Missing actual telephone number"},
		{"tel:+++ticker+++", "Not a telephone number"},
	}

	for i, tc := range testcases {
		test := &Test{Response: Response{
			BodyStr: fmt.Sprintf(`<!DOCTYPE html><html><body><a href="%s" /></body></html>`, tc.href),
		}}

		check := ValidHTML{}
		err := check.Prepare(test)
		if err != nil {
			t.Fatalf("Unexpected error: %#v", err)
		}

		err = check.Execute(test)
		if err == nil && tc.err != "" {
			t.Errorf("%d. %q: want error %s", i, tc.href, tc.err)
		} else if err != nil {
			if tc.err == "" {
				t.Errorf("%d. %q: unexpected error %s", i, tc.href, err)
			} else if got := err.(ErrorList)[0].Error(); got != "Line 1: "+tc.err {
				t.Errorf("%d. %q: got %q; want %q", i, tc.href, got, tc.err)
			}
		}
	}
}
