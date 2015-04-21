// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"strings"

	"github.com/vdobler/ht/third_party/json5"
)

var (
	// testPool is a pool of all loaded serialized tests
	testPool map[string]*rawTest
)

func init() {
	testPool = make(map[string]*rawTest)
}

// rawTest is the raw representation of a test as read from disk.
type rawTest struct {
	Name        string
	Description string   `json:",omitempty"`
	BasedOn     []string `json:",omitempty"`
	Request     Request
	Checks      CheckList `json:",omitempty"`

	// Unroll contains values to be used during unrolling the Test
	// generated from the deserialized data to several real Tests.
	Unroll map[string][]string `json:",omitempty"`

	Poll      Poll     `json:",omitempty"`
	Timeout   Duration `json:",omitempty"`
	Verbosity int      `json:",omitempty"`
}

func rawTestToTests(raw *rawTest, dir string) (tests []*Test, err error) {
	t := &Test{
		Name:        raw.Name,
		Description: raw.Description,
		Request:     raw.Request,
		Checks:      raw.Checks,
		Poll:        raw.Poll,
		Timeout:     raw.Timeout,
		Verbosity:   raw.Verbosity,
	}
	if len(raw.BasedOn) > 0 {
		origname := t.Name
		base := []*Test{t}
		for _, name := range raw.BasedOn {
			rb, err := findRawTest(name, dir)
			if err != nil {
				return nil, err
			}
			rb.Unroll = nil
			b, err := rawTestToTests(rb, dir)
			if err != nil {
				return nil, err
			}
			base = append(base, b...)
		}
		t, err = Merge(base...)
		// Beautify name and description: BasedOn is not a merge
		// between equal partners.
		t.Description = t.Name + "\n" + t.Description
		t.Name = origname
		if err != nil {
			return nil, err
		}
	}

	if len(raw.Unroll) > 0 {
		nreps := lcmOf(raw.Unroll)
		unrolled, err := Repeat(t, nreps, raw.Unroll)
		if err != nil {
			return nil, err
		}
		tests = append(tests, unrolled...)
	} else {
		tests = append(tests, t)
	}

	return tests, nil
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
			st, err := findRawTest(name, dir)
			if err != nil {
				return nil, fmt.Errorf("test %q: %s", name, err)
			}
			ts, err := rawTestToTests(st, dir)
			if err != nil {
				fmt.Printf("rawTestToTest failed with %s\n for %#v\n",
					err, st)
			}
			tests = append(tests, ts...)
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

// serSuite is the struct used to deserialize a Suite.
type serSuite struct {
	Tests []*rawTest `json:",omitempty"`
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

func beautifyJSONError(err error, jsonData []byte, filename string) string {
	se, ok := err.(*json5.SyntaxError)
	if !ok {
		return err.Error()
	}

	off := int(se.Offset)
	lines := bytes.Split(jsonData, []byte("\n"))
	total := 0
	lineNo := 0
	for total+len(lines[lineNo]) < off {
		total += len(lines[lineNo]) + 1 // +1 for the \n removed in splitting
		lineNo++
	}
	ch := off - total - 1
	return fmt.Sprintf("%s:%d: %s\n... %s ...\n    %s^",
		filename, lineNo+1, err, lines[lineNo], strings.Repeat(" ", ch))
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
	err = json5.Unmarshal(all, &s)
	if err != nil {
		err = fmt.Errorf(beautifyJSONError(err, all, filename))
		return s, dir, err
	}

	for _, t := range s.Tests {
		if _, ok := testPool[t.Name]; ok {
			log.Fatalf("Duplicate Test name %q", t.Name)
		}
		testPool[t.Name] = t
	}
	return s, dir, nil
}

// findRawTest tries to find a test with the given name in the given Suite.
// Non-local tests are tried to be loaded from dir.
func findRawTest(name string, dir string) (*rawTest, error) {
	if strings.HasPrefix(name, "@") {
		name = strings.TrimSpace(name[1:])
		if t, ok := testPool[name]; ok {
			return t, nil
		}
		name = path.Join(dir, name)
		return loadTestJSON(name)
	}

	if i := strings.LastIndex(name, "@"); i != -1 {
		file := strings.TrimSpace(name[i+1:])
		name = strings.TrimSpace(name[:i])
		if t, ok := testPool[name]; ok {
			return t, nil
		}
		// Load suite only from given dir (possible relative to dir).
		_, _, err := loadSuiteJSON(file, []string{dir})
		if err != nil {
			return nil, err
		}
	}

	if t, ok := testPool[name]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("raw test %q not found", name)
}

// loadJsonTest loads the file with the given filename and decodes into
// a rawTest and stores it in the global pool of raw tests.
func loadTestJSON(filename string) (*rawTest, error) {
	all, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	t := &rawTest{}
	err = json5.Unmarshal(all, t)
	if err != nil {
		err := fmt.Errorf(beautifyJSONError(err, all, filename))
		return nil, err
	}
	testPool[t.Name] = t
	return t, nil
}
