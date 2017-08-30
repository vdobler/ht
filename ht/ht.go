// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vdobler/ht/cookiejar"
)

var (
	// DefaultUserAgent is the user agent string to send in http requests
	// if no user agent header is specified explicitly.
	DefaultUserAgent = "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/36.0.1985.143 Safari/537.36"

	// DefaultAccept is the accept header to be sent if no accept header
	// is set explicitly in the test.
	DefaultAccept = "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8"

	// DefaultClientTimeout is the timeout used by the http clients.
	DefaultClientTimeout = 10 * time.Second
)

// Transport is the http Transport used while making requests.
// It is exposed to allow different Timeouts, less idle connections
// or laxer TLS settings.
var Transport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}).DialContext,
	MaxIdleConns:    100,
	IdleConnTimeout: 90 * time.Second,
	TLSClientConfig: &tls.Config{
		InsecureSkipVerify: false,
	},
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

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
	// files by special formated values.
	// The following formats are recognized:
	//    @file:/path/to/thefile
	//         read in /path/to/thefile and use its content as the
	//         parameter value. The path may be relative.
	//    @vfile:/path/to/thefile
	//         read in /path/to/thefile and perform variable substitution
	//         in its content to yield the parameter value.
	//    @file:@name-of-file:direct-data
	//    @vfile:@name-of-file:direct-data
	//         use direct-data as the parameter value and name-of-file
	//         as the filename. (There is no difference between the
	//         @file and @vfile variants; variable substitution has
	//         been performed already and is not done twice on direct-data.
	Params url.Values

	// ParamsAs determines how the parameters in the Param field are sent:
	//   "URL" or "": append properly encoded to URL
	//   "body"     : send as application/x-www-form-urlencoded in body.
	//   "multipart": send as multipart/form-data in body.
	// The two values "body" and "multipart" must not be used
	// on a GET or HEAD request.
	ParamsAs string `json:",omitempty"`

	// Header contains the specific http headers to be sent in this request.
	// User-Agent and Accept headers are set automaticaly to the global
	// default values if not set explicitly.
	Header http.Header `json:",omitempty"`

	// Cookies contains the cookies to send in the request.
	Cookies []Cookie `json:",omitempty"`

	// Body is the full body to send in the request. Body must be
	// empty if Params are sent as multipart or form-urlencoded.
	Body string `json:",omitempty"`

	// FollowRedirects determines if automatic following of
	// redirects should be done.
	FollowRedirects bool `json:",omitempty"`

	// BasicAuthUser and BasicAuthPass contain optional username and
	// password which will be sent in a Basic Authentication header.
	// If following redirects the authentication header is also sent
	// on subsequent requests to the same host.
	BasicAuthUser string `json:",omitempty"`
	BasicAuthPass string `json:",omitempty"`

	// Chunked turns of setting of the Content-Length header resulting
	// in chunked transfer encoding of POST bodies.
	Chunked bool `json:",omitempty"`

	// Timeout of this request. If zero use DefaultClientTimeout.
	Timeout time.Duration `json:",omitempty"`

	Request    *http.Request `json:"-"` // the 'real' request
	SentBody   string        `json:"-"` // the 'real' body
	SentParams url.Values    `json:"-"` // the 'real' parameters
}

// Response captures information about a http response.
type Response struct {
	// Response is the received HTTP response. Its body has bean read and
	// closed already.
	Response *http.Response `json:",omitempty"`

	// Duration to receive response and read the whole body.
	Duration time.Duration `json:",omitempty"`

	// The received body and the error got while reading it.
	BodyStr string `json:",omitempty"`
	BodyErr error  `json:",omitempty"`

	// Redirections records the URLs of automatic GET requests due to redirects.
	Redirections []string `json:",omitempty"`
}

// Body returns a reader of the response body.
func (resp *Response) Body() io.Reader {
	return strings.NewReader(resp.BodyStr)
}

// Cookie is a HTTP cookie.
type Cookie struct {
	Name  string
	Value string `json:",omitempty"`
}

// Execution contains parameters controlling the test execution.
type Execution struct {
	// Tries is the maximum number of tries made for this test.
	// Both 0 and 1 mean: "Just one try. No redo."
	// Negative values indicate that the test should be skipped
	// altogether.
	Tries int `json:",omitempty"`

	// Wait time between retries.
	Wait time.Duration `json:",omitempty"`

	// Pre-, Inter- and PostSleep are the sleep durations made
	// before the request, between request and the checks and
	// after the checks.
	PreSleep, InterSleep, PostSleep time.Duration `json:",omitempty"`

	// Verbosity level in logging.
	Verbosity int `json:",omitempty"`
}

// ----------------------------------------------------------------------------
// Test

