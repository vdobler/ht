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
		ctrl, mocks, err := startMocks(suite, test, rt, &mockResult, testScope)
		if err != nil {
			test.Status = ht.Bogus
			test.Error = err
		}

		// Execute the test (if not bogus).
		exstat := executor(test)

		if ctrl != nil {
			// We got running mocks: Stop mock handling and stop monitoring
			ctrl.stopMocks <- true
			<-ctrl.stopMocks
			close(ctrl.monitor)
			<-ctrl.monitoringDone

			// Analyse what we got and updates test.
			analyseMocks(test, mockResult, mocks)
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

// collect stuff to controll mock execution and result gathering.
type mockCtrl struct {
	stopMocks      chan bool
	monitor        chan *ht.Test
	monitoringDone chan bool
}

func startMocks(suite *Suite, test *ht.Test, rt *RawTest, mockResult *[]*ht.Test, testScope scope.Variables) (*mockCtrl, []*mock.Mock, error) {
	monitor := make(chan *ht.Test)
	mocks := make([]*mock.Mock, 0)

	for i, m := range rt.mocks {
		mockScope := scope.New(testScope, rt.Variables, false)
		mockScope["MOCK_DIR"] = m.Dirname()
		mockScope["MOCK_NAME"] = m.Basename()
		mk, err := m.ToMock(mockScope, true)
		if err != nil {
			return nil, nil,
				fmt.Errorf("mock %d %q is malformed: %s",
					i+1, m.Name, err)

		}
		if mk.Disable {
			continue // Don't start disabled mocks.
		}
		mk.Monitor = monitor
		// Prepend serial number to mock to allow identification.
		mk.Name = fmt.Sprintf("Mock %d: %s", i, mk.Name)
		mocks = append(mocks, mk)
	}
	if len(mocks) == 0 {
		return nil, nil, nil
	}

	// Report any calls that miss explicit mock handlers as 404.
	notFoundHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := ioutil.ReadAll(r.Body)
		u := r.URL.String()
		report := &ht.Test{
			Name:   "Not Found " + u,
			Status: ht.Fail,
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

	stopMocks, err := mock.Serve(mocks, notFoundHandler, suite.Log, "", "")
	if err != nil {
		return nil, nil, err
	}

	monitoringDone := make(chan bool)
	go func() {
		for report := range monitor {
			logMock(suite, report)
			*mockResult = append(*mockResult, report)
		}
		close(monitoringDone)
	}()
	time.Sleep(mockDelay) // I'm clueless...

	ctrl := &mockCtrl{
		stopMocks:      stopMocks,
		monitor:        monitor,
		monitoringDone: monitoringDone}

	return ctrl, mocks, nil
}

// The following cases can happen
//   - Mock executed and okay  --> Pass,  recorde in mockResults
//   - Mock executed and fail  --> Fail,  recorde in mockResults
//   - Mock not executed       --> Error, handled here
//   - Stray call to somewhere --> Fail,  recorde in mockResults via notFoundHandler
func analyseMocks(test *ht.Test, mockResult []*ht.Test, mocks []*mock.Mock) {
	// Collect mockResults into a generated sub-suite and attach as
	// metadata to the test.
	subsuite := &Suite{
		Name:        "Mocks",
		Description: fmt.Sprintf("Mock invocations expected during test %q", test.Name),
		Tests:       mockResult,
	}

	// Step 1: Mocks that actually where invked.
	actual := map[string]bool{} // set of actual invocations
	for _, mt := range mockResult {
		parts := strings.SplitN(mt.Name, ": ", 2) // split "Mock 4" and "Geolocation Mock"
		if len(parts) == 2 && strings.HasPrefix(parts[0], "Mock ") {
			actual[parts[0]] = true
		}
		subsuite.updateStatusAndErr(mt)
	}

	// Step 2: Are there mocks which where not invoked?
	for i, mock := range mocks {
		if actual[fmt.Sprintf("Mock %d", i)] {
			// Fine: mock was called, status propagation happend above.
			continue
		}

		// Add errorred test to subsuite.
		errored := &ht.Test{
			Name: mock.Name,
			Request: ht.Request{
				Method: mock.Method,
				URL:    mock.URL,
			},
			Status: ht.Error,
			Error:  fmt.Errorf("mock %q was not called", mock.Name),
		}
		subsuite.Tests = append(subsuite.Tests, errored)
		subsuite.updateStatusAndErr(errored)

	}
	// Propagete state of mock invocations to main test:
	// Subsuite Fail and Error should render the main test Fail (not Error as
	// Error indicates failure making the initial request).
	// Unclear what to do with Bogus.
	switch subsuite.Status {
	case ht.NotRun, ht.Skipped:
		panic("suite: subsuite status " + subsuite.Status.String())
	case ht.Pass:
		// Fine!
	case ht.Fail, ht.Error:
		if test.Status <= ht.Pass { // actually equal
			test.Status = ht.Fail
			test.Error = fmt.Errorf("Main test pased, but mock invocations failed: %s",
				subsuite.Error)
		}
	case ht.Bogus:
		panic("suite: ooops, should not happen")
	default:
		panic(fmt.Sprintf("suite: unknown subsuite status %d", int(subsuite.Status)))
	}

	// Now glue the subsuite as a metadata to the original Test.
	test.SetMetadata("Subsuite", subsuite)
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

func (suite *Suite) updateStatusAndErr(test *ht.Test) {
	if test.Status > suite.Status {
		suite.Status = test.Status
	}

	if test.Error == nil {
		return
	}
	if suite.Error == nil {
		suite.Error = ht.ErrorList{test.Error}
	} else if el, ok := suite.Error.(ht.ErrorList); ok {
		suite.Error = append(el, test.Error)
	} else {
		suite.Error = ht.ErrorList{suite.Error, test.Error}
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
