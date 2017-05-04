// Package mock provides basic functionality to mock HTTP responses.
//
// Its main use is the following szenario where a ht.Test is used to test
// an endpoint on a server. Handling this endpoint requires one or more
// additional requests to an external backend system which is mocked by
// a Mock.
// Like this the server endpoint can be tested without the need for a working
// backend system and at the same time it is possible to validate the
// request made by the server.
//
//      Suite     Test    Server    Mock
//        |         |     to test     |
//      +---+       |        |        |
//      |   |       |        |      +---+
//      |   +--start backend mock-->|   |
//      |   |       |        |      |   |
//      |   |     +---+      |      |   |
//      |   +---->|   |    +---+    |   |
//      |   |     |   |--->|   |    |   |
//      |   |     |   |    |   |--->|   |
//      |   |     |   |    |   |<---|   |
//      |   |     |   |<---|   |    |   |
//      |   |<----|   |    +---+    |   |
//      |   |     +---+             |   |
//      |   |                       |   |
//      |   |<--report if called----|   |
//      +---+                       +---+
//
// Of course Mocks can be used for general mocking too.
package mock

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/scope"
)

// Log is the interface used for logging.
type Log interface {
	Printf(format string, a ...interface{})
}

// Mock allows to mock a HTTP response for a certain request.
type Mock struct {
	// Name of this mock
	Name string

	// Description of this mock.
	Description string

	// Method for which this mock applies to.
	Method string

	// URL this mock applies to.
	// Schema, Port and Path are considered when deciding if
	// this mock is appropriate for the current request.
	// The path can be a Gorilla mux style path template in which
	// case variables are extracted.
	URL string

	// ParseForm allows to parse query- and form-parameters into variables.
	// If set to true then a request like
	//     curl -d A=1 -d B=2 -d B=3 http://localhost/?C=4
	// would extract the following variable/value-pairs:
	//     A     1
	//     B[0]  2
	//     B[1]  3
	//     C     4
	ParseForm bool

	// VarEx contains variable extraction definitions which are applied
	// to the incomming request.
	VarEx ht.ExtractorMap

	// Checks are applied to to the received HTTP request. This is done
	// by conveting the request to a HTTP response and populating a synthetic
	// ht.Test. This implies that several checks are inappropriate here.
	Checks ht.CheckList

	// Response to send for this mock.
	Response Response

	// Variables contains the default variables/values for this mock.
	Variables scope.Variables

	// Map is used to set variable values depending on other variables.
	// It is executed after VarEx but before constructing the response.
	Map []Mapping

	// Monitor is used to report invocations if this mock.
	// The incomming request and the outgoing mocked response are encoded
	// in a ht.Test. The optional results of the Checks are stored in the
	// Test's CheckResult field.
	// This is nonsensical but is the fastet way to get mocking up running.
	Monitor chan *ht.Test

	// Log to report infos to.
	Log Log

	tls bool // served via https
}

// Response to send as mocked answer.
type Response struct {
	// StatusCode of the response. A value of 0 will result in StatusCode 200.
	StatusCode int

	// Header is the HTTP header to send. If Go's default header is okay it
	// can be empty.
	Header http.Header

	// Body of the response. Body may start with "@file:" and "@vfile:" as
	// explained in detail for ht.FileData.
	Body string
}

// Mapping allows to set the value of a variable based on some other variable's
// value.
// Consider the follwing Mapping:
//      Variables: []string{ "first", "last", "age" },
//      Table: []string{
//          "John", "Smith", "20",
//          "John", "*",     "45",
//          "Paul", "Brown", "30",
//          "*",    "Brown", "55",
//          "*",    "*",     "25",
//     }
// It would set the variable "age" to 30 if first=="Paul" && last=="Brown".
// "John Miller" would be 45 years old and "Sue Carter" 25 because "*" matches
// any value. "John Brown" is 45 because matching happens left to right,
//
type Mapping struct {
	// Variables contains (single or multiple) input variable names and
	// the single output variable name.
	Variables []string

	// Table is the mapping table, its len must be an integer multiple
	// of 3*len(Variables).
	Table []string
}

func (m Mapping) lookup(vars scope.Variables) (string, string) {
	N := len(m.Variables)
	if N < 2 {
		return "", "-malformed-variables-"
	}
	if len(m.Table) == 0 || len(m.Table)%N != 0 {
		return "", "-malformed-table-"
	}
	from, to := m.Variables[:len(m.Variables)-1], m.Variables[len(m.Variables)-1]

	candidate := make([]int, len(m.Table)/N) // line numbers of possible matching candidates
	for i := range candidate {
		candidate[i] = i * N
	}

	// Thin list of candidate table lines
	for i, v := range from {
		x, ok := vars[v]
		if !ok {
			return to, fmt.Sprintf("-undefined-%s-", v)
		}

		remaining := []int{}
		for _, c := range candidate {
			if m.Table[c+i] == x || m.Table[c+i] == "*" {
				// still a candidate
				remaining = append(remaining, c)
			}
		}
		candidate = remaining
	}

	value, bestrank := "-undefined-", int64(-1)
	//   Paul  Brown   --> 3
	//   John  *       --> 2
	//   *     Brown   --> 1
	//   *     *       --> 0
	for _, c := range candidate {
		rank := uint64(0)
		for i := range from {
			if m.Table[c+i] == "*" {
				continue
			}
			rank |= 1 << uint64(i)
		}
		if r := int64(rank); r > bestrank {
			value = m.Table[c+len(from)]
			bestrank = r
		}
	}

	return to, value
}

