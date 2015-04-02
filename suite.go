// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"time"

	"log"

	"net"
	"net/http"
	"net/http/cookiejar"

	"strings"
	"sync"
)

// A Suite is a collection of tests which are run together.
type Suite struct {
	Name        string
	Description string

	// Test contains the actual tests to execute.
	Tests []*Test

	// OmitChecks allows to omit all checks defined on the main tests.
	OmitChecks bool

	// Setup contain tests to be executed before the execution actual tests.
	// If one or more setup test fail, the main tets won't be executed.
	// Teardown tests are excuted after the main test.
	// Setup and Teardown share the cookie jar with the main tests.
	Setup, Teardown []*Test

	// Variables contains global variables to be used during this
	// execution
	Variables map[string]string

	// KeepCookies determines whether to use a cookie jar to keep
	// cookies between tests.
	KeepCookies bool

	// Log is the logger to be used by tests and checks.
	Log *log.Logger
}

// Prepare all tests in s for execution.
func (s *Suite) Prepare() error {
	// Create cookie jar if needed.
	cp := &ClientPool{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
	if s.KeepCookies {
		jar, _ := cookiejar.New(nil)
		cp.Jar = jar
	}

	// Try to prepare all tests and inject jar and logger.
	prepare := func(t *Test, which string, omit bool) error {
		t.ClientPool = cp
		err := t.prepare(s.Variables, &TestResult{})
		if err != nil {
			return fmt.Errorf("Suite %q, cannot prepare %s test %q: %s",
				s.Name, which, t.Name, err)
		}
		if omit {
			t.checks = nil
		}
		return nil
	}
	for _, t := range s.Setup {
		if err := prepare(t, "setup", false); err != nil { // Cannot omit checks in setup.
			return err
		}
	}
	for _, t := range s.Tests {
		if err := prepare(t, "main", s.OmitChecks); err != nil {
			return err
		}
	}
	for _, t := range s.Teardown {
		if err := prepare(t, "teardown", s.OmitChecks); err != nil {
			return err
		}
	}
	return nil
}

// Execute the setup tests in s. The tests are executed sequentialy,
// execution stops on the first error.
func (s *Suite) ExecuteSetup() SuiteResult {
	return s.execute(s.Setup, "Setup")
}

// ExecuteTeardown runs all teardown tests ignoring all errors.
func (s *Suite) ExecuteTeardown() SuiteResult {
	return s.execute(s.Teardown, "Teardown")
}

// Execute all non-setup, non-teardown tests of s sequentialy.
func (s *Suite) ExecuteTests() SuiteResult {
	return s.execute(s.Tests, "MainTest")
}

func (s *Suite) execute(tests []*Test, which string) SuiteResult {
	if len(tests) == 0 {
		return SuiteResult{Status: Pass}
	}
	start := time.Now()
	result := SuiteResult{
		Name:        s.Name,
		Description: s.Description,
		Started:     start,
		TestResults: make([]TestResult, len(tests)),
	}
	for i, test := range tests {
		result.TestResults[i] = test.Run(s.Variables)
		result.TestResults[i].SeqNo = fmt.Sprintf("%s-%02d", which, i+1)
	}
	result.FullDuration = time.Since(start)
	result.Status = result.CombineTests()
	return result
}

// Execute the whole suite sequentially.
func (s *Suite) Execute() SuiteResult {
	println("Executing Suite", s.Name)
	result := s.ExecuteSetup()
	if result.Status > Pass {
		n, k, p, f, e, b := result.Stats()
		result.Error = fmt.Errorf("Setup failed: N=%d S=%d P=%d F=%d E=%d B=%d",
			n, k, p, f, e, b)
	} else {
		// Setup worked, run real tests.
		sutr := result.TestResults
		mresult := s.ExecuteTests()
		if mresult.Status != Pass {
			n, k, p, f, e, b := mresult.Stats()
			result.Error = fmt.Errorf("Suite failed: N=%d S=%d P=%d F=%d E=%d B=%d",
				n, k, p, f, e, b)
		}
		// Prepend setup results and update Status
		result.TestResults = append(sutr, mresult.TestResults...)
		result.Status = result.CombineTests()
	}

	// Teardown and append results. Failures/Errors in teardown do not
	// influence the suite status, but bogus teardown test render the
	// whole suite bogus
	tdResult := s.ExecuteTeardown()
	if tdResult.Status == Bogus && result.Status != Bogus {
		result.Status = Bogus
		result.Error = fmt.Errorf("Teardown is bogus.")
	}
	result.TestResults = append(result.TestResults, tdResult.TestResults...)

	return result
}

// ExecuteConcurrent executes all non-setup, non-teardown tests concurrently.
// But at most maxConcurrent tests of s are executed concurrently.
func (s *Suite) ExecuteConcurrent(maxConcurrent int) error {
	if maxConcurrent > len(s.Tests) {
		maxConcurrent = len(s.Tests)
	}
	s.Log.Printf("Running %d test concurrently", maxConcurrent)
	res := make(chan string, len(s.Tests))

	c := make(chan *Test, maxConcurrent)
	wg := sync.WaitGroup{}
	wg.Add(maxConcurrent)
	for i := 0; i < maxConcurrent; i++ {
		go func() {
			defer wg.Done()
			for test := range c {
				result := test.Run(s.Variables) // TODO
				if result.Status != Pass {
					res <- test.Name
				}
			}
		}()
	}
	for _, test := range s.Tests {
		c <- test
	}
	close(c)
	wg.Wait()
	close(res)

	var failures []string
	for ft := range res {
		failures = append(failures, ft)
	}
	if len(failures) > 0 {
		return fmt.Errorf("Failes %d of %d test: %s", len(failures),
			len(s.Tests), strings.Join(failures, ", "))
	}
	return nil
}
