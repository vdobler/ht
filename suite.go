// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"

	"log"

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

// Compile prepares all tests in s for execution.
func (s *Suite) Compile() error {
	// Create cookie jar if needed.
	var jar *cookiejar.Jar
	if s.KeepCookies {
		jar, _ = cookiejar.New(nil)
	}

	// Compile all tests and inject jar and logger.
	prepare := func(t *Test, which string, omit bool) error {
		err := t.prepare(s.Variables)
		if err != nil {
			return fmt.Errorf("Suite %q, cannot prepare %s %q: %s",
				s.Name, which, t.Name, err)
		}
		if omit {
			t.checks = nil
		}
		if s.KeepCookies {
			t.Jar = jar
		} else {
			t.Jar = nil
		}
		return nil
	}
	for _, t := range s.Setup {
		if err := prepare(t, "setup", false); err != nil { // Cannot omit checks in setup.
			return err
		}
	}
	for _, t := range s.Tests {
		if err := prepare(t, "test", s.OmitChecks); err != nil {
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
func (s *Suite) ExecuteSetup() Result {
	return s.execute(s.Setup)
}

// ExecuteTeardown runs all teardown tests ignoring all errors.
func (s *Suite) ExecuteTeardown() Result {
	return s.execute(s.Teardown)
}

// Execute all non-setup, non-teardown tests of s sequentialy.
func (s *Suite) ExecuteTests() Result {
	return s.execute(s.Tests)
}

func (s *Suite) execute(tests []*Test) Result {
	if len(tests) == 0 {
		return Result{Status: Pass}
	}
	result := Result{Elements: make([]Result, len(tests))}
	for i, test := range tests {
		result.Elements[i] = test.Run(s.Variables)
	}
	result.Status = CombinedStatus(result.Elements)
	return result
}

// Execute the whole suite sequentially.
func (s *Suite) Execute() Result {
	result := s.ExecuteSetup()
	if result.Status != Pass {
		result.Error = fmt.Errorf("Setup failed")
		return result
	}
	result = s.ExecuteTests()
	if result.Status != Pass {
		result.Error = fmt.Errorf("TODO")
		return result
	}
	s.ExecuteTeardown()
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