// Test is a single logical test which does one HTTP request and checks
// a number of Checks on the received Response.
type Test struct {
	// Name of the test.
	Name string

	// Description what this test's intentions are.
	Description string `json:",omitempty"`

	// Request is the HTTP request.
	Request Request

	// Response to the Request
	Response Response `json:",omitempty"`

	// Checks contains all checks to perform on the response to the HTTP request.
	Checks CheckList

	// VarEx may be used to popultate variables from the response. TODO: Rename.
	VarEx ExtractorMap // map[string]Extractor `json:",omitempty"`

	// ExValues contains the result of the extractions.
	ExValues map[string]Extraction `json:",omitempty"`

	// Execution controls the test execution.
	Execution Execution `json:",omitempty"`

	// Jar is the cookie jar to use
	Jar *cookiejar.Jar `json:"-"`

	// Variables contains name/value-pairs used for variable substitution
	// in files read in, e.g. for Request.Body = "@vfile:/path/to/file".
	Variables map[string]string `json:",omitempty"`

	// The following results are filled during Run.
	// This should be collected into something like struct TestResult{...}.
	Status       Status        `json:"-"`
	Started      time.Time     `json:"-"`
	Error        error         `json:"-"`
	Duration     time.Duration `json:"-"`
	FullDuration time.Duration `json:"-"`
	Tries        int           `json:"-"`
	CheckResults []CheckResult `json:"-"` // The individual checks.

	// Log is the logger to use.
	Log interface {
		Printf(format string, a ...interface{})
	} `json:"-"`

	client *http.Client

	// metadata allows to attach additional data to a Test.
	metadata map[string]interface{}
}

// Disable disables t by setting the maximum number of tries to -1.
func (t *Test) Disable() {
	t.Execution.Tries = -1
}

// SetMetadata attaches value to t under the given key.
func (t *Test) SetMetadata(key string, value interface{}) {
	if t.metadata == nil {
		t.metadata = make(map[string]interface{})
	}
	t.metadata[key] = value
}

// GetMetadata returns the meta data from t associated with the given
// key or nil if no such key has been assiciated.
func (t *Test) GetMetadata(key string) interface{} {
	if t.metadata == nil {
		return nil
	}
	return t.metadata[key]
}

// GetStringMetadata returns the meta data associated with t for the
// given key or the empty string if no data was associated. It panics if
// the meta data for key is not a string.
func (t *Test) GetStringMetadata(key string) string {
	if t.metadata == nil {
		return ""
	}
	if v, ok := t.metadata[key]; ok {
		return v.(string)
	}
	return ""
}

// GetIntMetadata returns the meta data associated with t for the
// given key or 0 if no data was associated. It panics if the meta
// data for key is not an int.
func (t *Test) GetIntMetadata(key string) int {
	if t.metadata == nil {
		return 0
	}
	if v, ok := t.metadata[key]; ok {
		return v.(int)
	}
	return 0
}

// CheckResult captures the outcome of a single check inside a test.
type CheckResult struct {
	Name     string        // Name of the check as registered.
	JSON     string        // JSON serialization of check.
	Status   Status        // Outcome of check. All status but Error
	Duration time.Duration // How long the check took.
	Error    ErrorList     // For a Status of Bogus or Fail.
}

// Extraction captures the result of a variable extraction.
type Extraction struct {
	Value string
	Error error
}

// mergeRequest implements the merge strategy described in Merge for the Request.
func mergeRequest(m *Request, r Request) error {
	allNonemptyMustBeSame := func(m *string, s string) error {
		if s != "" {
			if *m != "" && *m != s {
				return fmt.Errorf("Cannot merge %q into %q", s, *m)
			}
			*m = s
		}
		return nil
	}
	onlyOneMayBeNonempty := func(m *string, s string) error {
		if s != "" {
			if *m != "" {
				return fmt.Errorf("Won't overwrite %q with %q", *m, s)
			}
			*m = s
		}
		return nil
	}

	if err := allNonemptyMustBeSame(&(m.Method), r.Method); err != nil {
		return err
	}

	if err := onlyOneMayBeNonempty(&(m.URL), r.URL); err != nil {
		return err
	}

	for k, v := range r.Params {
		m.Params[k] = append(m.Params[k], v...)
	}

	if err := allNonemptyMustBeSame(&(m.ParamsAs), r.ParamsAs); err != nil {
		return err
	}

	for k, v := range r.Header {
		m.Header[k] = append(m.Header[k], v...)
	}

outer:
	for _, rc := range r.Cookies {
		for i := range m.Cookies {
			if m.Cookies[i].Name == rc.Name {
				m.Cookies[i].Value = rc.Value
				continue outer
			}
		}
		m.Cookies = append(m.Cookies, rc)
	}

	if err := onlyOneMayBeNonempty(&(m.Body), r.Body); err != nil {
		return err
	}

	m.FollowRedirects = r.FollowRedirects
	m.Chunked = r.Chunked

	if err := onlyOneMayBeNonempty(&(m.BasicAuthUser), r.BasicAuthUser); err != nil {
		return err
	}
	if err := onlyOneMayBeNonempty(&(m.BasicAuthPass), r.BasicAuthPass); err != nil {
		return err
	}

	return nil
}

