// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/kr/pretty"
	"github.com/vdobler/ht/internal/json5"
)

func TestStatusCode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(echoHandler))
	defer ts.Close()

	for _, code := range []int{200, 201, 204, 300, 400, 404, 500} {
		s := fmt.Sprintf("%d", code)
		test := Test{
			Name: "A very basic test.",
			Request: Request{
				Method:          "GET",
				URL:             ts.URL + "/",
				Params:          URLValues{"status": []string{s}},
				FollowRedirects: false,
			},
			Checks: []Check{
				StatusCode{Expect: code},
			},
		}

		test.Run(nil)
		if test.Status != Pass {
			t.Errorf("Unexpected error for %d: %s", code, t.Error)
		}
		if testing.Verbose() {
			test.PrintReport(os.Stdout)
		}
	}
}

func TestParameterHandling(t *testing.T) {
	test := Test{Request: Request{
		Method: "POST",
		URL:    "http://www.test.org",
		Params: URLValues{
			"single":  []string{"abc"},
			"multi":   []string{"1", "2"},
			"special": []string{"A%+& &?/Z"},
			"file":    []string{"@file:../testdata/upload.pdf"},
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
	if got := test.Request.Request.URL.String(); got != "http://www.test.org?file=%40file%3A..%2Ftestdata%2Fupload.pdf&multi=1&multi=2&single=abc&special=A%25%2B%26+%26%3F%2FZ" {
		t.Errorf("Bad URL, got %s", got)
	}
	test.Request.Body = ""

	// URLencoded in the body.
	test.Request.ParamsAs = "body"
	err = test.prepare(nil)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
	full, err := ioutil.ReadAll(test.Request.Request.Body)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
	if string(full) != "file=%40file%3A..%2Ftestdata%2Fupload.pdf&multi=1&multi=2&single=abc&special=A%25%2B%26+%26%3F%2FZ" {
		t.Errorf("Bad body, got %s", full)
	}
	test.Request.Body = ""

	test.Request.ParamsAs = "multipart"
	err = test.prepare(nil)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
	ct := test.Request.Request.Header.Get("Content-Type")
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
	r := multipart.NewReader(test.Request.Request.Body, boundary)
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
			Params: URLValues{
				"smin": []string{"{{SMIN}}"},
				"smax": []string{"{{SMAX}}"},
				"fail": {"5"},
			},
			FollowRedirects: false,
		},
		Checks: []Check{
			StatusCode{200},
		},
		Timeout: Duration(150 * time.Millisecond),
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

	err := suite.Prepare()
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

var (
	pollingHandlerFailCnt  = 0
	pollingHandlerErrorCnt = 0
)

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
func expectCheckFailures(t *testing.T, descr string, test Test) {
	if test.Status != Fail {
		t.Fatalf("%s: Expected Fail, got %s", descr, test.Status)
	}
	if len(test.CheckResults) != len(test.Checks) {
		t.Fatalf("%s: Expected %d entries, got %d: %#v",
			descr, len(test.Checks), len(test.CheckResults), test)
	}

	for i, r := range test.CheckResults {
		if r.Status != Fail {
			t.Errorf("%s check %d: Expect Fail, got %s", descr, i, r.Status)
		}
	}
}

// pollingHandler
//     /?t=faile&n=7   returns a 500 for 6 times and a 200 on the 7th request
//     /?t=error&n=4   waits for 100ms for 4 times and responds with 200 on the 5th request
func pollingHandler(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.FormValue("n"))
	if err != nil {
		panic(err.Error())
	}

	switch what := r.FormValue("t"); what {
	case "fail":
		pollingHandlerFailCnt++
		if n < pollingHandlerFailCnt {
			http.Error(w, "All good", http.StatusOK)
			return
		}
		http.Error(w, "ooops", http.StatusInternalServerError)
	case "error":
		pollingHandlerErrorCnt++
		if n < pollingHandlerErrorCnt {
			http.Error(w, "All good", http.StatusOK)
			return
		}
		time.Sleep(100 * time.Millisecond)
		http.Error(w, "sorry, busy", http.StatusInternalServerError)
	default:
		panic("Unknown type " + what)
	}
}

func TestPolling(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(pollingHandler))
	defer ts.Close()

	for i, tc := range []struct {
		max  int
		typ  string
		want Status
	}{
		{max: 2, typ: "fail", want: Fail},
		{max: 4, typ: "fail", want: Pass},
		{max: 1, typ: "fail", want: Fail},
		{max: 5, typ: "fail", want: Pass},
		{max: 2, typ: "error", want: Error},
		{max: 4, typ: "error", want: Pass},
		{max: 1, typ: "error", want: Error},
		{max: 5, typ: "error", want: Pass},
	} {
		pollingHandlerFailCnt, pollingHandlerErrorCnt = 0, 0
		test := Test{
			Name: "Polling",
			Request: Request{
				Method: "GET",
				URL:    ts.URL + "/",
				Params: URLValues{
					"n": {"3"},
					"t": {tc.typ},
				},
			},
			Checks: []Check{
				StatusCode{200},
			},
			Poll: Poll{
				Max:   tc.max,
				Sleep: Duration(200),
			},
			Timeout: Duration(50 * time.Millisecond),
		}
		test.Run(nil)
		if got := test.Status; got != tc.want {
			t.Errorf("%d: got %s, want %s", i, got, tc.want)
		}
		if tc.want == Pass && test.Error != nil {
			t.Errorf("%d: got non-nil eror: %+v", i, test.Error)
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
			Params: URLValues{
				"smin": {"100"}, "smax": {"110"},
			},
			FollowRedirects: false,
		},
		Checks: []Check{
			StatusCode{200},
		},
		Timeout: Duration(40 * time.Millisecond),
	}
	start := time.Now()
	test.Run(nil)
	if d := time.Since(start); d > 99*time.Millisecond {
		t.Errorf("Took too long: %s", d)
	}

	if test.Status != Error {
		t.Errorf("Got status %s, want Error", test.Status)
	}
}

func TestMarshalTest(t *testing.T) {
	test := &Test{
		Name:        "Unic: Search",
		Description: "Some searches",
		Request: Request{
			URL: "https://{{HOST}}/de/tools/suche.html",
			Params: URLValues{
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
		Checks: []Check{
			StatusCode{200},
			&Body{Contains: "© 2014 Unic", Count: 1},
		},
	}

	data, err := json5.MarshalIndent(test, "", "    ")
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	if testing.Verbose() {
		fmt.Println(string(data))
	}
	recreated := Test{}
	err = json5.Unmarshal(data, &recreated)
	if err != nil {
		w := err.(*json5.UnmarshalTypeError)
		t.Fatalf("Unexpected error: %#v\n%s\n%#v", err, w, recreated)
	}

	data2, err := json5.MarshalIndent(recreated, "", "    ")
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	if string(data) != string(data2) {
		t.Fatalf("Missmatch. Got\n%s\nWant\n%s\n", data2, data)

	}
}

func TestMerge(t *testing.T) {
	a := &Test{}
	b := &Test{}
	_, err := Merge(a, b)
	if err != nil {
		t.Fatalf("Unexpected error %#v", err)
	}

	a = &Test{
		Name:        "A",
		Description: "A does a in a very a-ish way.",
		Request: Request{
			Method: "POST",
			URL:    "http://demo.test",
			Header: http.Header{
				"User-Agent": []string{"A User Agent"},
				"Special-A":  []string{"Special A Value"},
			},
			Params: URLValues{
				"q": []string{"foo-A"},
				"a": []string{"aa", "AA"},
			},
			Cookies: []Cookie{
				{Name: "a", Value: "vaaaaalue"},
				{Name: "session", Value: "deadbeef"},
			},
			FollowRedirects: true,
		},
		PreSleep:   Duration(100),
		InterSleep: Duration(120),
		PostSleep:  Duration(140),
	}

	b = &Test{
		Name:        "B",
		Description: "B does b in a very b-ish way.",
		Request: Request{
			Method: "POST",
			Header: http.Header{
				"User-Agent": []string{"B User Agent"},
				"Special-B":  []string{"Special B Value"},
			},
			Params: URLValues{
				"q": []string{"foo-B"},
				"b": []string{"bb", "BB"},
			},
			Cookies: []Cookie{
				{Name: "b", Value: "vbbbbblue"},
				{Name: "session", Value: "othersession"},
			},
			FollowRedirects: false,
		},
		InterSleep: Duration(300),
	}

	c, err := Merge(a, b)
	if err != nil {
		t.Fatalf("Unexpected error %#v", err)
	}
	if testing.Verbose() {
		pretty.Printf("% #v\n", c)
	}
	if len(c.Request.Params) != 3 ||
		c.Request.Params["a"][0] != "aa" ||
		c.Request.Params["b"][0] != "bb" ||
		c.Request.Params["q"][0] != "foo-B" {
		t.Errorf("Bad Params. Got %#v", c.Request.Params)
	}
	if len(c.Request.Header) != 3 ||
		c.Request.Header["Special-A"][0] != "Special A Value" ||
		c.Request.Header["Special-B"][0] != "Special B Value" ||
		c.Request.Header["User-Agent"][0] != "B User Agent" {
		t.Errorf("Bad Header. Got %#v", c.Request.Header)
	}
	if len(c.Request.Cookies) != 3 ||
		c.Request.Cookies[0].Value != "vaaaaalue" ||
		c.Request.Cookies[1].Value != "othersession" ||
		c.Request.Cookies[2].Value != "vbbbbblue" {
		t.Errorf("Bad cookies. Got %#v", c.Request.Cookies)
	}

	if c.PreSleep != 100 || c.InterSleep != 420 || c.PostSleep != 140 {
		t.Errorf("Bad sleep times. Got pre=%s, inter=%s, post=%s",
			c.PreSleep, c.InterSleep, c.PostSleep)
	}

}

func TestUnmarshalURLValues(t *testing.T) {
	j := []byte(`{
    q: 7,
    w: "foo",
    z: [ 3, 1, 4, 1 ],
    x: [ "bar", "quz" ],
    y: [ 2, "waz", 9 ],
    v: [ 1.2, -1.2, 4.00001, -4.00001, 3.999999, -3.99999 ]
}`)

	var uv URLValues
	err := json5.Unmarshal(j, &uv)
	if err != nil {
		t.Fatalf("Unexpected error %#v", err)
	}

	s, err := json5.MarshalIndent(uv, "", "    ")
	if err != nil {
		t.Fatalf("Unexpected error %#v", err)
	}

	if string(s) != `{
    q: [
        "7"
    ],
    v: [
        "1.2",
        "-1.2",
        "4.00001",
        "-4.00001",
        "3.999999",
        "-3.99999"
    ],
    w: [
        "foo"
    ],
    x: [
        "bar",
        "quz"
    ],
    y: [
        "2",
        "waz",
        "9"
    ],
    z: [
        "3",
        "1",
        "4",
        "1"
    ]
}` {
		t.Errorf("Got unexpected value:\n%s", s)
	}

}

func TestUnmarshalURLValuesError(t *testing.T) {
	for i, tc := range []string{
		`{q: {a:1, b:2}}`,
		`{q: [ [1,2], [3,4] ] }`,
		`{q: [ 1, 2, {a:7, b:8}, 3 ] }`,
	} {

		var uv URLValues
		err := json5.Unmarshal([]byte(tc), &uv)
		if err == nil {
			t.Errorf("%d. missing error on %s", i, tc)
		}
	}
}
