// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path"

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

// Init initialises the suite by unrolling repeated test in Tests.
// Setup and Teardown tests are not unrolled.
func (s *Suite) Init() {
	// Unroll repeated test to single instances
	var tests []*Test
	for _, t := range s.Tests {
		if len(t.UnrollWith) == 0 {
			tests = append(tests, t)
		} else {
			nreps := lcmOf(t.UnrollWith)
			unrolled := t.Repeat(nreps, t.UnrollWith)
			// Clear repetitions to prevent re-unrolling during
			// a second Prepare.
			for _, u := range unrolled {
				u.UnrollWith = nil
			}
			tests = append(tests, unrolled...)
		}
	}
	tests = append(s.Setup, tests...)
	tests = append(tests, s.Teardown...)
	s.Tests = tests
}

// Compile prepares all tests in s for execution.
func (s *Suite) Compile() error {
	// Create cookie jar.
	var jar *cookiejar.Jar
	if s.KeepCookies {
		jar, _ = cookiejar.New(nil)
	}

	// Compile all tests and inject jar and logger.
	prepare := func(t *Test, which string, omit bool) error {
		err := t.Compile(s.Variables)
		if err != nil {
			return fmt.Errorf("Suite %q, cannot prepare %s %q: %s",
				s.Name, which, t.Name, err)
		}
		if omit {
			t.checks = nil
		}
		t.Jar = jar
		t.Log = s.Log
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
		result.Elements[i] = test.Run()
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
				result := test.Run() // TODO
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

// jsonSuite is the struct corresponding to the JSON serialisation od a Suite.
type jsonSuite struct {
	Tests []*Test
	Suite struct {
		Name, Description       string
		KeepCookies, OmitChecks bool
		Tests, Setup, Teardow   []string
	}
	Variables map[string]string
}

// loadJsonSuite loads the file with the given filename and decodes into a jsonSuite.
// It will try to find filename in all paths and will report the dir path back in which
// the suite was found.
func loadJsonSuite(filename string, paths []string) (rs jsonSuite, dir string, err error) {
	var all []byte
	for _, p := range paths {
		dir = path.Join(p, filename)
		all, err = ioutil.ReadFile(dir)
		if err == nil {
			break
		}
	}
	dir = path.Dir(dir)
	if err != nil {
		return rs, dir, err
	}
	err = json.Unmarshal(all, &rs)
	if err != nil {
		return rs, dir, err
	}
	return rs, dir, nil
}

// LoadSuite reads a suite from filename while looking for filename in the
// filesystem paths.
//
// References to individual tests are made by name of the test with two
// special ways to load test from their own files or from different suits
// to access "non-local tests":
//     @name-of-file-with-one-test             will load a single test from
//                                             the given file
//     Name of test @ filename-of-other suite  will use test "Name of Test"
//                                             from the suite found in the
//                                             given file
// Bothe filenames are relative to the position of the suite beeing read
// (i.e. pathsis ignored while loading such tests).
func LoadSuite(filename string, paths []string) (*Suite, error) {
	s, dir, err := loadJsonSuite(filename, paths)
	if err != nil {
		return nil, err
	}

	suite := Suite{
		Name:        s.Suite.Name,
		Description: s.Suite.Description,
		KeepCookies: s.Suite.KeepCookies,
		OmitChecks:  s.Suite.OmitChecks,
	}

	for _, name := range s.Suite.Setup {
		tp, err := findTest(name, s.Tests, dir)
		if err != nil {
			return nil, fmt.Errorf("test %q: %s", name, err)
		}
		suite.Setup = append(suite.Setup, tp)
	}
	for _, name := range s.Suite.Tests {
		tp, err := findTest(name, s.Tests, dir)
		if err != nil {
			return nil, fmt.Errorf("test %q: %s", name, err)
		}
		suite.Tests = append(suite.Tests, tp)
	}
	for _, name := range s.Suite.Teardow {
		tp, err := findTest(name, s.Tests, dir)
		if err != nil {
			return nil, fmt.Errorf("test %q: %s", name, err)
		}
		suite.Teardown = append(suite.Teardown, tp)
	}

	suite.Variables = s.Variables
	suite.KeepCookies = s.Suite.KeepCookies
	suite.OmitChecks = s.Suite.OmitChecks

	return &suite, nil
}

// findTest tries to find a test with the given name in the given Suite.
// Non-local tests are tried to be loaded from dir.
func findTest(name string, collection []*Test, dir string) (*Test, error) {
	if strings.HasPrefix(name, "@") {
		name = strings.TrimSpace(name[1:])
		name = path.Join(dir, name)
		println("Local Single Test", name)
		return loadJsonTest(name)
	}

	if i := strings.LastIndex(name, "@"); i != -1 {
		// Load suite only from given dir (possible relative to dir).
		file := strings.TrimSpace(name[i+1:])
		name = strings.TrimSpace(name[:i])
		println("Local References Test", file, name)
		rs, _, err := loadJsonSuite(file, []string{dir})
		if err != nil {
			return nil, err
		}
		collection = rs.Tests
	} else {
		println("Plain Test", name)
	}

	for i := range collection {
		if collection[i].Name == name {
			return collection[i], nil
		}
	}
	return nil, errors.New("not found")
}

// loadJsonTest loads the file with the given filename and decodes into a Test.
func loadJsonTest(filename string) (*Test, error) {
	all, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	t := &Test{}
	err = json.Unmarshal(all, t)
	if err != nil {
		return nil, err
	}
	return t, nil
}
