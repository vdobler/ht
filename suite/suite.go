// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package suite allows to read tests and collections of tests (suites) from
// disk and execute them in a controlled way or run throughput load test from
// these test/suites.
//
package suite

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/vdobler/ht/cookiejar"
	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/mock"
	"github.com/vdobler/ht/scope"
)

// A Suite is a collection of Tests which can be executed sequentily with the
// result captured.
type Suite struct {
	Name        string // Name of the Suite.
	Description string // Description of what's going on here.
	KeepCookies bool   // KeepCookies in a cookie jar common to all Tests.

	Status   ht.Status     // Status is the overall status of the whole suite.
	Error    error         // Error encountered during execution of the suite.
	Started  time.Time     // Start of the execution.
	Duration time.Duration // Duration of the execution.

	Tests []*ht.Test // The Tests to execute

	Variables      scope.Variables // The initial variable assignment
	FinalVariables scope.Variables // The final set of variables.
	Jar            *cookiejar.Jar  // The cookie jar used

	Verbosity int
	Log       interface {
		Printf(format string, a ...interface{})
	}

	globals          scope.Variables
	tests            []*RawTest
	noneTeardownTest int
}

func shouldRun(t int, rs *RawSuite, s *Suite) bool {
	if !rs.tests[t].IsEnabled() {
		return false
	}

	// Stop execution on errors during setup
	for i := 0; i < len(rs.Setup) && i < len(s.Tests); i++ {
		if s.Tests[i].Status > ht.Pass {
			return false
		}
	}
	return true
}

// NewFromRaw sets up a new Suite from rs, read to be Iterated.
func NewFromRaw(rs *RawSuite, global map[string]string, jar *cookiejar.Jar, logger *log.Logger) *Suite {
	// Create cookie jar if needed.
	if rs.KeepCookies {
		if jar == nil {
			// Make own, private-use jar.
			jar, _ = cookiejar.New(nil)
		}
	} else {
		jar = nil
	}

	if logger == nil {
		logger = log.New(ioutil.Discard, "", 0)
	}

	suite := &Suite{
		KeepCookies: rs.KeepCookies,

		Status: ht.NotRun,
		Error:  nil,

		Tests: make([]*ht.Test, 0, len(rs.tests)),

		Variables:        make(map[string]string),
		FinalVariables:   make(map[string]string),
		Jar:              jar,
		Log:              logger,
		Verbosity:        rs.Verbosity,
		tests:            rs.tests,
		noneTeardownTest: len(rs.Setup) + len(rs.Main),
	}

	suite.globals = scope.New(global, rs.Variables, true)
	suite.globals["SUITE_DIR"] = rs.File.Dirname()
	suite.globals["SUITE_NAME"] = rs.File.Basename()
	replacer := suite.globals.Replacer()

	suite.Name = replacer.Replace(rs.Name)
	suite.Description = replacer.Replace(rs.Description)

	for n, v := range suite.globals {
		suite.Variables[n] = v
	}

	return suite
}

// A Executor is responsible for executing the given test during the
// Iterate'ion of a Suite. It should return nil if execution should continue
// and ErrAbortExecution to stop further iteration.
type Executor func(test *ht.Test) error

var (
	// ErrAbortExecution indicates that suite iteration should stop.
	ErrAbortExecution = errors.New("Abort Execution")
)

var mockDelay = 50 * time.Millisecond

// Iterate the suite through the given executor.
func (suite *Suite) Iterate(executor Executor) {
	now := time.Now()
	now = now.Add(-time.Duration(now.Nanosecond()))
	suite.Started = now

	overall := ht.NotRun
	errors := ht.ErrorList{}

	for _, rt := range suite.tests {
		// suite.Log.Printf("Executing Test %q\n", rt.File.Name)
		callScope := scope.New(suite.globals, rt.contextVars, true)
		testScope := scope.New(callScope, rt.Variables, false)
		testScope["TEST_DIR"] = rt.File.Dirname()
		testScope["TEST_NAME"] = rt.File.Basename()
		test, err := rt.ToTest(testScope)
		if err != nil {
			test.Status = ht.Bogus
			test.Error = err
		}
		test.Jar = suite.Jar
		test.Log = suite.Log

		// Mocks requested for this test: We expect each mock to be
		// called exactly once (and this call should pass).
		var mockResult []*ht.Test
		stopMocks, monitor, monitoringDone, expect, err := startMocks(suite, test, rt, &mockResult, testScope)
		if err != nil {
			test.Status = ht.Bogus
			test.Error = err
		}

		// Execute the test (if not bogus).
		exstat := executor(test)

		if stopMocks != nil {
			// We got running mocks: Stop mock handling and stop monitoring
			stopMocks <- true
			<-stopMocks
			close(monitor)
			<-monitoringDone

			// Analyse what we got and updates test.
			analyseMocks(test, mockResult, expect)
		}
		if test.Status == ht.Pass {
			suite.updateVariables(test)
		}

		suite.Tests = append(suite.Tests, test)
		if test.Status > overall {
			overall = test.Status
		}
		if err := test.Error; err != nil {
			errors = append(errors, err)
		}

		if exstat == ErrAbortExecution {
			break
		}
	}
	suite.Duration = time.Since(suite.Started)
	clip := suite.Duration.Nanoseconds() % 1000000
	suite.Duration -= time.Duration(clip)
	suite.Status = overall
	if len(errors) == 0 {
		suite.Error = nil
	} else {
		suite.Error = errors
	}

	for n, v := range suite.globals {
		suite.FinalVariables[n] = v
	}
}

