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
		test.SetMetadata("Filename", rt.File.Name)
		if err != nil {
			test.Status = ht.Bogus
			test.Error = err
		}
		test.Jar = suite.Jar
		test.Log = suite.Log

		// Mocks requested for this test: We expect each mock to be
		// called exactly once (and this call should pass).
		mocks := make([]*mock.Mock, len(rt.mocks))
		for i, m := range rt.mocks {
			mockScope := scope.New(testScope, rt.Variables, false)
			mockScope["MOCK_DIR"] = m.Dirname()
			mockScope["MOCK_NAME"] = m.Basename()
			mk, err := m.ToMock(mockScope, true)
			if err != nil {
				test.Status = ht.Bogus
				test.Error = err
				break
			}
			mocks[i] = mk
		}

		ctrl, merr := mock.Provide(mocks, suite.Log)
		if merr != nil {
			test.Status = ht.Bogus
			test.Error = merr
		}

		// Execute the test (if not bogus).
		exstat := executor(test)

		if merr == nil {
			analyseMocks(test, ctrl)
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

// The following cases can happen
//   - Mock executed and okay  --> Pass,  recorde in mockResults
//   - Mock executed and fail  --> Fail,  recorde in mockResults
//   - Mock not executed       --> Error, handled here
//   - Stray call to somewhere --> Fail,  recorde in mockResults via notFoundHandler
func analyseMocks(test *ht.Test, ctrl mock.Control) {
	// Collect mockResults into a generated sub-suite and attach as
	// metadata to the test.
	subsuite := &Suite{
		Name:        "Mocks",
		Description: fmt.Sprintf("Mock invocations expected during test %q", test.Name),
		Tests:       mock.Analyse(ctrl),
	}
	for _, t := range subsuite.Tests {
		subsuite.updateStatusAndErr(t)
	}

	// Propagete state of mock invocations to main test:
	// Subsuite Fail and Error should render the main test Fail (not Error as
	// Error indicates failure making the initial request).
	// Unclear what to do with Bogus.
	switch subsuite.Status {
	case ht.NotRun:
		return // Fine, no mocks request, none invoked.
	case ht.Skipped:
		panic("suite: subsuite status " + subsuite.Status.String())
	case ht.Pass:
		// Fine!
	case ht.Fail, ht.Error:
		if test.Status <= ht.Pass { // actually equal
			test.Status = ht.Fail
			test.Error = fmt.Errorf("Main test passed, but mock invocations failed: %s",
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
