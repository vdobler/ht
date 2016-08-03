// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/vdobler/ht/internal/json5"
)

var exampleHTML = `
<html>
  <head>
    <meta http-equiv="content-type" content="text/html; charset=UTF-8" />
    <meta name="_csrf" content="18f0ca3f-a50a-437f-9bd1-15c0caa28413" />
    <title>Dummy HTML</title>
  </head>
  <body>
    <h1>Headline</h1>
    <div class="token"><span>
	DEAD-BEEF-0007

</span></div>
  </body>
</html>`

func TestHTMLExtractor(t *testing.T) {
	test := &Test{
		Response: Response{
			BodyStr: exampleHTML,
		},
	}

	ex := HTMLExtractor{
		Selector:  `head meta[name="_csrf"]`,
		Attribute: `content`,
	}

	val, err := ex.Extract(test)
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	} else if val != "18f0ca3f-a50a-437f-9bd1-15c0caa28413" {
		t.Errorf("Got %q, want 18f0ca3f-a50a-437f-9bd1-15c0caa28413", val)
	}

	ex = HTMLExtractor{
		Selector:  `body div.token > span`,
		Attribute: `~text~`,
	}
	val, err = ex.Extract(test)
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	} else if val != "DEAD-BEEF-0007" {
		t.Errorf("Got %q, want DEAD-BEEF-0007", val)
	}

}

func TestBodyExtractor(t *testing.T) {
	test := &Test{
		Response: Response{
			BodyStr: "Hello World! Foo 123 xyz ABC. Dog and cat.",
		},
	}

	ex := BodyExtractor{
		Regexp: "([1-9]+) (...) ([^ .]*)",
	}

	val, err := ex.Extract(test)
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	} else if val != "123 xyz ABC" {
		t.Errorf("Got %q, want 123 xyz ABC", val)
	}

	ex.Submatch = 2
	val, err = ex.Extract(test)
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	} else if val != "xyz" {
		t.Errorf("Got %q, want xyz", val)
	}

	test.Response.BodyStr = "blablabla"
	_, err = ex.Extract(test)
	if err == nil || err.Error() != `no match found in "blablabla"` {
		t.Errorf("Missing or wrong error: %v", err)
	}
}

var jsonExtractorTests = []struct {
	body string
	path string
	want string
	err  error
}{
	{`{"a":"foo", "b":"bar", "c": [1,2,3]}`, "a", "foo", nil},
	{`{"a":"foo", "b":"bar", "c": [1,2,3]}`, "b", "bar", nil},
	{`{"a":"foo", "b":"bar", "c": [1,2,3]}`, "c.2", "3", nil},
}

func TestJSONExtractor(t *testing.T) {
	for i, tc := range jsonExtractorTests {
		test := &Test{
			Response: Response{
				BodyStr: tc.body,
			},
		}
		ex := JSONExtractor{Path: tc.path}
		got, err := ex.Extract(test)
		if err != nil {
			if tc.err == nil {
				t.Errorf("%d. Path=%q: unexpected error %v",
					i, tc.path, err)
				continue
			}
			continue // TODO check type and message
		}
		if got != tc.want {
			t.Errorf("%d. Path=%q: got %q, want %q",
				i, tc.path, got, tc.want)
		}
	}
}

var cookieExtractorTests = []struct {
	name string
	want string
	err  error
}{
	{"sessionid", "123abc456", nil},
	{"missing", "", fmt.Errorf("cookie missing not received")},
	{"foo", "bar", nil},
}

func TestCookieExtractor(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"Set-Cookie": []string{
				"foo=bar",
				"sessionid=123abc456",
				"foo=wuz",
			},
		},
	}

	for i, tc := range cookieExtractorTests {
		test := &Test{
			Response: Response{Response: resp},
		}
		ex := CookieExtractor{Name: tc.name}
		got, err := ex.Extract(test)

		if tc.err != nil && err == nil {
			t.Errorf("%d. name=%s, missing error, want %s",
				i, tc.name, tc.err)
			continue
		}
		if tc.err == nil && err != nil {
			t.Errorf("%d. name=%s, unexpected error, got %s",
				i, tc.name, err)
			continue
		}
		if tc.err != nil && err != nil {
			if tc.err.Error() != err.Error() {
				t.Errorf("%d. name=%s, wrong error, got %s, want %s",
					i, tc.name, err, tc.err)
			}
			continue
		}
		if got != tc.want {
			t.Errorf("%d. name=%s, got %s, want %s",
				i, tc.name, got, tc.want)
		}
	}
}

