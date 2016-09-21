// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suite

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/vdobler/ht/cookiejar"
	"github.com/vdobler/ht/ht"
)

// A Suite is a collection of Tests after their (attempted) execution.
type Suite struct {
	Name        string
	Description string
	KeepCookies bool

	Status   ht.Status
	Error    error
	Started  time.Time
	Duration time.Duration

	Tests []*ht.Test

	Variables      map[string]string
	FinalVariables map[string]string
	Jar            *cookiejar.Jar
	Log            *log.Logger

	scope map[string]string
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
	scope := make(map[string]string, len(outer)+10)
	for gn, gv := range outer {
		scope[gn] = gv
	}
	if auto {
		scope["COUNTER"] = strconv.Itoa(<-GetCounter)
		scope["RANDOM"] = strconv.Itoa(100000 + rand.Intn(900000))
	}
	replacer := VarReplacer(scope)

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

		Variables:      make(map[string]string),
		FinalVariables: make(map[string]string),
		Jar:            jar,
		Log:            logger,
	}

	suite.scope = newScope(global, rs.Variables, true)
	suite.scope["SUITE_DIR"] = rs.File.Dirname()
	suite.scope["SUITE_NAME"] = rs.File.Basename()
	replacer := VarReplacer(suite.scope)

	suite.Name = replacer.Replace(rs.Name)
	suite.Description = replacer.Replace(rs.Description)

	for n, v := range suite.scope {
		suite.Variables[n] = v
	}

	return suite
}

// Execute the raw suite rs and capture the outcome in a Suite.
func (rs *RawSuite) Execute(global map[string]string, jar *cookiejar.Jar, logger *log.Logger) *Suite {
	setup, main := len(rs.Setup), len(rs.Main)
	i := 0
	executor := func(test *ht.Test) error {
		i++
		if i <= setup {
			test.Reporting.SeqNo = fmt.Sprintf("Setup-%02d", i)
		} else if i <= setup+main {
			test.Reporting.SeqNo = fmt.Sprintf("Main-%02d", i-setup)
		} else {
			test.Reporting.SeqNo = fmt.Sprintf("Teardown-%02d", i-setup-main)
		}

		if test.Status == ht.Bogus || test.Status == ht.Skipped {
			return nil
		}

		if !rs.tests[i-1].IsEnabled() {
			test.Status = ht.Skipped
			return nil
		}

		test.Run()
		if test.Status > ht.Pass && i <= setup {
			return ErrSkipExecution
		}
		return nil
	}

	// Overall Suite status is computetd from Setup and Main tests only.
	suite := rs.Iterate(global, jar, logger, executor)
	status := ht.NotRun
	errors := ht.ErrorList{}
	for i := 0; i < setup+main && i < len(suite.Tests); i++ {
		if ts := suite.Tests[i].Status; ts > status {
			status = ts
		}
		if err := suite.Tests[i].Error; err != nil {
			errors = append(errors, err)
		}
	}

	suite.Status = status
	if len(errors) == 0 {
		suite.Error = nil
	} else {
		suite.Error = errors
	}

	return suite
}

var (
	ErrAbortExecution = errors.New("Abort Execution")
	ErrSkipExecution  = errors.New("Skip further Tests")
)

type Executor func(test *ht.Test) error

func (rs *RawSuite) Iterate(global map[string]string, jar *cookiejar.Jar, logger *log.Logger, executor Executor) *Suite {
	suite := NewFromRaw(rs, global, jar, logger)
	now := time.Now()
	now = now.Add(-time.Duration(now.Nanosecond()))
	suite.Started = now

	if logger == nil {
		logger = log.New(ioutil.Discard, "", 0)
	}

	var exstat error

	for _, rt := range rs.tests {
		logger.Printf("Executing Test %q\n", rt.File.Name)
		callScope := newScope(suite.scope, rt.contextVars, true)
		testScope := newScope(callScope, rt.Variables, false)
		testScope["TEST_DIR"] = rt.File.Dirname()
		testScope["TEST_NAME"] = rt.File.Basename()
		test, err := rt.ToTest(testScope)
		if err != nil {
			test.Status = ht.Bogus
			test.Error = err
		}
		if exstat == ErrSkipExecution {
			test.Status = ht.Skipped
		}

		test.Jar = suite.Jar
		test.Execution.Verbosity = rs.Verbosity
		test.Log = logger

		exstat = executor(test)
		if test.Status == ht.Pass {
			suite.updateVariables(test)
		}

		if test.Status > ht.Pass {
			logger.Printf("%s test %q (%s) ==> %s",
				strings.ToUpper(test.Status.String()), test.Name,
				rt.File.Name, test.Error)
		} else {
			logger.Printf("%s test %q (%s)",
				strings.ToUpper(test.Status.String()), test.Name, rt.File.Name)
		}

		suite.Tests = append(suite.Tests, test)

		if exstat == ErrAbortExecution {
			break
		}
	}
	suite.Duration = time.Since(suite.Started)
	clip := suite.Duration.Nanoseconds() % 1000000
	suite.Duration -= time.Duration(clip)
	for n, v := range suite.scope {
		suite.FinalVariables[n] = v
	}

	return suite
}

func (s *Suite) updateVariables(test *ht.Test) {
	if test.Status != ht.Pass {
		return
	}

	for varname, value := range test.Extract() {
		if old, ok := s.scope[varname]; ok {
			s.Log.Printf("Updating variable %q from %q to %q\n",
				varname, old, value)
		} else {
			s.Log.Printf("Setting new variable %q to %q\n",
				varname, value)
		}
		s.scope[varname] = value
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
func (s *Suite) Stats() (notRun int, skipped int, passed int, failed int, errored int, bogus int) {
	for _, tr := range s.Tests {
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
				tr.Status, s.Name, tr.Name))
		}
	}
	return
}
