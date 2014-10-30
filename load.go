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
	"time"

	"github.com/vdobler/ht/check"

	"strings"
)

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
// (i.e. paths is ignored while loading such tests).
func LoadSuite(filename string, paths []string) (*Suite, error) {
	s, dir, err := loadSuiteJSON(filename, paths)
	if err != nil {
		return nil, err
	}

	suite := &Suite{
		Name:        s.Suite.Name,
		Description: s.Suite.Description,
		KeepCookies: s.Suite.KeepCookies,
		OmitChecks:  s.Suite.OmitChecks,
	}

	appendTests := func(testNames []string) ([]*Test, error) {
		tests := []*Test{}
		for _, name := range testNames {
			st, err := findSerTest(name, s.Tests, dir)
			if err != nil {
				return nil, fmt.Errorf("test %q: %s", name, err)
			}
			t := &Test{
				Name:        st.Name,
				Description: st.Description,
				Request:     st.Request,
				Checks:      st.Checks,
				Poll:        st.Poll,
				Timeout:     st.Timeout,
				Verbosity:   st.Verbosity,
			}
			if s.Suite.Verbosity > 0 {
				t.Verbosity = s.Suite.Verbosity
			}
			if len(st.Unroll) > 0 {
				nreps := lcmOf(st.Unroll)
				unrolled, err := Repeat(t, nreps, st.Unroll)
				if err != nil {
					return nil, err
				}
				tests = append(tests, unrolled...)
			} else {
				tests = append(tests, t)
			}
		}
		return tests, nil
	}

	suite.Setup, err = appendTests(s.Suite.Setup)
	if err != nil {
		return suite, err
	}
	suite.Tests, err = appendTests(s.Suite.Tests)
	if err != nil {
		return suite, err
	}
	suite.Teardown, err = appendTests(s.Suite.Teardown)
	if err != nil {
		return suite, err
	}

	suite.Variables = s.Variables
	suite.KeepCookies = s.Suite.KeepCookies
	suite.OmitChecks = s.Suite.OmitChecks

	return suite, nil
}

// serTest is used for deserialization of text to Test.
type serTest struct {
	Name        string
	Description string `json:",omitempty"`
	Request     Request
	Checks      check.CheckList `json:",omitempty"`

	// Unroll contains values to be used during unrolling the Test
	// generated from the deserialized data to several real Tests.
	Unroll map[string][]string `json:",omitempty"`

	Poll      Poll          `json:",omitempty"`
	Timeout   time.Duration `json:",omitempty"`
	Verbosity int           `json:",omitempty"`
}

// serSuite is the struct used to deserialize a Suite.
type serSuite struct {
	Tests []*serTest `json:",omitempty"`
	Suite struct {
		Name        string
		Description string   `json:",omitempty"`
		KeepCookies bool     `json:",omitempty"`
		OmitChecks  bool     `json:",omitempty"`
		Setup       []string `json:",omitempty"`
		Tests       []string `json:",omitempty"`
		Teardown    []string `json:",omitempty"`
		Verbosity   int      `json:",omitempty"`
	}
	Variables map[string]string `json:",omitempty"`
}

// loadSuiteJSON loads the file with the given filename and decodes into
// a serSuite. It will try to find filename in all paths and will report
// the dir path back in which the suite was found.
func loadSuiteJSON(filename string, paths []string) (s serSuite, dir string, err error) {
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
		return s, dir, err
	}
	err = json.Unmarshal(all, &s)
	if err != nil {
		return s, dir, err
	}
	return s, dir, nil
}

// findSerTest tries to find a test with the given name in the given Suite.
// Non-local tests are tried to be loaded from dir.
func findSerTest(name string, collection []*serTest, dir string) (*serTest, error) {
	if strings.HasPrefix(name, "@") {
		name = strings.TrimSpace(name[1:])
		name = path.Join(dir, name)
		return loadTestJSON(name)
	}

	if i := strings.LastIndex(name, "@"); i != -1 {
		// Load suite only from given dir (possible relative to dir).
		file := strings.TrimSpace(name[i+1:])
		name = strings.TrimSpace(name[:i])
		rs, _, err := loadSuiteJSON(file, []string{dir})
		if err != nil {
			return nil, err
		}
		collection = rs.Tests
	}
	for i := range collection {
		if collection[i].Name == name {
			return collection[i], nil
		}
	}
	return nil, errors.New("not found")
}

// loadJsonTest loads the file with the given filename and decodes into a Test.
func loadTestJSON(filename string) (*serTest, error) {
	all, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	t := &serTest{}
	err = json.Unmarshal(all, t)
	if err != nil {
		return nil, err
	}
	return t, nil
}
