// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*

Loading via (relative) file name

Suites have a dir they live in, this dir is the staring base dir for all
subsequent actions.

    basedir/some.suite       references "a.ht", "../b.ht", and "folder/c.ht"

so loading happens like this:

    a: basedir/a.ht
    b: basedir/../b.ht
    c: basedir/folder/c.ht

each of a, b and c reference a mixin "m.h". This should be loaded as

    a: basedir/m.ht
    b: basedir/../m.ht
    c: basedir/folder/m.ht

if the mixin m.ht itself is based on a mixin "../r.ht" then r.ht should be loaded as

    a: basedir/../r.ht
    b: basedir/../../r.ht
    c: basedir/folder/../r.ht

*/

package ht

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path"
	"strings"

	"github.com/vdobler/ht/internal/json5"
)

// rawTest is the raw representation of a test as read from disk.
type rawTest struct {
	Name        string
	Description string   `json:",omitempty"`
	BasedOn     []string `json:",omitempty"`
	Request     Request
	Checks      CheckList    `json:",omitempty"`
	VarEx       ExtractorMap `json:",omitempty"`

	// Unroll contains values to be used during unrolling the Test
	// generated from the deserialized data to several real Tests.
	Unroll map[string][]string `json:",omitempty"`

	Poll        Poll     `json:",omitempty"`
	Timeout     Duration `json:",omitempty"`
	Verbosity   int      `json:",omitempty"`
	Criticality Criticality

	PreSleep, InterSleep, PostSleep Duration `json:",omitempty"`
}

// rawTestToTest creates a list of real Tests by unrolling a rawTest
// after loading and merging al mixins.
func rawTestToTests(dir string, raw *rawTest, testPool map[string]*rawTest) (tests []*Test, err error) {
	t := &Test{
		Name:        raw.Name,
		Description: raw.Description,
		Request:     raw.Request,
		Checks:      raw.Checks,
		VarEx:       raw.VarEx,
		Poll:        raw.Poll,
		Timeout:     raw.Timeout,
		Verbosity:   raw.Verbosity,
		PreSleep:    raw.PreSleep,
		InterSleep:  raw.InterSleep,
		PostSleep:   raw.PostSleep,
		Criticality: raw.Criticality,
	}
	if len(raw.BasedOn) > 0 {
		origname := t.Name
		base := []*Test{t}
		for _, name := range raw.BasedOn {
			rb, basedir, err := findRawTest(dir, name, testPool, nil)
			if err != nil {
				return nil, err
			}
			rb.Unroll = nil // Mixins are not unroled.
			b, err := rawTestToTests(basedir, rb, testPool)
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

// findRawTest tries to find a test with the given name.
// The name is interpreted as a relative path from curdir. If that file has been
// loaded already it is returned from the pool otherwise it is read from the
// filesystem. The directory the file was found is returned as basedir.
// If the data parameter is non nil it will be used instead of the
// data read from the filesyetem to allow testing.
func findRawTest(curdir string, name string, testPool map[string]*rawTest, data []byte) (raw *rawTest, basedir string, err error) {
	name = path.Join(curdir, name)
	basedir = path.Dir(name)
	if t, ok := testPool[name]; ok {
		return t, basedir, nil
	}

	if data == nil {
		data, err = ioutil.ReadFile(name)
		if err != nil {
			return nil, basedir, err
		}
	}
	raw, err = loadRawTest(data, name)
	if err != nil {
		return nil, basedir, err
	}
	testPool[name] = raw
	return raw, basedir, nil
}

// loadRawTest unmarshals all to a rawTest.
func loadRawTest(all []byte, filename string) (*rawTest, error) {
	t := &rawTest{}
	err := json5.Unmarshal(all, t)
	if err != nil {
		err := fmt.Errorf(beautifyJSONError(err, all, filename))
		return nil, err
	}
	return t, nil
}

// LoadTest reads a test from filename. Tests and mixins are read relative
// to the directory the test lives in. Unrolling is performed.
// TODO
func LoadTest(filename string) ([]*Test, error) {
	dir := path.Dir(filename)
	name := path.Base(filename)
	testPool := make(map[string]*rawTest)
	raw, basedir, err := findRawTest(dir, name, testPool, nil)
	if err != nil {
		return nil, err
	}
	return rawTestToTests(basedir, raw, testPool)
}

// LoadSuite reads a suite from filename. Tests and mixins are read relative
// to the directory the suite lives in.
func LoadSuite(filename string) (*Suite, error) {
	raw, err := loadRawSuite(filename)
	if err != nil {
		return nil, err
	}
	curdir := path.Dir(filename)

	suite := &Suite{
		Name:        raw.Name,
		Description: raw.Description,
		KeepCookies: raw.KeepCookies,
		OmitChecks:  raw.OmitChecks,
	}

	testPool := make(map[string]*rawTest)

	appendTests := func(testNames []string) ([]*Test, error) {
		tests := []*Test{}
		for _, name := range testNames {
			raw, base, err := findRawTest(curdir, name, testPool, nil)
			if err != nil {
				return nil, fmt.Errorf("test %q: %s", name, err)
			}
			ts, err := rawTestToTests(base, raw, testPool)
			if err != nil {
				return nil, fmt.Errorf("test %q: %s", name, err)
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
	if ch < 0 {
		ch = 0
	}
	return fmt.Sprintf("%s:%d: %s\n... %s ...\n    %s^",
		file, lineNo+1, err, lines[lineNo], strings.Repeat(" ", ch))
}