// Merge merges all tests into one. The individual fields are merged in the
// following way.
//     Name         Join all names
//     Description  Join all descriptions
//     Request
//       Method     All nonempty must be the same
//       URL        Only one may be nonempty
//       Params     Merge by key
//       ParamsAs   All nonempty must be the same
//       Header     Merge by key
//       Cookies    Merge by cookie name
//       Body       Only one may be nonempty
//       FollowRdr  Last wins
//       Chunked    Last wins
//     Checks       Append all checks
//     VarEx        Merge, same keys must have same value
//     TestVars     Use values from first only.
//     Poll
//       Max        Use largest
//       Sleep      Use largest
//     Timeout      Use largets
//     Verbosity    Use largets
//     PreSleep     Summ of all;  same for InterSleep and PostSleep
//     ClientPool   ignore
func Merge(tests ...*Test) (*Test, error) {
	m := Test{}

	// Name and description
	s := []string{}
	for _, t := range tests {
		s = append(s, t.Name)
	}
	m.Name = "Merge of " + strings.Join(s, ", ")
	s = s[:0]
	for _, t := range tests {
		s = append(s, t.Description)
	}
	m.Description = strings.TrimSpace(strings.Join(s, "\n"))

	m.Variables = make(map[string]string)
	for n, v := range tests[0].Variables {
		m.Variables[n] = v
	}

	m.Request.Params = make(url.Values)
	m.Request.Header = make(http.Header)
	m.VarEx = make(map[string]Extractor)
	for _, t := range tests {
		err := mergeRequest(&m.Request, t.Request)
		if err != nil {
			return &m, err
		}
		m.Checks = append(m.Checks, t.Checks...)
		if t.Execution.Tries > m.Execution.Tries {
			m.Execution.Tries = t.Execution.Tries
		}
		if t.Execution.Wait > m.Execution.Wait {
			m.Execution.Wait = t.Execution.Wait
		}
		if t.Request.Timeout > m.Request.Timeout {
			m.Request.Timeout = t.Request.Timeout
		}
		if t.Execution.Verbosity > m.Execution.Verbosity {
			m.Execution.Verbosity = t.Execution.Verbosity
		}
		m.Execution.PreSleep += t.Execution.PreSleep
		m.Execution.InterSleep += t.Execution.InterSleep
		m.Execution.PostSleep += t.Execution.PostSleep
		for name, value := range t.VarEx {
			if old, ok := m.VarEx[name]; ok && old != value {
				return &m, fmt.Errorf("wont overwrite extractor for %s", name)
			}
			m.VarEx[name] = value
		}
	}

	return &m, nil
}

// PopulateCookies populates t.Request.Cookies with the those
// cookies from jar which would be sent to u.
func (t *Test) PopulateCookies(jar *cookiejar.Jar, u *url.URL) {
	if jar == nil || u == nil {
		return
	}

	for _, cookie := range jar.Cookies(u) {
		t.Request.Cookies = append(t.Request.Cookies,
			Cookie{Name: cookie.Name, Value: cookie.Value})
	}
}

// AsJSON returns a JSON representation of the test. Executed tests can
// be serialised and will contain basically all information required to
// debug or re-run the test but note that several fields in the actual
// *http.Request and *http.Response structs are cleared during this
// serialisation.
func (t *Test) AsJSON() ([]byte, error) {
	// Nil some un-serilisable stuff.
	t.Jar = nil
	t.client = nil
	if t.Request.Request != nil {
		t.Request.Request.Body = nil
		t.Request.Request.PostForm = nil
		t.Request.Request.MultipartForm = nil
		t.Request.Request.TLS = nil
		t.Request.Request.Close = false
	}
	if t.Response.Response != nil {
		t.Response.Response.TLS = nil
		t.Response.Response.Body = nil
	}

	return json.MarshalIndent(t, "", "    ")
}