var jsExtractorTests = []struct {
	script string
	want   string
	error  string
}{
	// Simple stuff, passing.
	{`123;`, "123", ""},
	{`true;`, "true", ""},
	{`false;`, "false", ""},
	{`"abc";`, "abc", ""},
	{`Test.Name;`, "JavaScript everywhere", ""},
	{`["Ooops"];`, "", "Ooops"},

	// Simple stuff, failing.
	{`null;`, "", "null"},
	{`var a; a;`, "", "undefined"},

	// Reporting errors; ugly but works.
	{`var err={"errmsg": "I'm sorry"}; err;`, "", "I'm sorry"},
	{`[ "I'm sorry" ];`, "", "I'm sorry"},
	{`var f = function() { return 7; }; f;`, "", "extracted Function"},

	// Complex stuff
	{`
             var body = JSON.parse(Test.Response.BodyStr);
             var zug = _.find(body, function(k){ return k.code=="ZG"; });
             if ( zug === undefined ) {
                 var err={"error": "Did not find ZG"};
                 err;
             } else {
                 zug.id;
             }
            `, "78", "",
	},
	{`
             var body = JSON.parse(Test.Response.BodyStr);
             var zug = _.find(body, function(k){ return k.code=="SO"; });
             if ( zug === undefined ) {
                 [ "Did not find SO" ];  // Array signal error too.
             } else {
                 zug.id;
             }
            `, "", "Did not find SO",
	},
}

func TestJSExtractor(t *testing.T) {
	body := `[
  { "id": 12, "code": "AG", "name": "Aargau" },
  { "id": 34, "code": "BE", "name": "Bern" },
  { "id": 56, "code": "ZH", "name": "Zürich" },
  { "id": 78, "code": "ZG", "name": "Zug" },
  { "id": 90, "code": "GE", "name": "Genf" }
]`

	for i, tc := range jsExtractorTests {
		test := &Test{
			Name: "JavaScript everywhere",
			Response: Response{
				BodyStr: body,
			},
		}
		ex := JSExtractor{Script: tc.script}
		got, err := ex.Extract(test)
		if err != nil {
			if tc.error == "" {
				t.Errorf("%d. Unexpected error %v", i, err)
			} else if egot := err.Error(); tc.error != egot {
				t.Errorf("%d. Wrong error '%s', want '%s'", i, egot, tc.error)
			}
			continue
		}
		if got != tc.want {
			t.Errorf("%d. got %q, want %q", i, got, tc.want)
		}
	}
}

func TestMarshalExtractorMap(t *testing.T) {
	em := ExtractorMap{
		"Foo": HTMLExtractor{
			Selector:  "div.footer p.copyright span.year",
			Attribute: "~text~",
		},
		"Bar": BodyExtractor{
			Regexp:   "[A-Z]+[0-9]+",
			Submatch: 1,
		},
	}

	out, err := em.MarshalJSON()
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	buf := &bytes.Buffer{}
	err = json5.Indent(buf, out, "", "    ")
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	fooExpected := `
    Foo: {
        Extractor: "HTMLExtractor",
        Selector: "div.footer p.copyright span.year",
        Attribute: "~text~"
    }`

	barExpected := `
    Bar: {
        Extractor: "BodyExtractor",
        Regexp: "[A-Z]+[0-9]+",
        Submatch: 1
    }`
	if s := buf.String(); !strings.Contains(s, fooExpected) || !strings.Contains(s, barExpected) {

		t.Errorf("Wrong JSON, got:\n%s", s)
	}
}

func TestUnmarshalExtractorMap(t *testing.T) {
	j := []byte(`{
    Foo: {
        Extractor: "HTMLExtractor",
        Selector: "form input[type=password]",
        Attribute: "value"
    },
    Bar: {
        Extractor: "BodyExtractor",
        Regexp: "[A-Z]+[0-9]*[g-p]",
        Submatch: 3
    }
}`)

	em := ExtractorMap{}
	err := (&em).UnmarshalJSON(j)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	if len(em) != 2 {
		t.Fatalf("Wrong len, got %d\n%#v", len(em), em)
	}

	if foo, ok := em["Foo"]; !ok {
		t.Errorf("missing Foo")
	} else {
		if htmlex, ok := foo.(*HTMLExtractor); !ok { // TODO: Why pointere here?
			t.Errorf("wrong type for foo. %#v", foo)
		} else {
			if htmlex.Selector != "form input[type=password]" {
				t.Errorf("HTMLElementSelector = %q", htmlex.Selector)
			}
			if htmlex.Attribute != "value" {
				t.Errorf("HTMLElementAttribte = %q", htmlex.Attribute)
			}
		}
	}

	if bar, ok := em["Bar"]; !ok {
		t.Errorf("missing Bar")
	} else {
		if bodyex, ok := bar.(*BodyExtractor); !ok { // TODO: Why pointere here?
			t.Errorf("wrong type for bar. %#v", bar)
		} else {
			if bodyex.Regexp != "[A-Z]+[0-9]*[g-p]" {
				t.Errorf("Regexp = %q", bodyex.Regexp)
			}
			if bodyex.Submatch != 3 {
				t.Errorf("Submatch = %d", bodyex.Submatch)
			}
		}
	}

}
