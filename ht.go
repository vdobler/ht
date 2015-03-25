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
	"sync"
	"time"

	"github.com/vdobler/ht/check"
	"github.com/vdobler/ht/response"
	"github.com/yosuke-furukawa/json5/encoding/json5"
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
	Params url.Values `json:",omitempty"`

	// ParamsAs determines how the parameters in the Param field are sent:
	//   "URL" or "": append properly encoded to URL
	//   "body"     : send as application/x-www-form-urlencoded in body.
	//   "multipart": send as multipart in body.
	// The two values "body" and "multipart" must not be used
	// on a GET or HEAD request.
	ParamsAs string `json:",omitempty"`

	// Header contains the specific http headers to be sent in this request.
	// User-Agent and Accept headers are set automaticaly to the global
	// default values if not set explicitely.
	Header http.Header `json:",omitempty"`

	// Cookies contains the cookies to send in the request.
	Cookies []Cookie `json:",omitempty"`

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
// a number of Checks on the recieved Response.
type Test struct {
	Name        string
	Description string

	// Request is the HTTP request.
	Request Request

	// Checks contains all checks to perform on the response to the HTTP request.
	Checks check.CheckList

	Poll      Poll          `json:",omitempty"`
	Timeout   time.Duration // If zero use DefaultClientTimeout.
	Verbosity int           // Verbosity level in logging.

	// ClientPool allows to inject special http.Transports or a
	// cookie jar to be used by this Test.
	ClientPool *ClientPool

	client   *http.Client
	request  *http.Request
	response *response.Response
	checks   []check.Check // compiled checks.
}

// ClientPool maintains a pool of clients for the given transport
// and cookie jar. ClientPools must not be copied.
type ClientPool struct {
	Transport http.RoundTripper
	Jar       http.CookieJar

	mu      sync.Mutex
	clients map[time.Duration]*http.Client
}

// get retreives a new or existing http.Client for the given timeout and
// redirect following policy.
func (p *ClientPool) Get(timeout time.Duration, followRedirects bool) *http.Client {
	if timeout == 0 {
		log.Fatalln("ClientPool.Get called with zero timeout.")
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.clients) == 0 {
		p.clients = make(map[time.Duration]*http.Client)
	}

	key := timeout
	if followRedirects {
		key = -key
	}

	if client, ok := p.clients[key]; ok {
		return client
	}

	var client *http.Client
	if followRedirects {
		client = &http.Client{CheckRedirect: doFollowRedirects, Timeout: timeout}
	} else {
		client = &http.Client{CheckRedirect: dontFollowRedirects, Timeout: timeout}
	}
	if p.Jar != nil {
		client.Jar = p.Jar
	}
	if p.Transport != nil {
		client.Transport = p.Transport
	}

	p.clients[key] = client
	return client
}

func doFollowRedirects(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	return nil
}

