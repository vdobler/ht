// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/vdobler/ht/check"
	"github.com/vdobler/ht/response"
)

var (
	// DefaultUserAgent is the user agent string to send in http requests
	// if no user agent header is specified explicitely.
	DefaultUserAgent = "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/36.0.1985.143 Safari/537.36"

	// DefaultAccept is the accept header to be sent if no accept header
	// is set explicitely in the test.
	DefaultAccept = "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8"

	// DefaultClientTimeout is the timeout used by the http clients.
	DefaultClientTimeout = 2 * time.Second
)

// Request is a HTTP request.
type Request struct {
	// Method is the HTTP method to use.
	// A empty method is equivalent to "GET"
	Method string `json:",omitempty"`

	// URL ist the URL of the request.
	URL string

	// Params contains the parameters and their values to send in
	// the request.
	//
	// If the parameters are sent as multipart it is possible to include
	// files: If a parameter values starts with "@file:" the rest of
	// the value is interpreted as as filename and this file is sent.
	Params url.Values

	// ParamsAs determines how the parameters in the Param field are sent:
	//   "URL" or "": append properly encoded to URL
	//   "body"     : send as application/x-www-form-urlencoded in body.
	//   "multipart": send as multipart in body.
	// The two values "body" and "multipart" must not be used
	// on a GET or HEAD request.
	ParamsAs string

	// Header contains the specific http headers to be sent in this request.
	Header http.Header

	// Cookies contains the cookies to send in the request.
	Cookies []Cookie

	// Body is the full body to send in the request. Body must be
	// empty if Params are sent as multipart or form-urlencoded.
	Body string `json:",omitempty"`

	// FollowRedirects determines if automatic following of
	// redirects should be done.
	FollowRedirects bool `json:",omitempty"`
}

// Cookie.
type Cookie struct {
	Name  string
	Value string `json:",omitempty"`
}

// ----------------------------------------------------------------------------
// Test

// Test is a single logical test which does one HTTP request and checks
// a number of Checks on the recieved Result.
type Test struct {
	Name        string
	Description string

	// Request is the HTTP request.
	Request Request

	// Checks contains all checks to perform on the response to the HTTP request.
	Checks check.CheckList

	// UnrollWith contains values to be used during unrolling the test
	// to several instances.
	// TODO: doesn't belong here: move to deserialization code.
	UnrollWith map[string][]string

	Jar http.CookieJar `json:"-"` // The possible prepopulated cookie jar to use.
	Log *log.Logger    `json:"-"` // The logger to use by the test and the checks.

	Poll    Poll          `json:",omitempty"`
	Timeout time.Duration // If zero use DefaultClientTimeout

	client   *http.Client
	request  *http.Request
	response *response.Response
	checks   []check.Check // compiled checks.

	verbose int
}

// Poll determines if and how to redo a test after a failure or if the
// test should be skipped alltogether. The zero value of Poll means "Just do
// the test once."
type Poll struct {
	// Maximum number of redos. Both 0 and 1 mean: "Just one try. No redo."
	// Negative values indicate that the test should be skipped.
	Max int `json:",omitempty"`

	// Duration to sleep between redos.
	Sleep time.Duration `json:",omitempty"`
}

// Skip return whether the test should be skipped.
func (p Poll) Skip() bool {
	return p.Max < 0
}

func (t *Test) Infof(format string, v ...interface{}) {
	if t.Log != nil && t.verbose > 1 {
		format = "INFO " + format
		t.Log.Printf(format, v...)
	}
}

func (t *Test) Warnf(format string, v ...interface{}) {
	if t.Log != nil && t.verbose > 0 {
		format = "WARN " + format
		t.Log.Printf(format, v...)
	}
}

func (t *Test) Debugf(format string, v ...interface{}) {
	if t.Log != nil && t.verbose > 2 {
		format = "DEBG " + format
		t.Log.Printf(format, v...)
	}
}