func startMocks(suite *Suite, test *ht.Test, rt *RawTest, mockResult *[]*ht.Test, testScope scope.Variables) (stopMocks chan bool, monitor chan *ht.Test, monitoringDone chan bool, expect []string, err error) {
	if len(rt.mocks) == 0 {
		return nil, nil, nil, nil, nil
	}

	monitor = make(chan *ht.Test)
	mocks := []*mock.Mock{}
	for i, m := range rt.mocks {
		mockScope := scope.New(testScope, rt.Variables, false)
		mockScope["MOCK_DIR"] = m.Dirname()
		mockScope["MOCK_NAME"] = m.Basename()
		mk, err := m.ToMock(mockScope, true)
		if err != nil {
			return nil, nil, nil, nil,
				fmt.Errorf("mock %d %q is malformed: %s",
					i+1, m.Name, err)

		}
		mk.Monitor = monitor
		mocks = append(mocks, mk)
		expect = append(expect, mk.Name)
	}

	// Report any calls that miss explicit mock handlers as 404.
	notFoundHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := ioutil.ReadAll(r.Body)
		u := r.URL.String()
		report := &ht.Test{
			Name: "Not Found " + u,
			Request: ht.Request{
				Method:   r.Method,
				URL:      u,
				Header:   r.Header,
				Request:  r,
				SentBody: string(body),
			},
		}
		http.Error(w, "No mock for "+u, http.StatusNotFound)
		monitor <- report
	})

	stopMocks, err = mock.Serve(mocks, notFoundHandler, suite.Log)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	monitoringDone = make(chan bool)
	go func() {
		for report := range monitor {
			logMock(suite, report)
			*mockResult = append(*mockResult, report)
		}
		close(monitoringDone)
	}()
	time.Sleep(mockDelay) // I'm clueless...

	return stopMocks, monitor, monitoringDone, expect, nil
}

func analyseMocks(test *ht.Test, mockResult []*ht.Test, expect []string) {
	actual := []string{}
	for _, mt := range mockResult {
		actual = append(actual, mt.Name)
		// Propagete state of mock invocations to main test.
		if mt.Status > test.Status {
			test.Status = mt.Status
			test.Error = mt.Error
		}
		// Augment check name with information about mock.
		for i := range mt.CheckResults {
			name := mt.CheckResults[i].Name
			name = fmt.Sprintf("Request to mock %q: %s", mt.Name, name)
			mt.CheckResults[i].Name = name
		}
		// Append check results of mock to main test checks.
		test.CheckResults = append(test.CheckResults, mt.CheckResults...)
	}

	// Did we get exactly what we expected?
	sort.Strings(expect)
	sort.Strings(actual)
	// Sorry for that.
	want, got := strings.Join(expect, " ※ "), strings.Join(actual, " ※ ")
	if got != want && test.Status == ht.Pass {
		test.Status = ht.Fail
		test.Error = fmt.Errorf("Expected mocks [%s] but actual mock invocations were [%s]",
			want, got)
	}
}

func logMock(suite *Suite, report *ht.Test) {
	if suite.Verbosity <= 0 {
		return
	}
	if suite.Verbosity < 3 {
		suite.Log.Printf("Mock invoked %q: %s %s", report.Name,
			report.Request.Method, report.Request.URL)
	} else {
		suite.Log.Printf("%s", mock.PrintReport(report))
	}
}

func (suite *Suite) updateVariables(test *ht.Test) {
	if test.Status != ht.Pass {
		return
	}

	for varname, value := range test.Extract() {
		if suite.Verbosity >= 2 {
			if old, ok := suite.globals[varname]; ok {
				if value != old {
					suite.Log.Printf("Updating variable %q to %q\n",
						varname, value)
				} else {
					suite.Log.Printf("Keeping  variable %q as %q\n",
						varname, value)
				}
			} else {
				suite.Log.Printf("Setting  variable %q to %q\n",
					varname, value)
			}
		}

		suite.globals[varname] = value
	}
}

func (suite *Suite) updateStatus(test *ht.Test) {
	if test.Status <= suite.Status {
		return
	}

	suite.Status = test.Status
	if test.Error != nil {
		suite.Error = test.Error
	}
}

// Stats counts the test results of s.
func (suite *Suite) Stats() (notRun int, skipped int, passed int, failed int, errored int, bogus int) {
	for _, tr := range suite.Tests {
		switch tr.Status {
		case ht.NotRun:
			notRun++
		case ht.Skipped:
			skipped++
		case ht.Pass:
			passed++
		case ht.Fail:
			failed++
		case ht.Error:
			errored++
		case ht.Bogus:
			bogus++
		default:
			panic(fmt.Sprintf("No such Status %d in suite %q test %q",
				tr.Status, suite.Name, tr.Name))
		}
	}
	return
}
