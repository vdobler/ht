// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"net/http"
	"net/url"
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

func TestHTMLLinks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTMLLinks test in short mode.")
	}
	// TODO: make a test which doesn't require network

	bodyOkay := `<!doctype html>
<html>
<head>
  <title>CSS Selectors</title>
  <link rel="copyright" title="Copyright" href="/impressum.html" />
  <script type="text/javascript" src="/js/jquery/jquery-1.7.1.min.js"></script>
</head>
<body>
  <img src="//www.heise.de/icons/ho/heise_online_logo_top.gif">
  <a href="http://www.google.com">Link4</a>
</body>
</html>`

	baseURL, err := url.Parse("http://www.heise.de")
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	test := &Test{
		Request: Request{
			Request: &http.Request{
				URL: baseURL,
			},
		},
		Response: Response{
			BodyBytes: []byte(bodyOkay),
		},
		Verbosity: 1,
	}

	checkA := Links{Which: "a img link script", Concurrency: 2}
	err = checkA.Prepare()
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	err = checkA.Execute(test)
	if err != nil {
		t.Errorf("Unexpected errors %#v", err)
	}

	// Now all links broken
	bodyBad := `<!doctype html>
<html>
<head>
  <title>CSS Selectors</title>
  <link rel="copyright" title="Copyright" href="/impressum404.html" />
  <script type="text/javascript" src="/js/jquery/jquery-9.9.9.min.js"></script>
</head>
<body>
  <img src="//www.heise.de/icons/ho/heise_online_logo_todownother.gif">
  <a href="http://www.google.com/fobbar">Link4</a>
</body>
</html>`
	test.Response.BodyBytes = []byte(bodyBad)
	err = checkA.Prepare()
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	err = checkA.Execute(test)
	if err == nil {
		t.Fatalf("Expected errors")
	}

	if err.Error() != `http://www.google.com/fobbar  -->  404
http://www.heise.de/icons/ho/heise_online_logo_todownother.gif  -->  404
http://www.heise.de/impressum404.html  -->  404
http://www.heise.de/js/jquery/jquery-9.9.9.min.js  -->  404` {
		t.Errorf("Got wrong error:\n%s", err.Error())
	}

}