// Run runs the test t. The actual HTTP request is crafted and executed and
// the checks are performed on the received response. This whole process
// is repeated on failure or skipped entirely according to t.Poll.
//
// Normally all checks in t.Checks are executed. If the first check in
// t.Checks is a StatusCode check against 200 and it fails, then the rest of
// the tests are skipped.
func (t *Test) Run(variables map[string]string) Result {
	var result Result

	maxTries := t.Poll.Max
	if maxTries == 0 {
		maxTries = 1
	}
	if maxTries < 0 {
		// This test is deliberately skipped.
		result.Status = Skipped
		return result
	}

	// Try until first success.
	try := 0
	start := time.Now()
	for try = 0; try < maxTries; try++ {
		if try > 0 {
			time.Sleep(t.Poll.Sleep)
		}
		err := t.prepare(variables)
		if err != nil {
			result.Status, result.Error = Bogus, err
			return result
		}
		result = t.execute()
		if result.Status == Pass {
			break
		}
	}
	result.FullDuration = time.Since(start)
	if t.Poll.Max > 1 {
		if result.Status == Pass {
			t.Debugf("Polling Test=%q succeded after %d tries.",
				t.Name, try+1)
		} else {
			t.Debugf("Polling Test=%q failed all %d tries.",
				t.Name, maxTries)
		}
	}
	return result
}

// execute does a single request and check the response.
func (t *Test) execute() Result {
	var result Result
	response, err := t.executeRequest()
	if err == nil {
		result.Elements = t.executeChecks(response)
		result.Status = CombinedStatus(result.Elements)
	} else {
		result.Status = Error
		result.Error = err
		result.Elements = make([]Result, len(t.Checks))
		for i := range result.Elements {
			result.Elements[i].Status = Skipped
		}
	}
	result.Duration = response.Duration
	// TODO: stuff respons into Result and return it
	return result
}

// prepare the test for execution by substituting the given variables and
// crafting the underlying http request the checks.
func (t *Test) prepare(variables map[string]string) error {
	// Create appropriate replace.
	nowVars := t.nowVariables(time.Now())
	allVars := mergeVariables(variables, nowVars)
	repl, err := newReplacer(allVars)
	if err != nil {
		return err
	}

	// Create the request.
	contentType, err := t.newRequest(repl)
	if err != nil {
		err := fmt.Errorf("failed preparing request for test %q: %s", t.Name, err.Error())
		t.Warnf(err.Error())
		return err
	}

	// Make a deep copy of the header and set standard stuff and cookies.
	t.request.Header = make(http.Header)
	for h, v := range t.Request.Header {
		rv := make([]string, len(v))
		for i := range v {
			rv[i] = repl.str.Replace(v[i])
		}
		t.request.Header[h] = rv

	}
	if t.request.Header.Get("Content-Type") == "" && contentType != "" {
		t.request.Header.Set("Content-Type", contentType)
	}
	if t.request.Header.Get("Accept") == "" {
		t.request.Header.Set("Accept", DefaultAccept)
	}
	if t.request.Header.Get("User-Agent") == "" {
		t.request.Header.Set("User-Agent", DefaultUserAgent)
	}
	for _, cookie := range t.Request.Cookies {
		cv := repl.str.Replace(cookie.Value)
		t.request.AddCookie(&http.Cookie{Name: cookie.Name, Value: cv})
	}

	// Compile the checks.
	t.checks = make([]check.Check, len(t.Checks))
	cfc, cfce := []int{}, []string{}
	for i := range t.Checks {
		t.checks[i] = check.SubstituteVariables(t.Checks[i], repl.str, repl.fn)
		if compiler, ok := t.checks[i].(check.Compiler); ok {
			e := compiler.Compile()
			if e != nil {
				cfc = append(cfc, i)
				cfce = append(cfce, err.Error())
				t.Warnf("Failed preparing check %d %q for test %q: %s",
					i, check.NameOf(t.Checks[i]), t.Name, err.Error())
			}
		}
	}
	if len(cfc) != 0 {
		err := fmt.Errorf("bogus checks %v: %s", cfc, strings.Join(cfce, "; "))
		return err
	}

	t.prepareClient()
	t.client.Jar = t.Jar
	return nil
}

