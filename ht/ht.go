// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/vdobler/ht/cookiejar"
	"github.com/vdobler/ht/internal/json5"
)

var (
	// DefaultUserAgent is the user agent string to send in http requests
	// if no user agent header is specified explicitly.
	DefaultUserAgent = "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/36.0.1985.143 Safari/537.36"

	// DefaultAccept is the accept header to be sent if no accept header
	// is set explicitly in the test.
	DefaultAccept = "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8"

	// DefaultClientTimeout is the timeout used by the http clients.
	DefaultClientTimeout = Duration(10 * time.Second)
)

// Transport is the http Transport used while making requests.
// It is exposed to allow different Timeouts or laxer TLS settings.
var Transport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	Dial: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).Dial,
	TLSClientConfig: &tls.Config{
		InsecureSkipVerify: false,
	},
	TLSHandshakeTimeout: 10 * time.Second,
}

// URLValues is a url.Values with a fancier JSON unmarshalling.
type URLValues url.Values

// UnmarshalJSON produces a url.Values (i.e. a map[string][]string) from
// various JSON5 representations. E.g.
//    {
//      a: 12,
//      b: "foo",
//      c: [ 23, "bar"]
//    }
// can be unmarshaled with the expected result.
func (v *URLValues) UnmarshalJSON(data []byte) error {
	vals := make(url.Values)
	raw := map[string]json5.RawMessage{}
	err := json5.Unmarshal(data, &raw)
	if err != nil {
		return err
	}
	for name, r := range raw {
		var generic interface{}
		err := json5.Unmarshal(r, &generic)
		if err != nil {
			return err
		}
		switch g := generic.(type) {
		case float64:
			vals[name] = []string{float64ToString(g)}
		case string:
			vals[name] = []string{g}
		case []interface{}:
			vals[name] = []string{}
			for _, sub := range g {
				switch gg := sub.(type) {
				case float64:
					vals[name] = append(vals[name], float64ToString(gg))
				case string:
					vals[name] = append(vals[name], gg)
				default:
					return fmt.Errorf("ht: illegal url query value %v of type %T for query parameter %s", sub, gg, name)
				}
			}
		default:
			return fmt.Errorf("ht: illegal url query value %v of type %T for query parameter %s", generic, g, name)
		}
	}

	*v = URLValues(vals)
	return nil
}

func float64ToString(f float64) string {
	t := math.Trunc(f)
	if math.Abs(t-f) < 1e-6 {
		return strconv.Itoa(int(t))
	}
	return fmt.Sprintf("%g", f)
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
	Params URLValues `json:",omitempty"`

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
	BasicAuthUser string
	BasicAuthPass string

	// Timeout of this request. If zero use DefaultClientTimeout.
	Timeout Duration `json:",omitempty"`

	Request  *http.Request `json:"-"` // the 'real' request
	SentBody string        `json:"-"` // the 'real' body
}

// Response captures information about a http response.
type Response struct {
	// Response is the received HTTP response. Its body has bean read and
	// closed already.
	Response *http.Response `json:",omitempty"`

	// Duration to receive response and read the whole body.
	Duration Duration

	// The received body and the error got while reading it.
	BodyStr string
	BodyErr error

	// Redirections records the URLs of automatic GET requests due to redirects.
	Redirections []string
}

// Body returns a reader of the response body.
func (resp *Response) Body() *bytes.Reader {
	return bytes.NewReader([]byte(resp.BodyStr)) // TODO
}

// Cookie is a HTTP cookie.
type Cookie struct {
	Name  string
	Value string `json:",omitempty"`
}

// Poll determines if and how to redo a test after a failure or if the
// test should be skipped altogether. The zero value of Poll means "Just do
// the test once."
type Poll struct {
	// Maximum number of redos. Both 0 and 1 mean: "Just one try. No redo."
	// Negative values indicate that the test should be skipped.
	Max int `json:",omitempty"`

	// Duration to sleep between redos.
	Sleep Duration `json:",omitempty"`
}