// Run runs the test t. The actual HTTP request is crafted and executed and
// the checks are performed on the received response. This whole process
// is repeated on failure or skipped entirely according to t.Execution.
//
// Normally all checks in t.Checks are executed. If the first check in
// t.Checks is a StatusCode check against 200 and it fails, then the rest of
// the tests are skipped.
//
// Run returns a non-nil error only if the test is bogus; a failing http
// request, problems reading the body or any failing checks do not trigger a
// non-nil return value.
func (t *Test) Run() error {
	t.Started = time.Now()
	defer func() { t.FullDuration = time.Since(t.Started) }()

	t.infof("Running")

	if t.Execution.Tries < 0 {
		// This test is deliberately skipped.
		t.Status = Skipped
		return nil
	} else if t.Execution.Tries == 0 {
		t.Execution.Tries = 1
	}

	// Prepare checks and request. Both may declare the Test to be bogus.
	err := t.PrepareChecks()
	if err != nil {
		t.Status, t.Error = Bogus, err
		return err
	}
	err = t.prepareRequest()
	if err != nil {
		t.Status, t.Error = Bogus, err
		return err
	}

	if t.Execution.PreSleep > 0 {
		t.debugf("PreSleep %s", t.Execution.PreSleep)
		time.Sleep(t.Execution.PreSleep)
	}

	// Try until first success.
	start := time.Now()
	try := 1
	for ; try <= t.Execution.Tries; try++ {
		t.Tries = try
		if try > 1 {
			t.infof("Retry %d", try)
			if t.Execution.Wait > 0 {
				t.debugf("Waiting %s", t.Execution.Wait)
				time.Sleep(t.Execution.Wait)
			}
		}
		t.resetRequest()
		// Clear status and error; is updated in executeChecks.
		t.Status, t.Error = NotRun, nil
		t.Response = Response{}
		t.execute()
		if t.Status == Pass {
			break
		}
	}
	t.Duration = time.Since(start)
	if t.Execution.Tries > 1 {
		if t.Status == Pass {
			t.debugf("Trying succeeded after %d tries", t.Tries)
		} else {
			t.debugf("Trying failed all %d tries", t.Execution.Tries)
		}
	}

	t.infof("Result: %s (%s %s) %d tries", t.Status,
		t.Duration, t.Response.Duration, t.Tries)

	if t.Execution.PostSleep > 0 {
		t.debugf("PostSleep %s", t.Execution.PostSleep)
		time.Sleep(t.Execution.PostSleep)
	}

	return nil
}

// execute does a single request and check the response.
func (t *Test) execute() {
	var err error
	switch t.Request.Request.URL.Scheme {
	case "file":
		err = t.executeFile()
	case "http", "https":
		err = t.executeRequest()
	case "bash":
		err = t.executeBash()
	case "sql":
		err = t.executeSQL()
		if _, ok := err.(bogusSQLQuery); ok {
			t.Status = Bogus
			t.Error = err
			return
		}
	default:
		t.Status = Bogus
		t.Error = fmt.Errorf("ht: unrecognized URL scheme %q", t.Request.Request.URL.Scheme)
		return
	}
	if err == nil {
		if len(t.Checks) > 0 {
			if t.Execution.InterSleep > 0 {
				t.debugf("InterSleep %s", t.Execution.InterSleep)
				time.Sleep(t.Execution.InterSleep)
			}
			t.ExecuteChecks()
		} else {
			t.Status = Pass
		}
	} else {
		t.Status = Error
		t.Error = err
	}
}

// PrepareChecks call Prepare() on all preparbel checks and sets up t
// for execution.
//
// TODO: clear CheckResults before Prepare
// TODO: identify Nr of unpreparable check.
func (t *Test) PrepareChecks() error {
	// Compile the checks.
	cel := ErrorList{}
	for i := range t.Checks {
		if prep, ok := t.Checks[i].(Preparable); ok {
			e := prep.Prepare(t)
			if e != nil {
				cel = append(cel, e)
				t.errorf("preparing check %d %q: %s",
					i, NameOf(t.Checks[i]), e.Error())
			}
		}
	}
	if len(cel) != 0 {
		return cel
	}

	// Prepare CheckResults.
	t.CheckResults = make([]CheckResult, len(t.Checks)) // Zero value is NotRun
	for i, c := range t.Checks {
		t.CheckResults[i].Name = NameOf(c)
		buf, err := json.Marshal(c)
		if err != nil {
			buf = []byte(err.Error())
		}
		t.CheckResults[i].JSON = string(buf)
	}

	return nil
}

// resetRequest rewinds the request body reader.
func (t *Test) resetRequest() {
	if t.Request.SentBody == "" {
		return
	}
	body := ioutil.NopCloser(strings.NewReader(t.Request.SentBody))
	t.Request.Request.Body = body
}