// newRequest sets up the request field of t.
// If a sepcial Content-Type header is needed (e.g. because of a multipart
// body) it is returned.
func (t *Test) newRequest(repl replacer) (contentType string, err error) {
	// Prepare request method.
	if t.Request.Method == "" {
		t.Request.Method = "GET"
	}

	rurl := repl.str.Replace(t.Request.URL)
	urlValues := make(url.Values)
	for param, vals := range t.Request.Params {
		rv := make([]string, len(vals))
		for i, v := range vals {
			rv[i] = repl.str.Replace(v)
		}
		urlValues[param] = rv
	}

	var body io.ReadCloser
	if len(t.Request.Params) > 0 {
		if t.Request.ParamsAs == "body" || t.Request.ParamsAs == "multipart" {
			if t.Request.Method == "GET" || t.Request.Method == "HEAD" {
				err := fmt.Errorf("%s does not allow body or multipart parameters", t.Request.Method)
				return "", err
			}
			if t.Request.Body != "" {
				err := fmt.Errorf("body used with body/multipart parameters")
				return "", err
			}
		}
		switch t.Request.ParamsAs {
		case "URL", "":
			if strings.Index(rurl, "?") != -1 {
				rurl += "&" + urlValues.Encode()
			} else {
				rurl += "?" + urlValues.Encode()
			}
		case "body":
			contentType = "application/x-www-form-urlencoded"
			encoded := urlValues.Encode()
			body = ioutil.NopCloser(strings.NewReader(encoded))
		case "multipart":
			b, boundary, err := multipartBody(t.Request.Params)
			if err != nil {
				return "", err
			}
			body = b
			contentType = "multipart/form-data; boundary=" + boundary
		default:
			err := fmt.Errorf("unknown parameter method %q", t.Request.ParamsAs)
			return "", err
		}
	}

	// The body.
	if t.Request.Body != "" {
		rbody := repl.str.Replace(t.Request.Body)
		body = ioutil.NopCloser(strings.NewReader(rbody))
	}

	t.request, err = http.NewRequest(t.Request.Method, rurl, body)
	if err != nil {
		return "", err
	}

	return contentType, nil
}

var (
	followingClient    *http.Client
	nonfollowingClient *http.Client
)

// prepareClient sets up t's HTTP client.
func (t *Test) prepareClient() {
	// TODO: sprinkle gently with mutex
	if t.Timeout == 0 || t.Timeout == DefaultClientTimeout {
		if t.Request.FollowRedirects {
			if followingClient == nil || followingClient.Timeout != DefaultClientTimeout {
				followingClient = &http.Client{
					CheckRedirect: func(req *http.Request, via []*http.Request) error {
						if len(via) >= 10 {
							return errors.New("stopped after 10 redirects")
						}
						return nil
					},
					Timeout: DefaultClientTimeout,
				}
			}
			t.client = followingClient
		} else {
			if nonfollowingClient == nil || nonfollowingClient.Timeout != DefaultClientTimeout {
				nonfollowingClient = &http.Client{
					CheckRedirect: func(req *http.Request, via []*http.Request) error {
						return redirectNofollow
					},
					Timeout: DefaultClientTimeout,
				}
			}
			t.client = nonfollowingClient
		}
		return
	}

	// Tests with special timeouts get their own individual client.
	if t.Request.FollowRedirects {
		t.client = &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return errors.New("stopped after 10 redirects")
				}
				return nil
			},
			Timeout: t.Timeout,
		}
	} else {
		t.client = &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return redirectNofollow
			},
			Timeout: t.Timeout,
		}
	}
}

var (
	redirectNofollow = errors.New("we do not follow redirects")
)

// executeRequest performs the HTTP request defined in t which must have been
// prepared by Prepare. Executing an unprepared Test results will panic.
func (t *Test) executeRequest() (*response.Response, error) {
	t.Debugf("Test %q requesting %q", t.Name, t.request.URL)

	var err error
	start := time.Now()

	resp, err := t.client.Do(t.request)
	if ue, ok := err.(*url.Error); ok && ue.Err == redirectNofollow &&
		!t.Request.FollowRedirects {
		// Clear err if it is just our redirect non-following policy.
		err = nil
	}

	response := &response.Response{}
	response.Response = resp
	msg := "okay"
	if err == nil {
		response.Body, response.BodyErr = ioutil.ReadAll(resp.Body)
		resp.Body.Close()
	} else {
		msg = fmt.Sprintf("fail %s", err.Error())
	}
	response.Duration = time.Since(start)

	t.Debugf("Test %q request took %s %s", t.Name, response.Duration, msg)

	return response, err
}

func hasFailures(result []Result) bool {
	for _, r := range result {
		if r.Status == Fail || r.Status == Error {
			return true
		}
	}
	return false
}