// ----------------------------------------------------------------------------
// Test

// Test is a single logical test which does one HTTP request and checks
// a number of Checks on the received Response.
type Test struct {
	Name        string
	Description string `json:",omitempty"`

	// Request is the HTTP request.
	Request Request

	// Checks contains all checks to perform on the response to the HTTP request.
	Checks CheckList

	// VarEx may be used to popultate variables from the response. TODO: Rename.
	VarEx map[string]Extractor `json:",omitempty"`

	Poll      Poll `json:",omitempty"`
	Verbosity int  `json:",omitempty"` // Verbosity level in logging.

	// Pre-, Inter- and PostSleep are the sleep durations made
	// before the request, between request and the checks and
	// after the checks.
	PreSleep, InterSleep, PostSleep Duration `json:",omitempty"`

	// Jar is the cookie jar to use
	Jar *cookiejar.Jar `json:"-"`

	// Variables contains variables attached to the Test itself. Variables
	// provided during Run will overwrite variables in TestVars.
	Variables map[string]string `json:",omitempty"`

	// The following results are filled during Run.
	// This should be collected into something like struct TestResult{...}.
	Response     Response      `json:",omitempty"`
	Status       Status        `json:"-"`
	Started      time.Time     `json:"-"`
	Error        error         `json:"-"`
	Duration     Duration      `json:"-"`
	FullDuration Duration      `json:"-"`
	Tries        int           `json:"-"`
	CheckResults []CheckResult `json:"-"` // The individual checks.
	SeqNo        string        `json:"-"`
	VarValues    map[string]string
	ExValues     map[string]Extraction

	client      *http.Client
	specialVars []string
	checks      []Check // prepared checks.
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

	m.Request.Params = make(URLValues)
	m.Request.Header = make(http.Header)
	m.VarEx = make(map[string]Extractor)
	for _, t := range tests {
		err := mergeRequest(&m.Request, t.Request)
		if err != nil {
			return &m, err
		}
		m.Checks = append(m.Checks, t.Checks...)
		if t.Poll.Max > m.Poll.Max {
			m.Poll.Max = t.Poll.Max
		}
		if t.Poll.Sleep > m.Poll.Sleep {
			m.Poll.Sleep = t.Poll.Sleep
		}
		if t.Request.Timeout > m.Request.Timeout {
			m.Request.Timeout = t.Request.Timeout
		}
		if t.Verbosity > m.Verbosity {
			m.Verbosity = t.Verbosity
		}
		m.PreSleep += t.PreSleep
		m.InterSleep += t.InterSleep
		m.PostSleep += t.PostSleep
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

// Run runs the test t. The actual HTTP request is crafted and executed and
// the checks are performed on the received response. This whole process
// is repeated on failure or skipped entirely according to t.Poll.
//
// The given variables are subsituted into the relevant parts of the request
// and the checks.
//
// Normally all checks in t.Checks are executed. If the first check in
// t.Checks is a StatusCode check against 200 and it fails, then the rest of
// the tests are skipped.
//
// Run returns a non-nil error only if the test is bogus; a failing http
// request, problems reading the body or any failing checks do not trigger a
// non-nil return value.
func (t *Test) Run(variables map[string]string) error {
	t.Started = time.Now()

	maxTries := t.Poll.Max
	if maxTries == 0 {
		maxTries = 1
	}
	if maxTries < 0 {
		// This test is deliberately skipped. A zero duration is okay.
		t.Status = Skipped
		return nil
	}

	time.Sleep(time.Duration(t.PreSleep))

	t.CheckResults = make([]CheckResult, len(t.Checks)) // Zero value is NotRun
	for i, c := range t.Checks {
		t.CheckResults[i].Name = NameOf(c)
		buf, err := json5.Marshal(c)
		if err != nil {
			buf = []byte(err.Error())
		}
		t.CheckResults[i].JSON = string(buf)
	}

	// specialVars is the cached version of special variables (NOW, RANDOM)
	// wich can be cached from one try to the next, but this set may change
	// from one run to the other so clear it here.
	t.specialVars = nil

	// Try until first success.
	start := time.Now()
	try := 1
	for ; try <= maxTries; try++ {
		t.Tries = try
		if try > 1 {
			time.Sleep(time.Duration(t.Poll.Sleep))
		}
		err := t.prepare(variables)
		if err != nil {
			t.Status, t.Error = Bogus, err
			return err
		}
		// Clear status and error; is updated in executeChecks.
		t.Status, t.Error = NotRun, nil
		t.execute()
		if t.Status == Pass {
			break
		}
	}
	t.Duration = Duration(time.Since(start))
	if t.Poll.Max > 1 {
		if t.Status == Pass {
			t.debugf("polling succeded after %d tries", try)
		} else {
			t.debugf("polling failed all %d tries", maxTries)
		}
	}

	t.infof("test %s (%s %s)", t.Status, t.Duration, t.Response.Duration)

	time.Sleep(time.Duration(t.PostSleep))

	t.FullDuration = Duration(time.Since(t.Started))
	return nil
}

// AsJSON5 returns a JSON5 representation of the test. Executed tests can
// be serialised and will contain basically all information required to
// debug or re-run the test but note that several fields in the actual
// *http.Request and *http.Response structs are cleared during this
// serialisation.
func (t *Test) AsJSON5() ([]byte, error) {
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

	return json5.MarshalIndent(t, "", "    ")
}

// execute does a single request and check the response.
func (t *Test) execute() {
	var err error
	err = t.executeRequest()
	if err == nil {
		if len(t.Checks) > 0 {
			time.Sleep(time.Duration(t.InterSleep))
			t.executeChecks(t.CheckResults)
		} else {
			t.Status = Pass
		}
	} else {
		t.Status = Error
		t.Error = err
	}
}

// prepare the test for execution by substituting the given variables and
// crafting the underlying http request the checks.
func (t *Test) prepare(variables map[string]string) error {
	// Create appropriate replacer.
	if t.specialVars == nil {
		t.specialVars = t.findSpecialVariables()
	}

	// Make a deep copy of variables.
	allVars := make(map[string]string)
	for n, v := range t.Variables {
		allVars[n] = v
	}
	for n, v := range variables {
		allVars[n] = v
	}

	if len(t.specialVars) > 0 {
		sv, err := specialVariables(time.Now(), t.specialVars)
		if err != nil {
			return err
		}
		// TODO: with mergeVariables(allVars, sv) the values in
		// sv overwrite the ones in allVars. Using it the other
		// way around like (sv, allVars) would allow the user to
		// "overwrite" special variables, e.g. she could fix
		// "{{NOW + 3m}}" to "Mon, 03 Oct 2016 18:00:07 MST"
		// which might come handy.
		allVars = mergeVariables(allVars, sv)
	}
	t.VarValues = allVars
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

	// Prepare the HTTP header.
	for h, v := range t.Request.Header {
		rv := make([]string, len(v))
		for i := range v {
			rv[i] = repl.str.Replace(v[i])
		}
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
		cv := repl.str.Replace(cookie.Value)
		t.Request.Request.AddCookie(&http.Cookie{Name: cookie.Name, Value: cv})
	}
	// Basic Auth
	if t.Request.BasicAuthUser != "" {
		t.Request.Request.SetBasicAuth(
			repl.str.Replace(t.Request.BasicAuthUser),
			repl.str.Replace(t.Request.BasicAuthPass))
	}

	// Compile the checks.
	t.checks = make([]Check, len(t.Checks))
	cfc, cfce := []int{}, []string{}
	for i := range t.Checks {
		t.checks[i] = SubstituteVariables(t.Checks[i], repl.str, repl.fn)
		e := t.checks[i].Prepare()
		if e != nil {
			cfc = append(cfc, i)
			cfce = append(cfce, e.Error())
			t.errorf("preparing check %d %q: %s",
				i, NameOf(t.Checks[i]), e.Error())
		}
	}
	if len(cfc) != 0 {
		err := fmt.Errorf("bogus checks %v: %s", cfc, strings.Join(cfce, "; "))
		return err
	}

	to := DefaultClientTimeout
	if t.Request.Timeout > 0 {
		to = t.Request.Timeout
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
			Jar:           nil,
			Timeout:       time.Duration(to),
		}
	} else {
		t.client = &http.Client{
			Transport:     Transport,
			CheckRedirect: dontFollowRedirects,
			Jar:           nil,
			Timeout:       time.Duration(to),
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
func (t *Test) newRequest(repl replacer) (contentType string, err error) {
	// Set efaults for the request method and the parameter transmission type.
	if t.Request.Method == "" {
		t.Request.Method = "GET"
	}
	if t.Request.ParamsAs == "" {
		t.Request.ParamsAs = "URL"
	}

	// Apply variable substitution.
	rurl := repl.str.Replace(t.Request.URL)
	urlValues := make(URLValues)
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
				rurl += "&" + url.Values(urlValues).Encode()
			} else {
				rurl += "?" + url.Values(urlValues).Encode()
			}
		case "body":
			contentType = "application/x-www-form-urlencoded"
			encoded := url.Values(urlValues).Encode()
			t.Request.SentBody = encoded
			body = ioutil.NopCloser(strings.NewReader(encoded))
		case "multipart":
			b, boundary, err := multipartBody(t.Request.Params, repl)
			if err != nil {
				return "", err
			}
			bb, err := ioutil.ReadAll(b)
			if err != nil {
				return "", err
			}
			t.Request.SentBody = string(bb)
			body = ioutil.NopCloser(bytes.NewReader(bb))
			contentType = "multipart/form-data; boundary=" + boundary
		default:
			err := fmt.Errorf("unknown parameter method %q", t.Request.ParamsAs)
			return "", err
		}
	}

	// The body.
	if t.Request.Body != "" {
		bodydata, _, err := fileData(repl.str.Replace(t.Request.Body), repl)
		if err != nil {
			return "", err
		}
		t.Request.SentBody = bodydata
		body = ioutil.NopCloser(strings.NewReader(bodydata))
	}

	t.Request.Request, err = http.NewRequest(t.Request.Method, rurl, body)
	if err != nil {
		return "", err
	}

	return contentType, nil
}

// fileData allows to reading file data to be used as the value for s.
// Handled cases if s is of the form:
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
func fileData(s string, repl replacer) (data string, basename string, err error) {
	if !strings.HasPrefix(s, "@file:") && !strings.HasPrefix(s, "@vfile:") {
		return s, "", nil
	}

	i := strings.Index(s, ":") // != -1 as proper @[v]file: prefix present
	typ, file := s[:i], s[i+1:]
	if len(file) == 0 {
		return "", "", fmt.Errorf("missing filename in @[v]file: parameter")
	}

	// Handle the follwoing syntax:
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
		data = repl.str.Replace(data)
	}
	return data, basename, nil
}

