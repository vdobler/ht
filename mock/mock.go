// Package mock provides basic functionality to mock HTTP responses.
//
// Its main use is the following szenario where a ht.Test is used to test
// an endpoint on a server. Handling this endpoint require an additional
// request to an external backend system which is mocked by a mock.Mock.
// Like this the server endpoint can be tested without the need for a working
// backend system and at the same time it is possible to validate the
// request made by the server.
//
//    Suite     Test    Server    Mock
//      |         |     to test     |
//    +---+       |        |        |
//    |   |       |        |      +---+
//    |   +--start backend mock--->   |
//    |   |       |        |      |   |
//    |   |     +---+      |      |   |
//    |   +---->|   |    +---+    |   |
//    |   |     |   |--->|   |    |   |
//    |   |     |   |    |   |--->|   |
//    |   |     |   |    |   |<---|   |
//    |   |     |   |<---|   |    |   |
//    |   |<----|   |    +---+    |   |
//    |   |     +---+             |   |
//    |   |                       |   |
//    |   |<--report if called----|   |
//    +---+                       +---+
//
//
package mock

import (
	"bytes"
	"context"
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

	// ParseForm allows to parse query and form parameters into variables.
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

	// Monitor is used to report invocations if this mock.
	// The incomming request and the outgoing mocked response are encoded
	// in a ht.Test. The optional results of the Checks are stored in the
	// Test's CheckResult field.
	// This is nonsensical but is the fastet way to get mocking up running.
	Monitor chan *ht.Test

	// Log to report infos to.
	Log Log
}

// Response to send as mocked answer.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       string
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

	// Construct a replacer for the response from the mux variables and
	// the extractions with extractions overwriting mux variables.
	vars := mux.Vars(r)
	if m.ParseForm {
		// Restore r.Body for form parsing.
		r.Body = ioutil.NopCloser(bytes.NewReader(body))
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
	for k, v := range extractions {
		vars[k] = v
	}
	oldnew := make([]string, 0, 2*len(vars))
	for k, v := range mux.Vars(r) {
		oldnew = append(oldnew, "{{"+k+"}}")
		oldnew = append(oldnew, v)
	}
	repl := strings.NewReplacer(oldnew...)

	// Write response to intermediate recorder for reuse in reporting.
	recw := httptest.NewRecorder()
	for key, vals := range m.Response.Header {
		for _, v := range vals {
			recw.Header().Add(key, repl.Replace(v))
		}
	}
	recw.WriteHeader(m.Response.StatusCode)
	// TODO: handle "file:" and "vfile:" body
	sentBody := repl.Replace(m.Response.Body)
	io.WriteString(recw, sentBody)
	response := recw.Result()

	// Send actual response.
	for h, vs := range response.Header {
		w.Header()[h] = vs
	}
	w.WriteHeader(m.Response.StatusCode)
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

func splitMocks(mocks []*Mock) (tls map[string][]*Mock, std map[string][]*Mock, err error) {
	tls, std = make(map[string][]*Mock), make(map[string][]*Mock)
	for _, m := range mocks {
		u, err := url.Parse(m.URL)
		if err != nil {
			return tls, std, err
		}
		// TODO: handle TLS
		port := u.Port()
		std[port] = append(std[port], m)
	}
	return tls, std, err
}

// ServerShutdownGraceperiode is the time given the mock servers
// to shut down.
var ServerShutdownGraceperiode = 250 * time.Millisecond

// Serve the given mocks until something is sent on the stop channel.
// once the mock servers have shut down stop is closed.
func Serve(mocks []*Mock, notfound http.Handler, log Log) (stop chan bool, err error) {
	stop = make(chan bool)
	tls, std, err := splitMocks(mocks)
	if err != nil {
		return stop, err
	}

	var servers []*http.Server

	for port, ms := range std {
		if port == "" {
			port = "80"
		}
		r := mux.NewRouter()
		for _, m := range ms {
			u, _ := url.Parse(m.URL) // Cannot fail: validated during splitMocks.
			if m.Method == "" {
				m.Method = http.MethodGet
			}
			r.Handle(u.Path, m).Methods(m.Method)
			log.Printf("Will handle %s %s", m.Method, m.URL)
		}
		r.NotFoundHandler = notfound
		srv := &http.Server{
			Addr:    ":" + port,
			Handler: r,
		}
		servers = append(servers, srv)
		go srv.ListenAndServe() // TODO: handle error
	}
	go func() {
		<-stop
		for _, srv := range servers {
			ctx, canc := context.WithTimeout(context.Background(), ServerShutdownGraceperiode)
			srv.Shutdown(ctx)
			canc()
		}
		close(stop)
	}()

	if len(tls) != 0 {
		panic("https mocks not implemented jet")
	}

	return stop, nil
}
