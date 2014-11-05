// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/kr/pretty"
	"github.com/vdobler/ht/check"
)

func TestStatusCode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(echoHandler))
	defer ts.Close()

	for _, code := range []int{200, 201, 204, 300, 400, 404, 500} {
		s := fmt.Sprintf("%d", code)
		test := Test{
			Request: Request{
				Method:          "GET",
				URL:             ts.URL + "/",
				Params:          url.Values{"status": []string{s}},
				FollowRedirects: false,
			},
			Checks: []check.Check{
				check.StatusCode{Expect: code},
			},
		}

		result := test.Run(nil)
		if result.Status != Pass {
			t.Errorf("Unexpected error for %d: %s", code, result.Error)
		}
	}
}

func TestParameterHandling(t *testing.T) {
	test := Test{Request: Request{
		Method: "POST",
		URL:    "http://www.test.org",
		Params: url.Values{
			"single":  []string{"abc"},
			"multi":   []string{"1", "2"},
			"special": []string{"A%+& &?/Z"},
			"file":    []string{"@file:testdata/upload.pdf"},
		},
		ParamsAs: "URL",
	}}

	// As part of the URL.
	err := test.prepare(nil)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
	if test.Request.Body != "" {
		t.Errorf("Expected empty body, got %q", test.Request.Body)
	}
	if test.request.URL.String() != "http://www.test.org?file=%40file%3Atestdata%2Fupload.pdf&multi=1&multi=2&single=abc&special=A%25%2B%26+%26%3F%2FZ" {
		t.Errorf("Bad URL, got %s", test.Request.URL)
	}
	test.Request.Body = ""

	// URLencoded in the body.
	test.Request.ParamsAs = "body"
	err = test.prepare(nil)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
	full, err := ioutil.ReadAll(test.request.Body)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
	if string(full) != "file=%40file%3Atestdata%2Fupload.pdf&multi=1&multi=2&single=abc&special=A%25%2B%26+%26%3F%2FZ" {
		t.Errorf("Bad body, got %s", full)
	}
	test.Request.Body = ""

	test.Request.ParamsAs = "multipart"
	err = test.prepare(nil)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
	ct := test.request.Header.Get("Content-Type")
	mt, p, err := mime.ParseMediaType(ct)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
	if mt != "multipart/form-data" {
		t.Fatalf("Unexpected multipart/form-data, got %s", mt)
	}
	boundary, ok := p["boundary"]
	if !ok {
		t.Fatalf("No boundary in content type")
	}
	r := multipart.NewReader(test.request.Body, boundary)
	f, err := r.ReadForm(1 << 10)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
	if len(f.Value) != 3 || len(f.Value["single"]) != 1 || len(f.Value["multi"]) != 2 {
		t.Errorf(pretty.Sprintf("Got\n%# v\n", f))
	}
}

func TestRTStats(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(echoHandler))
	defer ts.Close()

	test := &Test{
		Name: "Sleep {{SMIN}}-{{SMAX}}",
		Request: Request{
			Method: "GET",
			URL:    ts.URL + "/",
			Params: url.Values{
				"smin": []string{"{{SMIN}}"},
				"smax": []string{"{{SMAX}}"},
				"fail": {"5"},
			},
			FollowRedirects: false,
		},
		Checks: []check.Check{
			check.StatusCode{200},
		},
		Timeout: 150 * time.Millisecond,
	}

	rtimes := map[string][]string{
		"SMIN": []string{"5", "30", "50"},
		"SMAX": []string{"20", "70", "100"},
	}
	tests, _ := Repeat(test, 3, rtimes)

	suite := &Suite{
		Name:        "Response Time Statistics",
		Tests:       tests,
		KeepCookies: true,
	}

	err := suite.Compile()
	if err != nil || len(suite.Tests) != 3 {
		t.Fatalf("Unexpected error: %v %d", err, len(suite.Tests))
	}

}

// ----------------------------------------------------------------------------
// Test Handlers

// redirectHandler called with a path of /<n> will redirect to /<n-1> if n>0.
// A path of /0 prints "Hello World" and any other path results in a 500 error.
func redirectHandler(w http.ResponseWriter, r *http.Request) {
	np := r.URL.Path[1:]
	n, err := strconv.Atoi(np)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if n > 0 {
		u := r.URL
		u.Path = fmt.Sprintf("/%d", n-1)
		http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
		return
	}
	http.Error(w, "Hello World!", http.StatusOK)
}

