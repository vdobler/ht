// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

var hcr = Response{
	BodyBytes: []byte(`<!doctype html>
<html>
<link href="/css/base.css">
<head><title>CSS Selectors</title></head>
<body>
<h1 id="mt">FooBar</h1>
<p class="X">Hello <span class="X">World</span><p>
<p class="X" id="end">Thanks!</p>
<a href="#">Link1</a>
<a href="/foo/bar">Link2</a>
<a href="../waz#top">Link3</a>
<a href="http://www.google.com">Link4</a>
<img src="pic.jpg"><img src="http://www.google.com/logo.png">
<script src="/js/common.js"></script>
<script>blob="aaa"</script>
<div class="WS">
  <p class="em">Inter<em>word</em>emphasis</p>
  <p class="strong">
	Some
	<strong> important </strong>
	things.
  </p>
  <ul class="items"><li>Foo</li><li>Bar</li><li>Waz</li></ul>
  <ul class="fancy"><li>One</li><li>S<strong>econ</strong>d</li><li> Three </li></ul>
</div>
</body>
</html>
`)}

var htmlContainsTests = []TC{
	{hcr, &HTMLTag{Selector: "h1"}, nil},
	{hcr, &HTMLTag{Selector: "p.X", Count: 2}, nil},
	{hcr, &HTMLTag{Selector: "#mt", Count: 1}, nil},
	{hcr, &HTMLTag{Selector: "h2"}, NotFound},
	{hcr, &HTMLTag{Selector: "h1", Count: 2}, someError},
	{hcr, &HTMLTag{Selector: "h1", Count: -1}, FoundForbidden},
	{hcr, &HTMLTag{Selector: "p.z"}, NotFound},
	{hcr, &HTMLTag{Selector: "#nil"}, NotFound},
}

func TestHTMLTag(t *testing.T) {
	for i, tc := range htmlContainsTests {
		runTest(t, i, tc)
	}
}

var htmlContainsTextTests = []TC{
	{hcr, &HTMLContains{Selector: "p.X",
		Text: []string{"Hello World", "Thanks!"}}, nil},
	{hcr, &HTMLContains{Selector: "#mt",
		Text: []string{"FooBar"}, Complete: true}, nil},
	{hcr, &HTMLContains{Selector: "span",
		Text: []string{"World"}}, nil},
	{hcr, &HTMLContains{Selector: "span",
		Text: []string{"World"}, Complete: true}, nil},
	{hcr, &HTMLContains{Selector: "p.X",
		Text: []string{"Hello World", "FooBar"}}, someError},
	{hcr, &HTMLContains{Selector: "p.X",
		Text: []string{"Hello World"}, Complete: true}, someError},
	{hcr, &HTMLContains{Selector: "p.X",
		Text: []string{"Hello World", "Thanks!", "ZZZ"}}, someError},
	{hcr, &HTMLContains{Selector: "div.WS p.em",
		Text: []string{"Interwordemphasis"}}, nil},
	{hcr, &HTMLContains{Selector: "div.WS p.strong",
		Text: []string{"Some important things."}}, nil},
	{hcr, &HTMLContains{Selector: "ul.items",
		Text: []string{"Foo Bar Waz"}}, nil},
	{hcr, &HTMLContains{Selector: "ul.fancy",
		Text: []string{"One Second Three"}}, nil},
}

func TestHTMLContains(t *testing.T) {
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

func TestW3CValidHTML(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping W3C Validator based checks in short mode.")
	}

	body := `<!DOCTYPE html>
<html>
<head>
  <meta http-equiv="content-type" content="text/html; charset=UTF-8" />
  <title>This is okay</title>
</head>
<body>
  <h1>Here all good &amp; nice</h1>
</body>`

	rr := Response{BodyBytes: []byte(body)}
	check := W3CValidHTML{
		AllowedErrors: 0,
	}
	runTest(t, 0, TC{rr, check, nil})

	body2 := `<!DOCTYPE html>
<html>
<head>
  <meta http-equiv="content-type" content="text/html; charset=UTF-8" />
  <title>This is okay</title>
</head>
<body>
  <h1 title="K&K">Here some issues problems</h1>
  <button role="presentation">Button</button>
  <span><div>Strangly nested</div></span>
</body>`

	rr2 := Response{BodyBytes: []byte(body2)}
	check2 := W3CValidHTML{
		AllowedErrors: 1,
		IgnoredErrors: []Condition{
			{Prefix: "& did not start a character reference"},
		},
	}
	runTest(t, 1, TC{rr2, check2, someError})

	check3 := W3CValidHTML{
		AllowedErrors: 3,
	}
	runTest(t, 1, TC{rr2, check3, nil})

}

