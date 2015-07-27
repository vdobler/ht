// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path"
	"strings"

	"github.com/vdobler/ht/third_party/json5"
)

var (
	// testPool is the collection of all loaded raw tests and raw mixins,
	// it maps relative filenames to rawTests.
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
	VarEx       map[string]Extractor

	// Unroll contains values to be used during unrolling the Test
	// generated from the deserialized data to several real Tests.
	Unroll map[string][]string `json:",omitempty"`

	Poll      Poll     `json:",omitempty"`
	Timeout   Duration `json:",omitempty"`
	Verbosity int      `json:",omitempty"`
}

// rawTestToTest creates a list of real Tests by unrolling a rawTest
// after loading and merging al mixins.
func rawTestToTests(raw *rawTest, dir string) (tests []*Test, err error) {
	t := &Test{
		Name:        raw.Name,
		Description: raw.Description,
		Request:     raw.Request,
		Checks:      raw.Checks,
		VarEx:       raw.VarEx,
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
			rb.Unroll = nil // Mixins are not unroled.
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

// findRawTest tries to find a test with the given name: If the name has been loaded
// already it is returned from the pool otherwise it is read from directory.
func findRawTest(name string, directory string) (*rawTest, error) {
	if t, ok := testPool[name]; ok {
		return t, nil
	}
	name = path.Join(directory, name)
	return loadRawTest(name)
}

// loadRawTest loads the file with the given filename and decodes into
// a rawTest and stores it in the global pool of raw tests.
func loadRawTest(filename string) (*rawTest, error) {
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

// LoadSuite reads a suite from filename. Tests and mixins are read relative
// to the directory the suite lives in.
func LoadSuite(filename string) (*Suite, error) {
	raw, err := loadRawSuite(filename)
	if err != nil {
		return nil, err
	}
	dir := path.Dir(filename)

	suite := &Suite{
		Name:        raw.Name,
		Description: raw.Description,
		KeepCookies: raw.KeepCookies,
		OmitChecks:  raw.OmitChecks,
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

	suite.Setup, err = appendTests(raw.Setup)
	if err != nil {
		return suite, err
	}
	suite.Tests, err = appendTests(raw.Tests)
	if err != nil {
		return suite, err
	}
	suite.Teardown, err = appendTests(raw.Teardown)
	if err != nil {
		return suite, err
	}

	suite.Variables = raw.Variables
	suite.KeepCookies = raw.KeepCookies
	suite.OmitChecks = raw.OmitChecks

	return suite, nil
}

// rawSuite is the struct used to deserialize a Suite.
type rawSuite struct {
	Name        string
	Description string            `json:",omitempty"`
	KeepCookies bool              `json:",omitempty"`
	OmitChecks  bool              `json:",omitempty"`
	Setup       []string          `json:",omitempty"`
	Tests       []string          `json:",omitempty"`
	Teardown    []string          `json:",omitempty"`
	Verbosity   int               `json:",omitempty"`
	Variables   map[string]string `json:",omitempty"`
}

// loadRawSuite loads the file with the given filename and decodes into
// a rawSuite.
func loadRawSuite(filename string) (s rawSuite, err error) {
	var all []byte
	all, err = ioutil.ReadFile(filename)
	if err != nil {
		return s, err
	}

	err = json5.Unmarshal(all, &s)
	if err != nil {
		err = fmt.Errorf(beautifyJSONError(err, all, filename))
		return s, err
	}
	return s, nil
}

// beautifyJSONError returns a descriptive error message if err is a
// json5.SyntaxError returned while decoding jsonData which came from
// file.  If err is of any other type err.Error() is returned.
func beautifyJSONError(err error, jsonData []byte, file string) string {
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
		file, lineNo+1, err, lines[lineNo], strings.Repeat(" ", ch))
}