// parse the form value name as an int, defaulting to 0.
func intFormValue(r *http.Request, name string) int {
	s := r.FormValue(name)
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// echoHandler answers the request based on the parameters status (HTTP status
// code), text (the response body) and header and value (any HTTP header).
// The handler sleeps for a random duration beteen smin and smax milliseconds.
// If echoHandler is called with parameter fail it timeout with the given
// probability. The parameter bad controls the probability if bad responses.
func echoHandler(w http.ResponseWriter, r *http.Request) {
	fail := intFormValue(r, "fail")
	if fail > 0 {
		k := rand.Intn(100)
		if k < fail {
			time.Sleep(10 * time.Second)
			// panic("Fail")
		}
	}
	status := intFormValue(r, "status")
	if status == 0 {
		status = 200
	}
	smin, smax := intFormValue(r, "smin"), intFormValue(r, "smax")
	if smin > 0 {
		if smax <= smin {
			smax = smin + 1
		}
		sleep := rand.Intn(1000*(smax-smin)) + 1000*smin // in microseconds
		time.Sleep(1000 * time.Duration(sleep))          // now in nanoseconds
	}
	header, value := r.FormValue("header"), r.FormValue("value")
	if header != "" {
		w.Header().Set(header, value)
	}
	text := r.FormValue("text")
	bad := intFormValue(r, "bad")
	if bad > 0 {
		k := rand.Intn(100)
		if k < bad {
			text, status = "XXXXXXX", 500
		}
	}

	http.Error(w, text, status)
}

// cookieHandler
func cookieHandler(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	value := r.FormValue("value")
	httpOnly := r.FormValue("httponly") != ""
	maxAge := intFormValue(r, "maxage")
	if name != "" {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: value,
			HttpOnly: httpOnly,
			MaxAge:   maxAge,
		})
	}

	fmt.Fprintf(w, `<!doctype html>
<html>
<head><title>Received Cookies</title></head>
<body>
<h1>Received Cookies</h1>
<ul>`)
	for _, cookie := range r.Cookies() {
		fmt.Fprintf(w, "<li>Name=%q Value=%q</li>\n", cookie.Name, cookie.Value)
	}
	fmt.Fprintf(w, `</ul>
</body>
</html>
`)
}

// expectCheckFailures checks that each check failed.
func expectCheckFailures(t *testing.T, descr string, result TestResult, test Test) {
	if result.Status != Fail {
		t.Fatalf("%s: Expected Fail, got %s", descr, result.Status)
	}
	if len(result.CheckResults) != len(test.Checks) {
		t.Fatalf("%s: Expected %d entries, got %d: %#v",
			descr, len(test.Checks), len(result.CheckResults), result)
	}

	for i, r := range result.CheckResults {
		if r.Status != Fail {
			t.Errorf("%s check %d: Expect Fail, got %s", descr, i, r.Status)
		}
	}
}

func TestClientTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(echoHandler))
	defer ts.Close()

	test := Test{
		Name: "Client Timeout",
		Request: Request{
			Method: "GET",
			URL:    ts.URL + "/",
			Params: url.Values{
				"smin": {"100"}, "smax": {"110"},
			},
			FollowRedirects: false,
		},
		Checks: []check.Check{
			check.StatusCode{200},
		},
		Timeout: 40 * time.Millisecond,
	}
	err := test.prepare(nil)
	if err != nil {
		t.Fatalf("Unecpected error: %v", err)
	}

	start := time.Now()
	_, err = test.executeRequest()
	if err == nil {
		t.Fatalf("No error reported.")
	}
	if d := time.Since(start); d > 99*time.Millisecond {
		t.Errorf("Took too long: %s", d)
	}
}

func TestMarshalTest(t *testing.T) {
	test := &Test{
		Name:        "Unic: Search",
		Description: "Some searches",
		Request: Request{
			URL: "https://{{HOST}}/de/tools/suche.html",
			Params: url.Values{
				"q": []string{"{{QUERY}}"},
				"w": []string{"AB", "XZ"},
			},
			FollowRedirects: true,
			ParamsAs:        "URL",
			Body:            "Some data to send.",
			Header: http.Header{
				"User-Agent": {"Our-Test-Agent"},
			},
			Cookies: []Cookie{
				Cookie{Name: "first", Value: "false"},
				Cookie{Name: "trusted", Value: "true"},
			},
		},
		Checks: []check.Check{
			check.StatusCode{200},
			check.BodyContains{Text: "© 2014 Unic", Count: 1},
		},
	}

	data, err := json.MarshalIndent(test, "", "    ")
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	fmt.Println(string(data))

	recreated := Test{}
	err = json.Unmarshal(data, &recreated)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	data2, err := json.MarshalIndent(recreated, "", "    ")
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	if string(data) != string(data2) {
		t.Fatalf("Missmatch. Got\n%s\nWant\n%s\n", data2, data)

	}
}