// prepare the test for execution by crafting the underlying http request
func (t *Test) prepareRequest() error {
	// Create the request.
	contentType, err := t.newRequest()
	if err != nil {
		err = fmt.Errorf("failed preparing request: %s", err.Error())
		t.errorf("%s", err.Error())
		return err
	}

	// Prepare the HTTP header. TODO: Deep Coppy??
	for h, v := range t.Request.Header {
		rv := make([]string, len(v))
		copy(rv, v)
		t.Request.Request.Header[h] = rv

	}
	if t.Request.Request.Header.Get("Content-Type") == "" && contentType != "" {
		t.Request.Request.Header.Set("Content-Type", contentType)
	}
	if t.Request.Request.Header.Get("Accept") == "" {
		t.Request.Request.Header.Set("Accept", DefaultAccept)
	}
	if t.Request.Request.Header.Get("User-Agent") == "" {
		t.Request.Request.Header.Set("User-Agent", DefaultUserAgent)
	}
	for _, cookie := range t.Request.Cookies {
		t.Request.Request.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	}
	// Basic Auth
	if t.Request.BasicAuthUser != "" {
		t.Request.Request.SetBasicAuth(t.Request.BasicAuthUser, t.Request.BasicAuthPass)
	}

	if t.Request.Timeout <= 0 {
		t.Request.Timeout = DefaultClientTimeout
	}

	if t.Request.FollowRedirects {
		cr := func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			if req.URL.Host == t.Request.Request.URL.Host &&
				t.Request.BasicAuthUser != "" {
				if user, pass, ok := t.Request.Request.BasicAuth(); ok {
					req.SetBasicAuth(user, pass)
				}
			}
			t.Response.Redirections = append(t.Response.Redirections, req.URL.String())
			return nil
		}
		t.client = &http.Client{
			Transport:     Transport,
			CheckRedirect: cr,
			Timeout:       t.Request.Timeout,
		}
	} else {
		t.client = &http.Client{
			Transport:     Transport,
			CheckRedirect: dontFollowRedirects,
			Jar:           nil,
			Timeout:       t.Request.Timeout,
		}
	}
	if t.Jar != nil {
		t.client.Jar = t.Jar
	}

	return nil
}

// newRequest sets up the request field of t.
// If a sepcial Content-Type header is needed (e.g. because of a multipart
// body) it is returned.
func (t *Test) newRequest() (contentType string, err error) {
	// Set efaults for the request method and the parameter transmission type.
	if t.Request.Method == "" {
		t.Request.Method = http.MethodGet
	}
	if t.Request.ParamsAs == "" {
		t.Request.ParamsAs = "URL"
	}

	rurl := t.Request.URL
	prurl, err := url.Parse(rurl)
	if err != nil {
		return "", err
	}
	t.Request.SentParams = prurl.Query()

	// Deep copy. TODO might be unnecessary.
	urlValues := make(url.Values)
	for param, vals := range t.Request.Params {
		rv := make([]string, len(vals))
		for i, v := range vals {
			rv[i] = v
			t.Request.SentParams.Add(param, v)
		}
		urlValues[param] = rv
	}

	if len(t.Request.Params) > 0 {
		if t.Request.ParamsAs == "body" || t.Request.ParamsAs == "multipart" {
			if t.Request.Method == http.MethodGet || t.Request.Method == http.MethodHead {
				err = fmt.Errorf("%s does not allow body or multipart parameters", t.Request.Method)
				return "", err
			}
			if t.Request.Body != "" {
				err = fmt.Errorf("body used with body/multipart parameters")
				return "", err
			}
		}
		switch t.Request.ParamsAs {
		case "URL", "":
			if strings.Contains(rurl, "?") {
				rurl += "&" + urlValues.Encode()
			} else {
				rurl += "?" + urlValues.Encode()
			}
		case "body":
			contentType = "application/x-www-form-urlencoded"
			encoded := urlValues.Encode()
			t.Request.SentBody = encoded
		case "multipart":
			b, boundary, err := multipartBody(t.Request.Params, t.Variables)
			if err != nil {
				return "", err
			}
			bb, err := ioutil.ReadAll(b)
			if err != nil {
				return "", err
			}
			t.Request.SentBody = string(bb)
			contentType = "multipart/form-data; boundary=" + boundary
		default:
			err = fmt.Errorf("unknown parameter method %q", t.Request.ParamsAs)
			return "", err
		}
	}

	// The body.
	if t.Request.Body != "" {
		bodydata, _, err := FileData(t.Request.Body, t.Variables)
		if err != nil {
			return "", err
		}
		t.Request.SentBody = bodydata
	}

	// body := ioutil.NopCloser(strings.NewReader(t.Request.SentBody))
	t.Request.Request, err = http.NewRequest(t.Request.Method, rurl, nil /*body*/)
	if err != nil {
		return "", err
	}

	// Content-Length
	if cl := len(t.Request.SentBody); cl > 0 && !t.Request.Chunked {
		t.Request.Request.ContentLength = int64(cl)
	}

	return contentType, nil
}

