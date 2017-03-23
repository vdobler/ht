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
	"strconv"
	"time"

	"github.com/vdobler/ht/cookiejar"
	"github.com/vdobler/ht/ht"
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

	Variables      map[string]string // The initial variable assignment
	FinalVariables map[string]string // The final set of variables.
	Jar            *cookiejar.Jar    // The cookie jar used
	Log            *log.Logger       // The logger used.
	Verbosity      int

	scope            map[string]string
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

func newScope(outer, inner map[string]string, auto bool) map[string]string {
	// 1. Copy of outer scope
	scope := make(map[string]string, len(outer)+len(inner)+2)
	for gn, gv := range outer {
		scope[gn] = gv
	}
	if auto {
		scope["COUNTER"] = strconv.Itoa(<-GetCounter)
		scope["RANDOM"] = strconv.Itoa(100000 + ht.RandomIntn(900000))
	}
	replacer := varReplacer(scope)

	// 2. Merging inner defaults, allow substitutions from outer scope
	for name, val := range inner {
		if _, ok := scope[name]; ok {
			// Variable name exists in outer scope, do not
			// overwrite with suite defaults.
			continue
		}
		scope[name] = replacer.Replace(val)
	}

	return scope
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

	suite.scope = newScope(global, rs.Variables, true)
	suite.scope["SUITE_DIR"] = rs.File.Dirname()
	suite.scope["SUITE_NAME"] = rs.File.Basename()
	replacer := varReplacer(suite.scope)

	suite.Name = replacer.Replace(rs.Name)
	suite.Description = replacer.Replace(rs.Description)

	for n, v := range suite.scope {
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
		callScope := newScope(suite.scope, rt.contextVars, true)
		testScope := newScope(callScope, rt.Variables, false)
		testScope["TEST_DIR"] = rt.File.Dirname()
		testScope["TEST_NAME"] = rt.File.Basename()
		test, err := rt.ToTest(testScope)
		if err != nil {
			test.Status = ht.Bogus
			test.Error = err
		}
		test.Jar = suite.Jar
		test.Log = suite.Log

		exstat := executor(test)
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

	for n, v := range suite.scope {
		suite.FinalVariables[n] = v
	}
}

func (suite *Suite) updateVariables(test *ht.Test) {
	if test.Status != ht.Pass {
		return
	}

	for varname, value := range test.Extract() {
		if suite.Verbosity >= 2 {
			if old, ok := suite.scope[varname]; ok {
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

		suite.scope[varname] = value
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