// ServeHTTP implements http.Handler.
func (m *Mock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.Log != nil {
		m.Log.Printf("Mock %s serving %s", m.Name, r.URL)
	}
	started := time.Now()
	reportStatus := ht.Pass

	// Consume request body and set up a "reversed" fake Test to run
	// Checks against the request and extract variables from the request.
	body, bodyerr := ioutil.ReadAll(r.Body)
	// Restore r.Body for form parsing.
	r.Body = ioutil.NopCloser(bytes.NewReader(body))
	faketest := &ht.Test{
		Name:   "Fake Test for Mock " + m.Name,
		Checks: m.Checks,
		Response: ht.Response{
			Response: &http.Response{
				Status:        "200 OK", // fake
				StatusCode:    200,      // fake
				Header:        r.Header,
				ContentLength: int64(len(body)),
			},
			Duration: 1 * time.Millisecond, // something nonzero
			BodyStr:  string(body),
			BodyErr:  bodyerr,
		},
		VarEx: m.VarEx,
	}
	checkPrepareErr := faketest.PrepareChecks()
	if checkPrepareErr == nil {
		faketest.ExecuteChecks()
		reportStatus = faketest.Status
	}
	extractions := faketest.Extract()

	repl, scope := m.replacer(r, extractions)

	// Body handling: Default variable replacement happens first, then
	// @file and @vfile syntax is handled. This allows to read different
	// files based on the variables extracted from the request.
	preBody := repl.Replace(m.Response.Body)
	sentBody, _, err := ht.FileData(preBody, scope)
	if err != nil {
		http.Error(w,
			fmt.Sprintf("mock: cannot read response body for mock %q: %s",
				m.Name, err),
			http.StatusInternalServerError)
		return
	}

	// Write response to intermediate recorder for reuse in reporting.
	recw := httptest.NewRecorder()
	for key, vals := range m.Response.Header {
		for _, v := range vals {
			recw.Header().Add(key, repl.Replace(v))
		}
	}
	recw.WriteHeader(m.Response.StatusCode)

	// TODO: handle "file:" and "vfile:" body
	io.WriteString(recw, sentBody)
	response := recw.Result()

	// Send actual response.
	for h, vs := range response.Header {
		w.Header()[h] = vs
	}
	status := m.Response.StatusCode
	if status == 0 {
		status = http.StatusOK // 200 is the default
	}
	w.WriteHeader(status)
	io.WriteString(w, sentBody)

	if m.Monitor == nil {
		return
	}

	// Set up a Test used to report the received request and the
	// mocked response. Include the result of the Checks from above.
	report := &ht.Test{
		Name:        m.Name,
		Description: "Autogenerated during mocking a response.",
		Request: ht.Request{
			Method:   r.Method,
			URL:      r.URL.String(),
			Header:   r.Header,
			Request:  r,
			SentBody: string(body),
		},
		Response: ht.Response{
			Response: response,
			Duration: 1 * time.Millisecond, // fake something nonzero
			BodyStr:  sentBody,
			BodyErr:  nil,
		},
		Status:       reportStatus,
		Started:      started,
		Duration:     time.Since(started),
		FullDuration: time.Since(started),
		Tries:        1,
		CheckResults: faketest.CheckResults,
		ExValues:     faketest.ExValues,
	}
	if checkPrepareErr != nil {
		report.Status, report.Error = ht.Bogus, checkPrepareErr
	}

	m.Monitor <- report
}

// Construct a replacer for the response from the mux variables and
// the extractions with extractions overwriting mux variables.
func (m *Mock) replacer(r *http.Request, extractions scope.Variables) (*strings.Replacer, scope.Variables) {
	vars := scope.New(scope.Variables(mux.Vars(r)), m.Variables, false)

	if m.ParseForm {
		// TODO: reformualte to scope.New
		if r.Header.Get("Content-Type") == "multipart/form-data" {
			_ = r.ParseMultipartForm(1 << 24)
		} else {
			_ = r.ParseForm()
		}
		for key, vals := range r.Form {
			if len(vals) == 1 {
				vars[key] = vals[0]
			} else {
				for i, v := range vals {
					vars[fmt.Sprintf("%s[%d]", key, i)] = v
				}
			}
		}
	}

	vars = scope.New(extractions, vars, true)

	// Work through manual variable setting.
	for _, mapping := range m.Map {
		name, val := mapping.lookup(vars)
		vars[name] = val
	}

	return vars.Replacer(), vars
}

