// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var verboseTest = flag.Bool("ht.verbose", false, "be verbose during testing")

func TestSkippingChecks(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(echoHandler))
	defer ts.Close()

	for i, tc := range []struct {
		code, first, second int
		fstatus, sstatus    Status
		pointer             bool
	}{
		{code: 200, first: 200, second: 200, fstatus: Pass, sstatus: Pass, pointer: false},
		{code: 200, first: 200, second: 400, fstatus: Pass, sstatus: Fail, pointer: false},
		{code: 500, first: 200, second: 400, fstatus: Fail, sstatus: Skipped, pointer: false},
		{code: 400, first: 400, second: 400, fstatus: Pass, sstatus: Pass, pointer: false},
		{code: 400, first: 400, second: 200, fstatus: Pass, sstatus: Fail, pointer: false},
		{code: 400, first: 300, second: 200, fstatus: Fail, sstatus: Fail, pointer: false},

		{code: 200, first: 200, second: 200, fstatus: Pass, sstatus: Pass, pointer: true},
		{code: 200, first: 200, second: 400, fstatus: Pass, sstatus: Fail, pointer: true},
		{code: 500, first: 200, second: 400, fstatus: Fail, sstatus: Skipped, pointer: true},
		{code: 400, first: 400, second: 400, fstatus: Pass, sstatus: Pass, pointer: true},
		{code: 400, first: 400, second: 200, fstatus: Pass, sstatus: Fail, pointer: true},
		{code: 400, first: 300, second: 200, fstatus: Fail, sstatus: Fail, pointer: true},
	} {
		test := Test{
			Request: Request{
				Method: "GET",
				URL:    ts.URL + "/",
				Params: url.Values{
					"status": []string{fmt.Sprintf("%d", tc.code)}},
			},
			Checks: []Check{
				StatusCode{Expect: tc.first},
				StatusCode{Expect: tc.second},
			},
		}
		if tc.pointer {
			// StatusCode as well as *StatusCode satisfy the Check interface.
			sc0 := test.Checks[0].(StatusCode)
			sc1 := test.Checks[1].(StatusCode)
			test.Checks[0] = &sc0
			test.Checks[1] = &sc1
		}
		test.Run()
		if test.CheckResults[0].Status != tc.fstatus ||
			test.CheckResults[1].Status != tc.sstatus {
			t.Errorf("%d,%t: %d against %d/%d, got %s/%s want %s/%s", i, tc.pointer, tc.code,
				tc.first, tc.second,
				test.CheckResults[0].Status, test.CheckResults[1].Status,
				tc.fstatus, tc.sstatus)
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
			"file":    []string{"@file:testdata/somefile.txt"},
		},
		ParamsAs: "URL",
	}}

	// As part of the URL.
	err := test.prepareRequest()
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
	if test.Request.Body != "" {
		t.Errorf("Expected empty body, got %q", test.Request.Body)
	}
	if got := test.Request.Request.URL.String(); got != "http://www.test.org?file=%40file%3Atestdata%2Fsomefile.txt&multi=1&multi=2&single=abc&special=A%25%2B%26+%26%3F%2FZ" {
		t.Errorf("Bad URL, got %s", got)
	}
	test.Request.Body = ""

	// URLencoded in the body.
	test.Request.ParamsAs = "body"
	err = test.prepareRequest()
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
	test.resetRequest()
	full, err := ioutil.ReadAll(test.Request.Request.Body)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
	if string(full) != "file=%40file%3Atestdata%2Fsomefile.txt&multi=1&multi=2&single=abc&special=A%25%2B%26+%26%3F%2FZ" {
		t.Errorf("Bad body, got %s", full)
	}
}