var (
	redirectNofollow = errors.New("we do not follow redirects")
)

// executeRequest performs the HTTP request defined in t which must have been
// prepared by Prepare. Executing an unprepared Test results will panic.
func (t *Test) executeRequest() error {
	t.infof("%s %q", t.Request.Request.Method, t.Request.Request.URL.String())

	var err error
	abortedRedirection := false
	t.Response.Redirections = nil

	start := time.Now()

	if t.Verbosity >= 4 {
		t.tracef("Full Request follows")
		t.Request.Request.Write(os.Stderr)
		// "Rewind body"
		t.Request.Request.Body = ioutil.NopCloser(strings.NewReader(t.Request.SentBody))
		t.tracef("Full Request end")
	}

	resp, err := t.client.Do(t.Request.Request)
	if ue, ok := err.(*url.Error); ok && ue.Err == redirectNofollow &&
		!t.Request.FollowRedirects {
		// Clear err if it is just our redirect non-following policy.
		err = nil
		abortedRedirection = true
		t.tracef("Aborted redirect chain")
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
			t.tracef("Unzipping gzip body")
		default:
			reader = resp.Body
		}
		bb, be := ioutil.ReadAll(reader)
		t.Response.BodyStr = string(bb)
		t.Response.BodyErr = be
		reader.Close()
		if t.Verbosity >= 4 {
			t.tracef("Full Response follows")
			fmt.Fprintf(os.Stderr, "%s %s\n",
				t.Response.Response.Proto,
				t.Response.Response.Status)
			t.Response.Response.Header.Write(os.Stderr)
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, t.Response.BodyStr)
			t.tracef("Full Response end")
		}
	} else {
		msg = fmt.Sprintf("fail %s", err.Error())
	}