// executeChecks applies the checks in t to the HTTP response received during
// executeRequest. A non-nil error is returned for bogus checks and checks
// which have errors: Just failing checks do not lead to non-nil-error
//
// Normally all checks in t.Checks are executed. If the first check in
// t.Checks is a StatusCode check against 200 and it fails, then the rest of
// the tests are skipped.
func (t *Test) executeChecks(response *response.Response) []Result {
	result := make([]Result, len(t.Checks))
	for i, ck := range t.Checks {
		start := time.Now()
		err := ck.Okay(response)
		result[i].Duration = time.Since(start)
		result[i].Error = err
		if err != nil {
			if _, ok := err.(check.MalformedCheck); ok {
				result[i].Status = Bogus
			} else {
				result[i].Status = Fail
			}
			if err != nil {
				t.Debugf("Check Failed Test=%q %d. Check=%s: %s",
					t.Name, i, check.NameOf(ck), err)
			}
			// Abort needles checking if all went wrong.
			if sc, ok := ck.(check.StatusCode); ok && i == 0 && sc.Expect == 200 {
				for j := 1; j < len(result); j++ {
					result[j].Status = Skipped
				}
				return result
			}
		} else {
			result[i].Status = Pass
		}
	}
	return result
}

func (t *Test) prepared() bool {
	return t.request != nil
}

// Benchmark executes t count many times and reports the outcome.
// Before doing the measurements warmup many request are made and discarded.
// Conc determines the concurrency level. If conc==1 the given pause
// is made between request. A conc > 1 will execute conc many request
// in paralell (without pauses).
// TODO: move this into an BenmarkOptions
func (t *Test) Benchmark(variables map[string]string, warmup int, count int, pause time.Duration, conc int) []Result {
	println("Test.Benchmark ", count)
	for n := 0; n < warmup; n++ {
		if n > 0 {
			time.Sleep(pause)
		}
		t.prepare(variables)
		t.executeRequest()
	}

	results := make([]Result, count)
	origPollMax := t.Poll.Max
	t.Poll.Max = 1

	if conc == 1 {
		// One request after the other, nicely spaced.
		for n := 0; n < count; n++ {
			time.Sleep(pause)
			results[n] = t.Run(variables)
		}
	} else {
		// Start conc request and restart an other once one finishes.
		rc := make(chan Result, conc)
		for i := 0; i < conc; i++ {
			go func() {
				rc <- t.Run(variables)
			}()
		}
		for j := 0; j < count; j++ {
			results[j] = <-rc
			go func() {
				rc <- t.Run(variables)
			}()
		}

	}
	t.Poll.Max = origPollMax

	return results
}

func CombinedStatus(result []Result) Status {
	status := NotRun
	for _, r := range result {
		if r.Status > status {
			status = r.Status
		}
	}
	return status
}

var skippedError = errors.New("Skipped")

// ----------------------------------------------------------------------------
// Multipart bodies
var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

// TODO: handle errors
func multipartBody(param map[string][]string) (io.ReadCloser, string, error) {
	var body *bytes.Buffer = &bytes.Buffer{}

	var mpwriter = multipart.NewWriter(body)
	// All non-file parameters come first
	for n, v := range param {
		if len(v) > 0 && strings.HasPrefix(v[0], "@file:") {
			continue // files go at the end
		}
		// TODO: handle errors
		if len(v) > 0 {
			for _, vv := range v {
				mpwriter.WriteField(n, vv)
			}
		} else {
			mpwriter.WriteField(n, "")
		}
	}

	// File parameters at bottom
	for n, v := range param {
		if !(len(v) > 0 && strings.HasPrefix(v[0], "@file:")) {
			continue // allready written
		}
		filename := v[0][6:]
		file, err := os.Open(filename)
		if err != nil {
			return nil, "", fmt.Errorf(
				"Unable to read file %q for multipart parameter %q: %s",
				filename, n, err.Error())
		}
		defer file.Close()
		basename := path.Base(filename)

		// Doing fw, err := mpwriter.CreateFormFile(n, basename) would
		// be much simpler but would fix the content type to
		// application/octet-stream. We can do a bit better.
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition",
			fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
				escapeQuotes(n), escapeQuotes(basename)))
		var ct string = "application/octet-stream"
		if i := strings.LastIndex(filename, "."); i != -1 {
			ct = mime.TypeByExtension(filename[i:])
			if ct == "" {
				ct = "application/octet-stream"
			}
		}
		h.Set("Content-Type", ct)
		fw, err := mpwriter.CreatePart(h)

		if err != nil {
			return nil, "", fmt.Errorf(
				"Unable to create part for parameter %q: %s",
				n, err.Error())
		}

		io.Copy(fw, file)
	}
	mpwriter.Close()

	return ioutil.NopCloser(body), mpwriter.Boundary(), nil
}