// FileData allows to reading file data to be used as the value for s.
// Handled cases of s are:
//    @file:/path/to/thefile
//               read in /path/to/thefile and use its content as s
//               basename is thefile
//    @vfile:/path/to/thefile
//               read in /path/to/thefile and apply repl on its content
//               basename is thefile
//    @[v]file:@name-of-file:direct-data
//               use direct-data as s (variable substitutions not performed again)
//               basename is name-of-file
//    anything-else
//               s is anything-else and basename is ""
func FileData(s string, variables map[string]string) (data string, basename string, err error) {
	if !strings.HasPrefix(s, "@file:") && !strings.HasPrefix(s, "@vfile:") {
		return s, "", nil
	}

	i := strings.Index(s, ":") // != -1 as proper @[v]file: prefix present
	typ, file := s[:i], s[i+1:]
	if len(file) == 0 {
		return "", "", fmt.Errorf("missing filename in @[v]file: parameter")
	}

	// Handle the following syntax:
	//     @file:@filename:direct-file-data
	// which does not read from the filesystem.
	if j := strings.Index(file, ":"); j != -1 && file[0] == '@' {
		basename = file[1:j]
		data = file[j+1:]
		return data, basename, nil
	}

	basename = path.Base(file)
	raw, err := ioutil.ReadFile(file)
	if err != nil {
		return "", "", err
	}
	data = string(raw)
	if typ == "@vfile" {
		data = newReplacer(variables).Replace(data)
	}
	return data, basename, nil
}

var (
	errRedirectNofollow = errors.New("we do not follow redirects")
)

// executeRequest performs the HTTP request defined in t which must have been
// prepared by Prepare. Executing an unprepared Test results will panic.
func (t *Test) executeRequest() error {
	t.infof("%s %q", t.Request.Request.Method, t.Request.Request.URL.String())

	var err error
	abortedRedirection := false
	t.Response.Redirections = nil

	start := time.Now()

	if t.Execution.Verbosity >= 4 {
		buf := &bytes.Buffer{}
		t.Request.Request.Write(buf)
		t.tracef(" Full Request\n%s\n", buf.String())
		// "Rewind body"
		t.Request.Request.Body = ioutil.NopCloser(strings.NewReader(t.Request.SentBody))
	}

	resp, err := t.client.Do(t.Request.Request)
	if ue, ok := err.(*url.Error); ok && ue.Err == errRedirectNofollow &&
		!t.Request.FollowRedirects {
		// Clear err if it is just our redirect non-following policy.
		err = nil
		abortedRedirection = true
		t.debugf("Aborted redirect chain")
	}

	t.Response.Response = resp
	msg := "okay"
	if err == nil {
		if t.Request.Request.Method == "HEAD" || abortedRedirection {
			goto done
		}
		var reader io.ReadCloser
		switch resp.Header.Get("Content-Encoding") {
		case "gzip":
			reader, err = gzip.NewReader(resp.Body)
			if err != nil {
				t.Response.BodyErr = err
				goto done
			}
			t.debugf("Unzipping gzip body")
		default:
			reader = resp.Body
		}
		bb, be := ioutil.ReadAll(reader)
		t.Response.BodyStr = string(bb)
		t.Response.BodyErr = be
		reader.Close()
		if t.Execution.Verbosity >= 4 {
			buf := &bytes.Buffer{}
			t.Response.Response.Header.Write(buf)

			t.tracef(" Full Response\n%s %s\n\n%s",
				t.Response.Response.Proto,
				t.Response.Response.Status,
				t.Response.BodyStr)
		}
	} else {
		msg = fmt.Sprintf("fail %s", err.Error())
	}

done:
	t.Response.Duration = time.Since(start)

	for i, via := range t.Response.Redirections {
		t.infof("Redirection %d: %s", i+1, via)
	}

	t.debugf("Request took %s, %s", t.Response.Duration, msg)

	return err
}

// ExecuteChecks applies the checks in t to the HTTP response received during
// executeRequest.
//
// Normally all checks in t.Checks are executed. If the first check in
// t.Checks is a StatusCode check against 200 and it fails, then the rest of
// the tests are skipped.
func (t *Test) ExecuteChecks() {
	done := false
	for i, ck := range t.Checks {
		start := time.Now()
		err := ck.Execute(t)
		t.CheckResults[i].Duration = time.Since(start)
		if el, ok := err.(ErrorList); ok {
			t.CheckResults[i].Error = el
		} else {
			t.CheckResults[i].Error = ErrorList{err}
		}
		if err != nil {
			t.debugf("Check %d %s Fail: %s", i+1, NameOf(ck), err)
			if _, ok := err.(MalformedCheck); ok {
				t.CheckResults[i].Status = Bogus
			} else {
				t.CheckResults[i].Status = Fail
			}
			var errlist ErrorList
			if el, ok := t.Error.(ErrorList); ok {
				errlist = el
			}
			for _, pce := range t.CheckResults[i].Error {
				errlist = append(errlist, fmt.Errorf("Check %s: %s",
					t.CheckResults[i].Name, pce))
			}
			if len(errlist) != 0 {
				t.Error = errlist
			}

			// Abort needles checking if all went wrong.
			if i == 0 { // only first check is checked against StatusCode/200.
				sc, ok := ck.(StatusCode)
				if !ok {
					if psc, pok := ck.(*StatusCode); pok {
						ok = true
						sc = *psc
					}
				}
				if ok && sc.Expect == 200 {
					t.debugf("skipping remaining tests as bad StatusCode %s", t.Response.Response.Status)
					// Clear Status and Error field as these might be
					// populated from a prior try run of the test.
					for j := 1; j < len(t.CheckResults); j++ {
						t.CheckResults[j].Status = Skipped
						t.CheckResults[j].Error = nil
					}
					done = true
				}
			}
		} else {
			t.CheckResults[i].Status = Pass
			t.debugf("Check %d %s: Pass", i+1, NameOf(ck))
		}
		if t.CheckResults[i].Status > t.Status {
			t.Status = t.CheckResults[i].Status
		}
		if done {
			break
		}
	}
}