func TestMultipartParameterHandling(t *testing.T) {
	test := Test{Request: Request{
		Method: "POST",
		URL:    "http://www.test.org",
		Params: url.Values{
			"single":  []string{"abc"},
			"multi":   []string{"1", "2"},
			"special": []string{"A%+& &?/Z"},
			"file":    []string{"@file:testdata/somefile.txt", "@file:testdata/home.png"},
			"vfile":   []string{"@vfile:testdata/somefile.txt"},
			"dfile":   []string{"@file:@name:the-data"},
		},
		ParamsAs: "multipart",
	},
		Variables: map[string]string{"XYZ": "+++"},
	}

	err := test.prepareRequest()
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
	test.resetRequest()
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

	value := f.Value
	if len(value) != 3 || len(value["single"]) != 1 || len(value["multi"]) != 2 ||
		len(value["special"]) != 1 {
		t.Errorf("Wrong size, got \n%#v\n", value)
	} else if value["single"][0] != "abc" ||
		value["multi"][0] != "1" || value["multi"][1] != "2" ||
		value["special"][0] != "A%+& &?/Z" {
		t.Errorf("Wrong content, got \n%#v\n", value)
	}

	files := f.File
	if len(files) != 3 || len(files["file"]) != 2 ||
		len(files["dfile"]) != 1 || len(files["vfile"]) != 1 {
		t.Errorf("Wrong size, got \n%#v\n", files)
	} else {
		file0 := files["file"][0]
		if file0.Filename != "somefile.txt" ||
			!strings.Contains(file0.Header["Content-Type"][0], "text/plain") ||
			!strings.Contains(file0.Header["Content-Disposition"][0], `filename="somefile.txt"`) {
			t.Errorf("Wrong file[0], got %+v", file0)
		}
		compareMPFileContent(t, "Hello {{XYZ}} World.\n", file0)

		file1 := files["file"][1]
		if file1.Filename != "home.png" ||
			!strings.Contains(file1.Header["Content-Type"][0], "image/png") ||
			!strings.Contains(file1.Header["Content-Disposition"][0], `filename="home.png"`) {
			t.Errorf("Wrong file[1], got %+v", file1)
		}

		vfile := files["vfile"][0]
		if vfile.Filename != "somefile.txt" ||
			!strings.Contains(vfile.Header["Content-Type"][0], "text/plain") ||
			!strings.Contains(vfile.Header["Content-Disposition"][0], `filename="somefile.txt"`) {
			t.Errorf("Wrong file[0], got %+v", vfile)
		}
		compareMPFileContent(t, "Hello +++ World.\n", vfile)

		dfile := files["dfile"][0]
		if dfile.Filename != "name" ||
			!strings.Contains(dfile.Header["Content-Type"][0], "application/octet-stream") ||
			!strings.Contains(dfile.Header["Content-Disposition"][0], `filename="name"`) {
			t.Errorf("Wrong dfile, got %+v", dfile)
		}
		compareMPFileContent(t, "the-data", dfile)
	}
}

func compareMPFileContent(t *testing.T, want string, fh *multipart.FileHeader) {
	file, err := fh.Open()
	if err != nil {
		t.Error(err)
		return
	}
	got, err := ioutil.ReadAll(file)
	if err != nil {
		t.Error(err)
		return
	}
	if g := string(got); g != want {
		t.Errorf("Got %q want %q", g, want)
	}
}

