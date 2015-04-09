// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"time"
)

// A Suite is a collection of tests which are run together. A Suite must be prepared
// before it can be executed or Executes concurrently.
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

	// Populated during execution
	Status   Status
	Error    error
	Started  time.Time
	Duration Duration
}

func (s Suite) AllTests() []*Test {
	return append(append(s.Setup, s.Tests...), s.Teardown...)
}

// Prepare all tests in s for execution. This will also prepare all tests to
// detect bogus tests early.
func (s *Suite) Prepare() error {
	// Create cookie jar if needed.
	cp := &ClientPool{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			DisableCompression:  true,
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

		err := t.prepare(s.Variables)
		if err != nil {
			return fmt.Errorf("Suite %q, cannot prepare %s test %q: %s",
				s.Name, which, t.Name, err)
		}
		if omit {
			t.Checks = nil
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
func (s *Suite) ExecuteSetup() {
	s.execute(s.Setup, "Setup")
}

// ExecuteTeardown runs all teardown tests ignoring all errors.
func (s *Suite) ExecuteTeardown() {
	s.execute(s.Teardown, "Teardown")
}

// Execute all non-setup, non-teardown tests of s sequentialy.
func (s *Suite) ExecuteTests() {
	s.execute(s.Tests, "MainTest")
}

func (s *Suite) execute(tests []*Test, which string) {
	if len(tests) == 0 {
		return
	}
	for i, test := range tests {
		test.SeqNo = fmt.Sprintf("%s-%02d", which, i+1)
		test.Run(s.Variables)
		if test.Status > s.Status {
			s.Status = test.Status
			if test.Error != nil {
				s.Error = test.Error
			}
		}
	}
}

// Execute the whole suite sequentially.
func (s *Suite) Execute() {
	s.Status, s.Error = NotRun, nil
	s.Started = time.Now()
	defer func() {
		s.Duration = Duration(time.Since(s.Started))
	}()

	s.ExecuteSetup()
	if s.Status > Pass {
		n, k, p, f, e, b := s.Stats()
		s.Error = fmt.Errorf("Setup failed: N=%d S=%d P=%d F=%d E=%d B=%d",
			n, k, p, f, e, b)
		return
	}

	s.ExecuteTests()
	if s.Status != Pass {
		n, k, p, f, e, b := s.Stats()
		s.Error = fmt.Errorf("Suite failed: N=%d S=%d P=%d F=%d E=%d B=%d",
			n, k, p, f, e, b)
	}

	// Teardown and append results. Failures/Errors in teardown do not
	// influence the suite status, but bogus teardown test render the
	// whole suite bogus
	status := s.Status
	s.ExecuteTeardown()
	if s.Status == Bogus && status != Bogus {
		s.Error = fmt.Errorf("Teardown is bogus.")
	} else {
		s.Status = status
	}
}

// ExecuteConcurrent executes all non-setup, non-teardown tests concurrently.
// But at most maxConcurrent tests of s are executed concurrently.
func (s *Suite) ExecuteConcurrent(maxConcurrent int) error {
	s.Status = NotRun
	s.Error = nil
	if maxConcurrent > len(s.Tests) {
		maxConcurrent = len(s.Tests)
	}
	if s.Log != nil {
		s.Log.Printf("Running %d test concurrently", maxConcurrent)
	}

	c := make(chan *Test, maxConcurrent)
	wg := sync.WaitGroup{}
	wg.Add(maxConcurrent)
	for i := 0; i < maxConcurrent; i++ {
		go func() {
			defer wg.Done()
			for test := range c {
				test.Run(s.Variables) // TODO
			}
		}()
	}
	for _, test := range s.Tests {
		c <- test
	}
	close(c)
	wg.Wait()

	for _, test := range s.Tests {
		if test.Status > s.Status {
			s.Status = test.Status
		}
	}

	if s.Status > Pass {
		n, k, p, f, e, b := s.Stats()
		s.Error = fmt.Errorf("Suite failed: N=%d S=%d P=%d F=%d E=%d B=%d",
			n, k, p, f, e, b)
		return s.Error
	}
	return nil
}