func (t *Test) errorf(format string, v ...interface{}) {
	if t.Execution.Verbosity >= 0 && t.Log != nil {
		format = "ERROR " + format + " [%q]"
		v = append(v, t.Name)
		t.Log.Printf(format, v...)
	}
}

func (t *Test) infof(format string, v ...interface{}) {
	if t.Execution.Verbosity >= 1 && t.Log != nil {
		format = "INFO  " + format + " [%q]"
		v = append(v, t.Name)
		t.Log.Printf(format, v...)
	}
}

func (t *Test) debugf(format string, v ...interface{}) {
	if t.Execution.Verbosity >= 2 && t.Log != nil {
		format = "DEBUG " + format + " [%q]"
		v = append(v, t.Name)
		t.Log.Printf(format, v...)
	}
}

func (t *Test) tracef(format string, v ...interface{}) {
	if t.Execution.Verbosity >= 3 && t.Log != nil {
		format = "TRACE Begin [%q]" + format + "TRACE End"
		v = append([]interface{}{t.Name}, v...)
		t.Log.Printf(format, v...)
	}
}

// ----------------------------------------------------------------------------
//  Multipart bodies

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

// multipartBody formats the given param as a proper multipart/form-data
// body and returns a reader ready to use as the body as well as the
// multipart boundary to be include in the content type.
func multipartBody(param map[string][]string, variables map[string]string) (io.ReadCloser, string, error) {
	var body = &bytes.Buffer{}

	isFile := func(v string) bool {
		return strings.HasPrefix(v, "@file:") || strings.HasPrefix(v, "@vfile:")
	}

	var mpwriter = multipart.NewWriter(body)

	// All non-file parameters come first.
	for n, v := range param {
		if len(v) > 0 {
			for _, vv := range v {
				if isFile(vv) {
					continue // files go at the end
				}
				if err := mpwriter.WriteField(n, vv); err != nil {
					return nil, "", err
				}
			}
		} else {
			if err := mpwriter.WriteField(n, ""); err != nil {
				return nil, "", err
			}
		}
	}

	// File parameters go to the end.
	for n, v := range param {
		for _, vv := range v {
			if !isFile(vv) {
				continue // already written
			}
			err := addFilePart(mpwriter, n, vv, variables)
			if err != nil {
				return nil, "", err
			}
		}
	}
	mpwriter.Close()

	return ioutil.NopCloser(body), mpwriter.Boundary(), nil
}

// addFilePart to mpwriter where the parameter n has a @file:-value vv.
func addFilePart(mpwriter *multipart.Writer, n, vv string, variables map[string]string) error {
	data, basename, err := FileData(vv, variables)
	if err != nil {
		return err
	}

	// Doing fw, err := mpwriter.CreateFormFile(n, basename) would
	// be much simpler but would fix the content type to
	// application/octet-stream. We can do a bit better.
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			escapeQuotes(n), escapeQuotes(basename)))
	var ct = "application/octet-stream"
	if i := strings.LastIndex(basename, "."); i != -1 {
		ct = mime.TypeByExtension(basename[i:])
		if ct == "" {
			ct = "application/octet-stream"
		}
	}
	h.Set("Content-Type", ct)
	fw, err := mpwriter.CreatePart(h)
	if err != nil {
		return fmt.Errorf("Unable to create part for parameter %q: %s",
			n, err.Error())
	}

	_, err = io.WriteString(fw, data)
	return err
}

// -------------------------------------------------------------------------
//  Methods of Poll and ClientPool

// Skip return whether the test should be skipped.
func (p Execution) Skip() bool {
	return p.Tries < 0
}

func dontFollowRedirects(*http.Request, []*http.Request) error {
	return errRedirectNofollow
}

// newReplacer produces a strings.Replacer which
// given variables with their values. A key of the form "#123" (i.e. hash
// followed by literal decimal integer) is treated as an integer substitution.
// Other keys are treated as string variables which subsitutes "{{k}}" with
// vars[k] for a key k. Maybe just have a look at the code.
func newReplacer(vars map[string]string) *strings.Replacer {
	oldnew := []string{}
	for k, v := range vars {
		// A string substitution.
		oldnew = append(oldnew, "{{"+k+"}}") // TODO: make configurable ??
		oldnew = append(oldnew, v)
	}
	return strings.NewReplacer(oldnew...)
}