func htmlLinksHandler(w http.ResponseWriter, r *http.Request) {
	status := 200
	if strings.Index(r.URL.Path, "/404/") != -1 {
		status = 404
	} else if strings.Index(r.URL.Path, "/302/") != -1 {
		status = 302
	}
	linksHandlerCalls <- r.Host + r.URL.String()
	http.Error(w, "Link Handler", status)
}

var linksHandlerCalls chan string

func TestHTMLLinksExtraction(t *testing.T) {
	body := `<!doctype html>
<html>
<head>
  <title>CSS Selectors</title>
  <link rel="copyright" title="Copyright" href="/impressum.html#top" />
  <script type="text/javascript" src="/js/jquery.js"></script>
</head>
<body>
  <a href="/path/link4">Link4</a>
  <img src="/some/image.gif">
  <a href="/path/link4#nav">Link4</a>
  <a href="http://www.google.com">Google</a>
  <a href="rel/path">Page</a>
  <img src="http://www.amazon.com/logo.png">
</body>
</html>`

	baseURL, err := url.Parse("http://www.example.org/foo/bar.html")
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	test := &Test{
		Request: Request{
			Request: &http.Request{URL: baseURL},
		},
		Response: Response{BodyBytes: []byte(body)},
	}

	for i, tc := range []struct{ which, want string }{
		{"img", "http://www.example.org/some/image.gif http://www.amazon.com/logo.png"},
		{"link", "http://www.example.org/impressum.html"},
		{"a", "http://www.example.org/path/link4 http://www.google.com http://www.example.org/foo/rel/path"},
		{"script", "http://www.example.org/js/jquery.js"},
	} {

		check := Links{Which: tc.which}
		err = check.Prepare()
		if err != nil {
			t.Fatalf("%d: unexpected error: %#v", i, err)
		}
		urls, err := check.collectURLs(test)
		if err != nil {
			t.Fatalf("%d: Unexpected error: %#v", i, err)
		}
		expectedURLs := strings.Split(tc.want, " ")
		for _, expected := range expectedURLs {
			if _, ok := urls[expected]; !ok {
				t.Errorf("%d: Missing expected URL %q", i, expected)
			}
		}
		if len(urls) > len(expectedURLs) {
			t.Errorf("%d: Extracted too many URLs: Want %d, got %v",
				i, len(expectedURLs), urls)
		}
	}
}

func testHTMLLinks(t *testing.T, urls []string) (called []string, err error) {
	ts1 := httptest.NewServer(http.HandlerFunc(htmlLinksHandler))
	defer ts1.Close()
	ts2 := httptest.NewServer(http.HandlerFunc(htmlLinksHandler))
	defer ts2.Close()

	body := fmt.Sprintf(`<!doctype html>
<html>
<head>
  <title>CSS Selectors</title>
  <link rel="copyright" title="Copyright" href="%s#top" />
  <script type="text/javascript" src="%s"></script>
</head>
<body>
  <a href="%s">Link4</a>
  <img src="%s">
  <a href="%s#nav">Link5</a>
  <a href="%s">LinkWAZ</a>
</body>
</html>`, urls[0], urls[1], ts1.URL+urls[2], ts1.URL+urls[3], ts1.URL+urls[4], ts2.URL+urls[5])

	baseURL, err := url.Parse(ts1.URL)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	test := &Test{
		Request:   Request{Request: &http.Request{URL: baseURL}},
		Response:  Response{BodyBytes: []byte(body)},
		Verbosity: 1,
	}

	check := Links{Which: "a img link script", Concurrency: 2}
	err = check.Prepare()
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	linksHandlerCalls = make(chan string, 10)
	err = check.Execute(test)
	close(linksHandlerCalls)

	for c := range linksHandlerCalls {
		called = append(called, c)
	}

	return called, err
}

func TestHTMLLinksOkay(t *testing.T) {
	urls := []string{
		"/impressum.html",
		"/js/jquery.js",
		"/foo",
		"/supertoll/bild.gif",
		"/foo",
		"/waz",
	}
	called, err := testHTMLLinks(t, urls)
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	}
	if len(called) != 5 {
		t.Errorf("Unexpected error: %v", called)
	}
}

func TestHTMLLinksBroken(t *testing.T) {
	urls := []string{
		"/404/impressum.html",
		"/404/js/jquery.js",
		"/404/foo",
		"/404/supertoll/bild.gif",
		"/404/foo",
		"/404/waz",
	}
	called, err := testHTMLLinks(t, urls)
	if err == nil {
		t.Fatalf("Missing error: %#v", err)
	}
	if len(called) != 5 {
		t.Errorf("Unexpected error: %v", called)
	}
}