func TestSendBody(t *testing.T) {
	test := Test{Request: Request{
		Method: "POST",
		URL:    "http://www.test.org",
		Body:   "@vfile:testdata/somefile.txt",
	},
		Variables: map[string]string{"XYZ": "+++"},
	}
	err := test.prepareRequest()
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
	if test.Request.SentBody != "Hello +++ World.\n" {
		t.Errorf("got %q", test.Request.SentBody)
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
// The handler sleeps for a random duration between smin and smax milliseconds.
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
	pollingHandlerMu       = &sync.Mutex{}
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
//     /?t=fail&n=7   returns a 500 for 6 times and a 200 on the 7th request
//     /?t=error&n=4   waits for 70ms for 4 times and responds with 200 on the 5th request
func pollingHandler(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.FormValue("n"))
	if err != nil {
		panic(err.Error())
	}

	pollingHandlerMu.Lock()
	defer pollingHandlerMu.Unlock()
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
		time.Sleep(70 * time.Millisecond)
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
		pollingHandlerMu.Lock()
		pollingHandlerFailCnt, pollingHandlerErrorCnt = 0, 0
		pollingHandlerMu.Unlock()
		test := Test{
			Name: "Polling",
			Request: Request{
				Method: "GET",
				URL:    ts.URL + "/",
				Params: url.Values{
					"n": {"3"},
					"t": {tc.typ},
				},
				Timeout: 60 * time.Millisecond,
			},
			Checks: []Check{
				StatusCode{200},
			},
			Execution: Execution{
				Tries: tc.max,
				Wait:  5 * time.Millisecond,
			},
		}
		test.Run()
		if got := test.Status; got != tc.want {
			t.Errorf("%d: got %s, want %s (error=%s)", i, got, tc.want, test.Error)
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
			Params: url.Values{
				"smin": {"100"}, "smax": {"110"},
			},
			FollowRedirects: false,
			Timeout:         40 * time.Millisecond,
		},
		Checks: []Check{
			StatusCode{200},
		},
	}
	start := time.Now()
	test.Run()
	if d := time.Since(start); d > 99*time.Millisecond {
		t.Errorf("Took too long: %s", d)
	}

	if test.Status != Error {
		t.Errorf("Got status %s, want Error", test.Status)
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
			Params: url.Values{
				"q": []string{"foo-A"},
				"a": []string{"aa", "AA"},
			},
			Cookies: []Cookie{
				{Name: "a", Value: "vaaaaalue"},
				{Name: "session", Value: "deadbeef"},
			},
			FollowRedirects: true,
			Chunked:         false,
		},
		Execution: Execution{
			PreSleep:   100,
			InterSleep: 120,
			PostSleep:  140,
		},
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
			Params: url.Values{
				"q": []string{"foo-B"},
				"b": []string{"bb", "BB"},
			},
			Cookies: []Cookie{
				{Name: "b", Value: "vbbbbblue"},
				{Name: "session", Value: "othersession"},
			},
			FollowRedirects: false,
			Chunked:         true,
			BasicAuthUser:   "foo.bar",
			BasicAuthPass:   "secret",
		},
		Execution: Execution{
			InterSleep: 300,
		},
	}

	c, err := Merge(a, b)
	if err != nil {
		t.Fatalf("Unexpected error %#v", err)
	}
	if *verboseTest {
		jr, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err.Error())
		}
		fmt.Println(string(jr))
	}
	if len(c.Request.Params) != 3 ||
		c.Request.Params["a"][0] != "aa" ||
		c.Request.Params["b"][0] != "bb" ||
		c.Request.Params["q"][0] != "foo-A" ||
		c.Request.Params["q"][1] != "foo-B" {
		t.Errorf("Bad Params. Got %#v", c.Request.Params)
	}
	if len(c.Request.Header) != 3 ||
		c.Request.Header["Special-A"][0] != "Special A Value" ||
		c.Request.Header["Special-B"][0] != "Special B Value" ||
		c.Request.Header["User-Agent"][0] != "A User Agent" ||
		c.Request.Header["User-Agent"][1] != "B User Agent" {
		t.Errorf("Bad Header. Got %#v", c.Request.Header)
	}
	if len(c.Request.Cookies) != 3 ||
		c.Request.Cookies[0].Value != "vaaaaalue" ||
		c.Request.Cookies[1].Value != "othersession" ||
		c.Request.Cookies[2].Value != "vbbbbblue" {
		t.Errorf("Bad cookies. Got %#v", c.Request.Cookies)
	}

	if c.Request.BasicAuthUser != "foo.bar" || c.Request.BasicAuthPass != "secret" {
		t.Errorf("Bad BasicAuth. Got %q : %q", c.Request.BasicAuthUser,
			c.Request.BasicAuthPass)
	}

	if c.Request.FollowRedirects || !c.Request.Chunked {
		t.Errorf("FollowRedirect=%t Chunked=%t",
			c.Request.FollowRedirects, c.Request.Chunked)
	}

	if c.Execution.PreSleep != 100 || c.Execution.InterSleep != 420 || c.Execution.PostSleep != 140 {
		t.Errorf("Bad sleep times. Got pre=%s, inter=%s, post=%s",
			c.Execution.PreSleep, c.Execution.InterSleep, c.Execution.PostSleep)
	}

}

func bodyReadTestHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/hello":
		http.Error(w, "Hello World", http.StatusOK)
	case "/redirect-plain":
		r.URL.Path = "/hello"
		w.Header().Set("Location", r.URL.String())
		w.WriteHeader(302)
	case "/redirect-content":
		r.URL.Path = "/hello"
		w.Header().Set("Location", r.URL.String())
		w.WriteHeader(302)
		fmt.Fprintln(w, "Please go to /hello")
	default:
		http.Error(w, "Ooops", http.StatusInternalServerError)
	}
}

func TestReadBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(bodyReadTestHandler))
	defer ts.Close()

	for _, path := range []string{"/redirect-plain", "/redirect-content"} {
		test := Test{
			Request: Request{
				Method:          "GET",
				URL:             ts.URL + path,
				FollowRedirects: false,
			},
			Checks: []Check{NoServerError{}},
		}
		test.Run()

		if test.Response.BodyErr != nil {
			t.Errorf("Path %q: Unexpected problem reading body: %#v",
				path, test.Response.BodyErr)
		}
	}
}

// ----------------------------------------------------------------------------
// file://

func TestFileSchema(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	p := wd + "/testdata/fileprotocol"
	u := "file://" + p

	tests := []*Test{
		{
			Name: "PUT",
			Request: Request{
				URL:  u,
				Body: "Tadadadaaa!",
			},
			Checks: []Check{
				StatusCode{Expect: 200},
				&Body{Prefix: "Successfully wrote " + u},
			},
		},
		{
			Name: "PUT",
			Request: Request{
				URL:  u + "/iouer/cxxs/dlkfj",
				Body: "Tadadadaaa!",
			},
			Checks: []Check{
				StatusCode{Expect: 403},
				&Body{Contains: p},
				&Body{Contains: "not a directory"},
			},
		},
		{
			Name: "GET",
			Request: Request{
				URL: u,
			},
			Checks: []Check{
				StatusCode{Expect: 200},
				&Body{Equals: "Tadadadaaa!"},
			},
		},
		{
			Name: "GET Fail",
			Request: Request{
				URL: u,
			},
			Checks: []Check{
				StatusCode{Expect: 200},
				&Body{Equals: "something else"},
				&Header{Header: "Foo", Absent: true},
			},
		},
		{
			Name: "GET",
			Request: Request{
				URL: u + "/slkdj/cxmvn",
			},
			Checks: []Check{
				StatusCode{Expect: 404},
				&Body{Contains: "not a directory"},
				&Body{Contains: p},
			},
		},
		{
			Name: "GET Error",
			Request: Request{
				URL: "file://remote.host/some/path",
			},
			Checks: []Check{
				StatusCode{Expect: 200},
			},
		},
		{
			Name: "DELETE",
			Request: Request{
				URL: u,
			},
			Checks: []Check{
				StatusCode{Expect: 200},
				&Body{Prefix: "Successfully deleted " + u},
			},
		},
		{
			Name: "DELETE",
			Request: Request{
				URL: u + "/sdjdfh/oieru",
			},
			Checks: []Check{
				StatusCode{Expect: 404},
				&Body{Contains: "no such file or directory"},
				&Body{Contains: p},
			},
		},
	}

	for i, test := range tests {
		method, want := test.Name, "Pass"
		p := strings.Index(test.Name, " ")
		if p != -1 {
			method, want = test.Name[:p], test.Name[p+1:]
		}
		test.Request.Method = method
		err = test.Run()
		if err != nil {
			t.Fatalf("%d. %s: Unexpected error: ", i, err)
		}

		got := test.Status.String()
		if got != want {
			t.Errorf("%d. %s: got %s, want %s.\nError=%v\nBody=%q",
				i, test.Name, got, want, test.Error, test.Response.BodyStr)

		}
	}
}