func dontFollowRedirects(req *http.Request, via []*http.Request) error {
	return redirectNofollow
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

func (t *Test) errorf(format string, v ...interface{}) {
	if t.Verbosity >= 0 {
		format = "ERROR " + format + " [%q]"
		v = append(v, t.Name)
		log.Printf(format, v...)
	}
}

func (t *Test) infof(format string, v ...interface{}) {
	if t.Verbosity >= 1 {
		format = "INFO " + format + " [%q]"
		v = append(v, t.Name)
		log.Printf(format, v...)
	}
}

func (t *Test) debugf(format string, v ...interface{}) {
	if t.Verbosity >= 2 {
		format = "DEBUG " + format + " [%q]"
		v = append(v, t.Name)
		log.Printf(format, v...)
	}
}

func (t *Test) tracef(format string, v ...interface{}) {
	if t.Verbosity >= 3 {
		format = "TRACE " + format + " [%q]"
		v = append(v, t.Name)
		log.Printf(format, v...)
	}
}

// Run runs the test t. The actual HTTP request is crafted and executed and
// the checks are performed on the received response. This whole process
// is repeated on failure or skipped entirely according to t.Poll.
//
// Normally all checks in t.Checks are executed. If the first check in
// t.Checks is a StatusCode check against 200 and it fails, then the rest of
// the tests are skipped.
func (t *Test) Run(variables map[string]string) TestResult {
	// Set up a TestResult, prefill static information
	result := TestResult{
		Name:        t.Name,
		Description: t.Description,
		Started:     time.Now(),
	}
	result.CheckResults = make([]CheckResult, len(t.Checks)) // Zero value is NotRun
	for i, c := range t.Checks {
		result.CheckResults[i].Name = check.NameOf(c)
		buf, err := json5.Marshal(c)
		if err != nil {
			buf = []byte(err.Error())
		}
		result.CheckResults[i].JSON = string(buf)
	}

	maxTries := t.Poll.Max
	if maxTries == 0 {
		maxTries = 1
	}
	if maxTries < 0 {
		// This test is deliberately skipped. A zero duration is okay.
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
		t.execute(&result)
		if result.Status == Pass {
			break
		}
	}
	result.FullDuration = time.Since(start)
	result.Tries = try + 1
	if t.Poll.Max > 1 {
		if result.Status == Pass {
			t.debugf("polling succeded after %d tries", try+1)
		} else {
			t.debugf("polling failed all %d tries", maxTries)
		}
	}

	t.infof("test %s (%s %s)", result.Status, result.FullDuration, result.Response.Duration)

	return result
}

// execute does a single request and check the response.
func (t *Test) execute(result *TestResult) {
	response, err := t.executeRequest()
	if err == nil {
		if len(t.Checks) > 0 {
			t.executeChecks(response, result.CheckResults)
			result.Status = result.CombineChecks()
		} else {
			result.Status = Pass
		}
	} else {
		result.Status = Error
		result.Error = err
	}
	result.Response = response
	result.Duration = response.Duration
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
		err := fmt.Errorf("failed preparing request: %s", err.Error())
		t.errorf("%s", err.Error())
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
				t.errorf("preparing check %d %q: %s",
					i, check.NameOf(t.Checks[i]), err.Error())
			}
		}
	}
	if len(cfc) != 0 {
		err := fmt.Errorf("bogus checks %v: %s", cfc, strings.Join(cfce, "; "))
		return err
	}

	to := DefaultClientTimeout
	if t.Timeout > 0 {
		to = t.Timeout
	}
	if t.ClientPool != nil {
		t.tracef("Taking client from pool")
		t.client = t.ClientPool.Get(to, t.Request.FollowRedirects)
	} else if t.Request.FollowRedirects {
		t.client = &http.Client{CheckRedirect: doFollowRedirects, Timeout: to}
	} else {
		t.client = &http.Client{CheckRedirect: dontFollowRedirects, Timeout: to}
	}
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
	redirectNofollow = errors.New("we do not follow redirects")
)

// executeRequest performs the HTTP request defined in t which must have been
// prepared by Prepare. Executing an unprepared Test results will panic.
func (t *Test) executeRequest() (*response.Response, error) {
	t.debugf("requesting %q", t.request.URL.String())

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

	t.debugf("request took %s, %s", response.Duration, msg)

	return response, err
}

// executeChecks applies the checks in t to the HTTP response received during
// executeRequest. A non-nil error is returned for bogus checks and checks
// which have errors: Just failing checks do not lead to non-nil-error
//
// Normally all checks in t.Checks are executed. If the first check in
// t.Checks is a StatusCode check against 200 and it fails, then the rest of
// the tests are skipped.
func (t *Test) executeChecks(response *response.Response, result []CheckResult) {
	for i, ck := range t.Checks {
		start := time.Now()
		err := ck.Okay(response)
		result[i].Duration = time.Since(start)
		result[i].Error = err
		if err != nil {
			t.debugf("check %d %s failed: %s", i, check.NameOf(ck), err)
			if _, ok := err.(check.MalformedCheck); ok {
				result[i].Status = Bogus
			} else {
				result[i].Status = Fail
			}
			// Abort needles checking if all went wrong.
			if sc, ok := ck.(check.StatusCode); ok && i == 0 && sc.Expect == 200 {
				t.tracef("skipping remaining tests")
				// Clear Status and Error field as these might be
				// populated from a prioer try run of the test.
				for j := 1; j < len(result); j++ {
					result[j].Status = Skipped
					result[j].Error = nil
				}
			}
		} else {
			result[i].Status = Pass
			t.tracef("check %d %s: Pass", i, check.NameOf(ck))
		}
	}
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
func (t *Test) Benchmark(variables map[string]string, warmup int, count int, pause time.Duration, conc int) []TestResult {
	for n := 0; n < warmup; n++ {
		if n > 0 {
			time.Sleep(pause)
		}
		t.prepare(variables)
		t.executeRequest()
	}

	results := make([]TestResult, count)
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
		rc := make(chan TestResult, conc)
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
