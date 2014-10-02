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
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vdobler/ht/check"
	"github.com/vdobler/ht/response"
)

var (
	// DefaultUserAgent is the user agent string to send in http requests
	// if no user agent header is specified.
	DefaultUserAgent = "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/36.0.1985.143 Safari/537.36"

	// DefaultAccept is the accept header to be sent if no accept header
	// is defined in the test.
	DefaultAccept = "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8"

	// DefaultClientTimeout is the timeout used by the http clients.
	DefaultClientTimeout = 2 * time.Second
)

// Request is a HTTP request.
// The only required field is URL, all others have suitable defaults.
type Request struct {
	// Method is the HTTP method to use.
	// A empty method is equivalent to "GET"
	Method string `json:",omitempty"`

	// URL ist the URL of the request
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

// ----------------------------------------------------------------------------
// Test

// Test is a single logical test which does one HTTP request and checks
// a number of Checks on the recieved Result.
type Test struct {
	Name        string
	Description string
	Request     Request         // The HTTP request to test.
	Checks      check.CheckList // Checks contains all checks to perform on the Response

	// UnrollWith contains values to be used during unrolling the test
	// to several instances.
	UnrollWith map[string][]string

	Jar http.CookieJar `json:"-"` // The possible prepopulated cookie jar to use.
	Log *log.Logger    `json:"-"` // The logger to use by the test and the checks.

	Poll    Poll          `json:",omitempty"`
	Timeout time.Duration // If zero use DefaultClientTimeout

	client  *http.Client
	request *http.Request
	checks  []check.Check // compiled checks.

	verbose int
}

// Poll determines if and how to redo a test after a failure or if the
// test should be skipped alltogether.
type Poll struct {
	// Maximum number of redos. Both 0 and 1 mean: "Just one try. No redo."
	// Negative values indicate that the test should be skipped.
	Max int `json:",omitempty"`

	// Duration to sleep between redos.
	Sleep time.Duration `json:",omitempty"`
}

func (p Poll) Skip() bool {
	return p.Max < 0
}

type Cookie struct {
	Name  string
	Value string `json:",omitempty"`
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

// Repeat returns count copies of test with variables replaced based
// on vars. The keys of vars are the variable names. The values of a
// variable v are choosen from vars[v] by cycling through the list:
// In the n'th repetition is vars[v][n%N] with N=len(vars[v])).
func (test *Test) Repeat(count int, vars map[string][]string) []*Test {
	reps := make([]*Test, count)
	for r := 0; r < count; r++ {
		// Construct appropriate Replacer from variables.
		oldnew := make([]string, 0, 2*len(vars))
		for k, v := range vars {
			oldnew = append(oldnew, "{{"+k+"}}") // TODO: make configurable ??
			n := r % len(v)
			oldnew = append(oldnew, v[n])
		}
		replacer := strings.NewReplacer(oldnew...)
		reps[r] = test.SubstituteVariables(replacer)
	}
	return reps
}

// lcm computest the least common multiple of m and n.
func lcm(m, n int) int {
	a, b := m, n
	for a != b {
		if a < b {
			a += m
		} else {
			b += n
		}
	}
	return a
}

// lcmOf computes the least common multiple of the length of all valuesin vars.
func lcmOf(vars map[string][]string) int {
	n := 0
	for _, v := range vars {
		if n == 0 {
			n = len(v)
		} else {
			n = lcm(n, len(v))
		}
	}
	return n
}

// SubstituteVariables returns a copy of t with replacer applied.
func (t *Test) SubstituteVariables(replacer *strings.Replacer) *Test {
	// Apply to name, description, URL and body.
	c := &Test{
		Name:        replacer.Replace(t.Name),
		Description: replacer.Replace(t.Description),
		Request: Request{
			URL:  replacer.Replace(t.Request.URL),
			Body: replacer.Replace(t.Request.Body),
		},
	}

	// Apply to request parameters.
	c.Request.Params = make(url.Values)
	for param, vals := range t.Request.Params {
		rv := make([]string, len(vals))
		for i, v := range vals {
			rv[i] = replacer.Replace(v)
		}
		c.Request.Params[param] = rv
	}

	// Apply to http header.
	c.Request.Header = make(http.Header)
	for h, vals := range t.Request.Header {
		rv := make([]string, len(vals))
		for i, v := range vals {
			rv[i] = replacer.Replace(v)
		}
		c.Request.Header[h] = rv
	}

	// Apply to cookie values.
	for _, cookie := range t.Request.Cookies {
		rc := Cookie{Name: cookie.Name, Value: replacer.Replace(cookie.Value)}
		c.Request.Cookies = append(c.Request.Cookies, rc)
	}

	// Apply to checks.
	c.Checks = make([]check.Check, len(t.Checks))
	for i := range t.Checks {
		c.Checks[i] = check.SubstituteVariables(t.Checks[i], replacer)
	}

	return c
}

// Compile prepares the test for execution by substitutin the given
// variables and crafting the underlying http request and compiling
// the checks.
//
// It is possible to re-compile a test after changing fields in t.Request.
func (t *Test) Compile(variables map[string]string) error {
	// Create appropriate replace.
	nowVars := t.nowVariables(time.Now())
	allVars := MergeVariables(variables, nowVars)
	repl := NewReplacer(allVars)

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
			rv[i] = repl.Replace(v[i])
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
		cv := repl.Replace(cookie.Value)
		t.request.AddCookie(&http.Cookie{Name: cookie.Name, Value: cv})
	}

	// Compile the checks.
	t.checks = make([]check.Check, len(t.Checks))
	cfc, cfce := []int{}, []string{}
	for i := range t.Checks {
		t.checks[i] = check.SubstituteVariables(t.Checks[i], repl)
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
func (t *Test) newRequest(repl *strings.Replacer) (contentType string, err error) {
	// Prepare request method.
	if t.Request.Method == "" {
		t.Request.Method = "GET"
	}

	rurl := repl.Replace(t.Request.URL)
	urlValues := make(url.Values)
	for param, vals := range t.Request.Params {
		rv := make([]string, len(vals))
		for i, v := range vals {
			rv[i] = repl.Replace(v)
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
		body = ioutil.NopCloser(strings.NewReader(repl.Replace(t.Request.Body)))
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

// prepareClient sets up t's http client.
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

// ExecuteRequest makes the HTTP request defined in t and measures the response time.
func (t *Test) ExecuteRequest() (*response.Response, error) {
	if !t.prepared() {
		log.Fatalf("ExecuteRequest on unprepared test %q", t.Name)
	}
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

// ExecuteChecks applies the checks in t to the HTTP response received during
// ExecuteRequest. A non-nil error is returned for bogus checks and checks
// which have errors: Just failing checks do not lead to non-nil-error
//
// Normally all checks in t.Checks are executed. If the first check in
// t.Checks is a StatusCode check against 200 and it fails, then the rest of
// the tests are skipped.
func (t *Test) ExecuteChecks(response *response.Response) []Result {
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

// Run the given test: Execute the request and perform the test until success or
// test.Poll.Max tries are exceeded.
func (t *Test) Run() Result {
	var result Result
	if !t.prepared() {
		err := t.Compile(nil) // TODO
		if err != nil {
			result.Status, result.Error = Bogus, err
			return result
		}
	}

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
		response, err := t.ExecuteRequest()
		if err == nil {
			result.Elements = t.ExecuteChecks(response)
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

// Benchmark executes t count many times and reports the outcome.
// Before doing the measurements warmup many request are made and discarded.
// Conc determines the concurrency level. If conc==1 the given pause
// is made between request. A conc > 1 will execute conc many request
// in paralell (without pauses).
func (t *Test) Benchmark(warmup int, count int, pause time.Duration, conc int) []Result {
	println("Test.Benchmark ", count)
	for n := 0; n < warmup; n++ {
		if n > 0 {
			time.Sleep(pause)
		}
		t.ExecuteRequest()
	}

	results := make([]Result, count)
	origPollMax := t.Poll.Max
	t.Poll.Max = 1

	if conc == 1 {
		// One request after the other, nicely spaced.
		for n := 0; n < count; n++ {
			time.Sleep(pause)
			results[n] = t.Run()
		}
	} else {
		// Start conc request and restart an other once one finishes.
		rc := make(chan Result, conc)
		for i := 0; i < conc; i++ {
			go func() {
				rc <- t.Run()
			}()
		}
		for j := 0; j < count; j++ {
			results[j] = <-rc
			rc <- t.Run()
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

// CheckError contains the results of all checks of a test.
type CheckError []error

func (ce CheckError) Error() string {
	s := ""
	for i, e := range ce {
		s += fmt.Sprintf("Check %d:", i)
		if e == nil {
			s += "<<Pass>>"
		} else {
			s += e.Error()
		}
		if i < len(ce)-1 {
			s += "\t"
		}
	}
	return s
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

// ----------------------------------------------------------------------------
// Variable substitutions

var nowTimeRe = regexp.MustCompile(`{{NOW *([+-] *[1-9][0-9]*[smhd])? *(\| *"(.*)")?}}`)

// findNowVariables return all occurences of a time-variable.
func (t *Test) findNowVariables() (v []string) {
	add := func(s string) {
		m := nowTimeRe.FindAllString(s, 1)
		if m == nil {
			return
		}
		v = append(v, m[0])
	}

	add(t.Name)
	add(t.Description)
	add(t.Request.URL)
	add(t.Request.Body)
	for _, pp := range t.Request.Params {
		for _, p := range pp {
			add(p)
		}
	}
	for _, hh := range t.Request.Header {
		for _, h := range hh {
			add(h)
		}
	}
	for _, cookie := range t.Request.Cookies {
		add(cookie.Value)
	}
	for _, c := range t.Checks {
		v = append(v, findNV(c)...)
	}
	return v
}

// nowVariables looks through t, extracts all occurences of now variables, i.e.
//     {{NOW + 30s | "2006-Jan-02"}}
// and formats the desired time. It returns a map suitable for merging with
// other, real variable/value-Pairs.
func (t *Test) nowVariables(now time.Time) (vars map[string]string) {
	nv := t.findNowVariables()
	vars = make(map[string]string)
	for _, k := range nv {
		m := nowTimeRe.FindAllStringSubmatch(k, 1)
		if m == nil {
			panic("Unmatchable " + k)
		}
		kk := k[2 : len(k)-2] // Remove {{ and }} to produce the "variable name".
		if _, ok := vars[kk]; ok {
			continue // We already processed this variable.
		}
		var off time.Duration
		delta := m[0][1]
		if delta != "" {
			num := strings.TrimLeft(delta[1:len(delta)-1], " ")
			n, err := strconv.Atoi(num)
			if err != nil {
				panic(err)
			}
			if delta[0] == '-' {
				n *= -1
			}
			switch delta[len(delta)-1] {
			case 'm':
				n *= 60
			case 'h':
				n *= 60 * 60
			case 'd':
				n *= 24 * 26 * 60
			}
			off = time.Duration(n) * time.Second
		}
		format := time.RFC1123
		if m[0][3] != "" {
			format = m[0][3]
		}
		formatedTime := now.Add(off).Format(format)
		vars[kk] = formatedTime
	}
	return vars
}

// MergeVariables merges all variables found in vars.
func MergeVariables(vars ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, e := range vars {
		for k, v := range e {
			result[k] = v
		}
	}
	return result
}

// NewReplacer produces a replacer to perform substitution of the
// given variables with their values. The keys of vars are the variable
// names and the replacer subsitutes "{{k}}" with vars[k] for each key
// in vars.
func NewReplacer(vars map[string]string) *strings.Replacer {
	oldnew := make([]string, 0, 2*len(vars))
	for k, v := range vars {
		oldnew = append(oldnew, "{{"+k+"}}") // TODO: make configurable ??
		oldnew = append(oldnew, v)
	}

	replacer := strings.NewReplacer(oldnew...)
	return replacer
}