var testCurlCalls = []struct {
	paramsAs string
	want     string
}{
	{"URL", `curl -X POST -u 'root:secret' -H 'X-Custom-A: go' -H 'X-Custom-A: fast' -H 'Accept: */*' -H 'User-Agent: unknown' -H 'Cookie: session=deadbeef' 'http://localhost:808/foo?bar=1&abc=12&abc=34'`},
	{"body", `curl -X POST -u 'root:secret' -H 'X-Custom-A: go' -H 'X-Custom-A: fast' -H 'Accept: */*' -H 'User-Agent: unknown' -H 'Cookie: session=deadbeef' -d 'abc=12' -d 'abc=34' 'http://localhost:808/foo?bar=1'`},
	{"multipart", `curl -X POST -u 'root:secret' -H 'X-Custom-A: go' -H 'X-Custom-A: fast' -H 'Accept: */*' -H 'User-Agent: unknown' -H 'Cookie: session=deadbeef' -F 'abc=12' -F 'abc=34' 'http://localhost:808/foo?bar=1'`},
}

func TestCurlCall(t *testing.T) {
	theSame := func(a, b string) bool {
		// Forgive me, it has been a loooong day...
		as, bs := strings.Split(a, " "), strings.Split(b, " ")
		sort.Strings(as)
		sort.Strings(bs)
		a, b = strings.Join(as, " "), strings.Join(bs, " ")
		same := a == b
		if !same {
			am, bm := make(map[string]bool, len(as)), make(map[string]bool, len(bs))
			for _, p := range as {
				am[p] = true
			}
			for _, p := range bs {
				if !am[p] {
					t.Logf("Missing in a: %s", p)
				}
				bm[p] = true
			}
			for _, p := range as {
				if !bm[p] {
					t.Logf("Missing in b: %s", p)
				}
			}
		}
		return same
	}

	test := &Test{
		Name: "Simple",
		Request: Request{
			Method: "POST",
			URL:    "http://localhost:808/foo?bar=1",
			Params: url.Values{
				"abc": []string{"12", "34"},
				// "file": []string{"@file:testdata/somefile.txt"},
			},
			Cookies: []Cookie{
				{Name: "session", Value: "deadbeef"},
			},
			Header: http.Header{
				"Accept":     []string{"*/*"},
				"User-Agent": []string{"unknown"},
				"X-Custom-A": []string{"go", "fast"},
			},
			BasicAuthUser: "root",
			BasicAuthPass: "secret",
		},
	}

	for i, tc := range testCurlCalls {
		test.Request.ParamsAs = tc.paramsAs
		test.Run()
		got := test.CurlCall()
		if !theSame(got, tc.want) {
			t.Errorf("%d. %s\nGot : %s\nWant: %s",
				i, tc.paramsAs, got, tc.want)
		}
	}
}

var testCurlCallsBody = []struct {
	body string
	want string
}{
	{"simple text", "curl -X POST --data-binary 'simple text' 'http://localhost'"},
	{"\x12\x17ABC\"XYZ'abc", `tmp=$(mktemp)
printf "\x12\x17ABC\x22XYZ\x27abc" > $tmp
curl -X POST --data-binary "@$tmp" 'http://localhost'`},
	{"@file:testdata/somefile.txt", "curl -X POST --data-binary '@testdata/somefile.txt' 'http://localhost'"},
}

func TestCurlCallBody(t *testing.T) {
	test := &Test{
		Name: "Simple",
		Request: Request{
			Method: "POST",
			URL:    "http://localhost",
			Body:   "",
		},
	}

	for i, tc := range testCurlCallsBody {
		test.Request.Body = tc.body
		test.Run()
		got := test.CurlCall()
		if got != tc.want {
			t.Errorf("%d. %q\nGot : %s\nWant: %s",
				i, tc.body, got, tc.want)
		}
	}
}

func TestStatusFromString(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Status
	}{
		{"NotRun", NotRun},
		{"skipped", Skipped},
		{"PASS", Pass},
		{" fail ", Fail},
		{"Error", Error},
		{"bogus", Bogus},
		{"foobar", Status(-1)},
	} {
		if got := StatusFromString(tc.in); got != tc.want {
			t.Errorf("StatusFromString(%q)=%d, want %d",
				tc.in, got, tc.want)
		}
	}

}