// ServerShutdownGraceperiode is the time given the mock servers
// to shut down.
var ServerShutdownGraceperiode = 250 * time.Millisecond

// Serve the given mocks until something is sent on the stop channel.
// once the mock servers have shut down stop is closed.
// The notfound handler is used to catch all request not matching
// a defined route and log can be used for diagnostic messages, both
// may be nil.
// To handle TLS connections you can provide certFile and keyFile as
// described in https://golang.org/pkg/net/http/#Server.ListenAndServeTLS.
// All mocks must use the same certificate/key pair.
func Serve(mocks []*Mock, notfound http.Handler, log Log, certFile, keyFile string) (stop chan bool, err error) {
	stop = make(chan bool)
	group, err := groupMocks(mocks)
	if err != nil {
		return nil, err
	}
	haveTLS := false
	for _, ms := range group {
		if ms[0].tls {
			haveTLS = true
			break
		}
	}
	if haveTLS && (certFile == "" || keyFile == "") {
		return nil, errors.New("mock: need cert and key file to mock https")
	}
	// TODO: handle unreadable cert/keyfile here.

	var servers []*http.Server
	serveErrs := make(chan error)

	// Start servers listeing an all ports with mocks.
	for port, ms := range group {
		tls := ms[0].tls
		if port == "" {
			if tls {
				port = "443"
			} else {
				port = "80"
			}
		}
		srv := createServer(port, ms, notfound, log)
		servers = append(servers, srv)
		if tls {
			go func() {
				err := srv.ListenAndServeTLS(certFile, keyFile)
				serveErrs <- err
			}()
		} else {
			go func() {
				err := srv.ListenAndServe()
				serveErrs <- err
			}()
		}
	}

	// Start goroutine which handels stopping the servers.
	go func() {
		<-stop
		for _, srv := range servers {
			ctx, canc := context.WithTimeout(context.Background(), ServerShutdownGraceperiode)
			srv.Shutdown(ctx)
			canc()
		}
		time.Sleep(5 * time.Millisecond)
		close(stop)
	}()

	// TODO: waiting is bad, better stuff needed.
	select {
	case <-time.After(50 * time.Millisecond):
		// TCP listerners now probably ready.
		// TODO: Thsi should be replaced by our own code. Unfortunately
		// this is some work: a) setup TLS config b) net.Listen on the
		// ports, c) start serving, d) implement own shutdown logic.
		// Especially d) is out of my reach for now.
	case serr := <-serveErrs:
		// At least one server could not start. Shutdown all.
		stop <- true
		<-stop // Wait until all are stopped
		return nil, serr
	}

	return stop, nil
}

// groupMocks groups the mocks by their port number.
func groupMocks(mocks []*Mock) (map[string][]*Mock, error) {
	group := make(map[string][]*Mock)
	for _, m := range mocks {
		u, err := url.Parse(m.URL)
		if err != nil {
			return nil, err
		}
		if u.Scheme == "https" {
			m.tls = true
		}
		port := u.Port()
		group[port] = append(group[port], m)
	}

	// Cannot mix tls and non-tls on one port for now.
	for port, ms := range group {
		for i := 2; i < len(ms); i++ {
			if ms[i].tls == ms[0].tls {
				continue
			}
			return nil, fmt.Errorf(
				"mock: TLS and non-TLS mocks on port %s (e.g. %q and %q)",
				port, ms[0].Name, ms[i].Name)
		}
	}

	return group, nil
}

func createServer(port string, mocks []*Mock, notfound http.Handler, log Log) *http.Server {
	r := mux.NewRouter()
	for _, m := range mocks {
		u, _ := url.Parse(m.URL) // Cannot fail: validated during splitMocks.
		if m.Method == "" {
			m.Method = http.MethodGet
		}
		r.Handle(u.Path, m).Methods(m.Method)
		if log != nil {
			log.Printf("Will handle %s %s", m.Method, m.URL)
		}
	}
	r.NotFoundHandler = notfound
	return &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}
}

// PrintReport produces a multiline report of the request/response pair
// in report, typically generated by monotoring mock servervin.
func PrintReport(report *ht.Test) string {
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "Mock invoked %q: %s %s\n", report.Name,
		report.Request.Method, report.Request.URL)
	fmt.Fprintf(buf, "  Request\n    Header\n")
	for k, v := range report.Request.Request.Header {
		fmt.Fprintf(buf, "      %s: %s\n", k, v)
	}
	fmt.Fprintf(buf, "    Body\n")
	fmt.Fprintf(buf, "      %s\n", report.Request.SentBody)
	fmt.Fprintf(buf, "  Response\n    Header\n")
	for k, v := range report.Response.Response.Header {
		fmt.Fprintf(buf, "      %s: %s\n", k, v)
	}
	fmt.Fprintf(buf, "    Body\n")
	fmt.Fprintf(buf, "      %s\n", report.Response.BodyStr)
	fmt.Fprintf(buf, "========================================================\n")

	return buf.String()
}