// ----------------------------------------------------------------------------
// Generating curl calls

func escapeForBash(s string) string {
	// The easy case: single quotes preserve everything in bash
	// but single quotes may not appear (not even escaped) within
	// single quoted strings so concatenate:
	//     foo'bar  -->  'foo'"'"'bar'
	// This is not the most efficient or readable quotation, but
	// it is easy to implement and should work reliable.
	parts := strings.Split(s, "'")
	for i, p := range parts {
		parts[i] = "'" + p + "'"
	}
	return strings.Join(parts, `"'"`)
}

func nontrivialData(s string) bool {
	for _, r := range s {
		if r < ' ' || r == 127 {
			return true
		}

	}
	return false
}

func stripAtFile(s string) string {
	if strings.HasPrefix(s, "@file:") {
		return strings.Replace(s, "file:", "", 1)
	}
	if strings.HasPrefix(s, "@vfile:") {
		return strings.Replace(s, "vfile:", "", 1)
	}
	return s
}

// CurlCall tries to create a command line (for bash) curl call which produces
// the same HTTP request as t.
func (t *Test) CurlCall() string {
	call := "curl"

	nontrivial := nontrivialData(t.Request.Body)
	if nontrivial {
		// We have a request body which will be hard or impossible
		// to escape to be copy/pasted to a bash command line
		// prompt. Hack:
		//     tmp=$(mktemp)
		//     printf "\x12\x19\x00" > $tmp
		//     curl --data-binary "@$tmp"
		call = "tmp=$(mktemp)\n"
		buf := &bytes.Buffer{}
		p := make([]byte, 4)
		for _, r := range t.Request.SentBody {
			if r >= ' ' && r <= '~' &&
				!(r == '"' || r == '\'' || r == '\\') {
				buf.WriteRune(r)
			} else {
				buf.WriteString(`\x`)
				n := utf8.EncodeRune(p, r)
				for _, b := range p[:n] {
					fmt.Fprintf(buf, "%02x", b)
				}
			}
		}
		call += `printf "` + buf.String() + `" > $tmp`
		call += "\ncurl"
	}

	// We need the parsed URL which may be unavailable.
	var reqURL *url.URL
	if t.Request.Request != nil {
		reqURL = t.Request.Request.URL
	} else {
		var err error
		reqURL, err = url.Parse(t.Request.URL)
		if err != nil {
			// Fake one.
			reqURL = &url.URL{
				Scheme: "http",
				Host:   "this.should",
				Path:   "not/happen",
			}
		}
	}

	// Method
	if t.Request.Method != "" {
		call += fmt.Sprintf(" -X %s", t.Request.Method)
	}

	// HTTP header
	for header, vals := range t.Request.Header {
		ch := http.CanonicalHeaderKey(header)
		if ch == "Cookie" {
			continue // Cookies are handled below.
		}
		for _, v := range vals {
			if v == "" {
				call += fmt.Sprintf(" -H %s;", ch)
			} else {
				line := fmt.Sprintf("%s: %s", ch, v)
				call += fmt.Sprintf(" -H %s", escapeForBash(line))
			}
		}
	}

	// BasicAuth
	if t.Request.BasicAuthUser != "" {
		arg := fmt.Sprintf("%s:%s", t.Request.BasicAuthUser, t.Request.BasicAuthPass)
		call += fmt.Sprintf(" -u %s", escapeForBash(arg))

	}

	// Cookies
	nvp := []string{}
	for _, cookie := range t.Request.Cookies {
		nvp = append(nvp, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
	}
	for _, cookie := range t.Jar.Cookies(reqURL) {
		nvp = append(nvp, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
	}
	if len(nvp) > 0 {
		line := fmt.Sprintf("Cookie: %s", strings.Join(nvp, "; "))
		call += fmt.Sprintf(" -H %s", escapeForBash(line))
	}

	// Parameters and URL
	theURL := t.Request.URL
	pType := "-d"
	switch t.Request.ParamsAs {
	case "multipart":
		pType = "-F"
		fallthrough
	case "body":
		for name, params := range t.Request.Params {
			for _, p := range params {
				// BUG: @vfile will link to the unreplace file.
				arg := fmt.Sprintf("%s=%s", name, stripAtFile(p))
				call += fmt.Sprintf(" %s %s", pType, escapeForBash(arg))
			}
		}
	case "URL":
		fallthrough
	default:
		theURL = reqURL.String() // contains the parameters
	}

	// The Body
	if t.Request.Body != "" &&
		(t.Request.Method == "POST" || t.Request.Method == "PUT") {
		if nontrivial {
			call += ` --data-binary "@$tmp"`
		} else {
			arg := escapeForBash(stripAtFile(t.Request.Body))
			call += fmt.Sprintf(" --data-binary %s", arg)
		}
	}

	// URL
	call += fmt.Sprintf(" %s", escapeForBash(theURL))

	return call
}