done:
	t.Response.Duration = Duration(time.Since(start))

	for i, via := range t.Response.Redirections {
		t.infof("Redirection %d: %s", i+1, via)
	}

	t.debugf("request took %s, %s", t.Response.Duration, msg)

	return err
}

// executeChecks applies the checks in t to the HTTP response received during
// executeRequest. A non-nil error is returned for bogus checks and checks
// which have errors: Just failing checks do not lead to non-nil-error
//
// Normally all checks in t.Checks are executed. If the first check in
// t.Checks is a StatusCode check against 200 and it fails, then the rest of
// the tests are skipped.
func (t *Test) executeChecks(result []CheckResult) {
	done := false
	for i, ck := range t.checks {
		start := time.Now()
		err := ck.Execute(t)
		result[i].Duration = Duration(time.Since(start))
		if el, ok := err.(ErrorList); ok {
			result[i].Error = el
		} else {
			result[i].Error = ErrorList{err}
		}
		if err != nil {
			t.debugf("check %d %s failed: %s", i, NameOf(ck), err)
			if _, ok := err.(MalformedCheck); ok {
				result[i].Status = Bogus
			} else {
				result[i].Status = Fail
			}
			if t.Error == nil {
				t.Error = err
			}
			// Abort needles checking if all went wrong.
			if i == 0 { // only first check is checked against StatusCode/200.
				sc, ok := ck.(StatusCode)
				psc, pok := ck.(*StatusCode)
				if (ok && sc.Expect == 200) || (pok && psc.Expect == 200) {
					t.tracef("skipping remaining tests")
					// Clear Status and Error field as these might be
					// populated from a prior try run of the test.
					for j := 1; j < len(result); j++ {
						result[j].Status = Skipped
						result[j].Error = nil
					}
					done = true
				}
			}
		} else {
			result[i].Status = Pass
			t.tracef("check %d %s: Pass", i, NameOf(ck))
		}
		if result[i].Status > t.Status {
			t.Status = result[i].Status
		}
		if done {
			break
		}
	}
}

func (t *Test) prepared() bool {
	return t.Request.Request != nil
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

// ----------------------------------------------------------------------------
//  Multipart bodies

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

// multipartBody formats the given param as a proper multipart/form-data
// body and returns a reader ready to use as the body as well as the
// multipart boundary to be include in the content type.
func multipartBody(param map[string][]string, repl replacer) (io.ReadCloser, string, error) {
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
			err := addFilePart(mpwriter, n, vv, repl)
			if err != nil {
				return nil, "", err
			}
		}
	}
	mpwriter.Close()

	return ioutil.NopCloser(body), mpwriter.Boundary(), nil
}

// addFilePart to mpwriter where the parameter n has a @file:-value vv.
func addFilePart(mpwriter *multipart.Writer, n, vv string, repl replacer) error {
	data, basename, err := fileData(vv, repl)
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
	if err != nil {
		return err
	}
	return nil
}

// -------------------------------------------------------------------------
//  Methods of Poll and ClientPool

// Skip return whether the test should be skipped.
func (p Poll) Skip() bool {
	return p.Max < 0
}

func dontFollowRedirects(*http.Request, []*http.Request) error {
	return redirectNofollow
}
