// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*

Loading via (relative) file name

Suites have a dir they live in, this dir is the starting base dir for all
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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"

	hjson "github.com/hjson/hjson-go"
	"github.com/vdobler/ht/internal/json5"
	"github.com/vdobler/ht/populate"
)

// rawTest is the raw representation of a test as read from disk.
type rawTest struct {
	Name        string
	Description string
	BasedOn     []string
	Request     Request
	Checks      []map[string]interface{}
	Variables   map[string]string
	VarEx       map[string]map[string]interface{}

	Poll      Poll
	Verbosity int

	PreSleep, InterSleep, PostSleep Duration
}

// rawTestToTest creates a real Tests after loading and merging
// all mixins.
func rawTestToTest(dir string, raw *rawTest) (*Test, error) {

	test := &Test{
		Name:        raw.Name,
		Description: raw.Description,
		Request:     raw.Request,
		Variables:   raw.Variables,
		Poll:        raw.Poll,
		// Timeout:     raw.Timeout,
		Verbosity:  raw.Verbosity,
		PreSleep:   raw.PreSleep,
		InterSleep: raw.InterSleep,
		PostSleep:  raw.PostSleep,
	}

	/*
			checks, err := CheckList{}, nil // rawChecksToChecks(raw.Checks)
			if err != nil {
				return nil, err
			}
			test.Checks = checks
			varex, err := nil, nil // rawVarexToVarex(raw.VarEx)
			if err != nil {
				return nil, err
			}
		test.VarEx = varex
	*/

	if len(raw.BasedOn) > 0 {
		var err error
		origname, origfollow := test.Name, test.Request.FollowRedirects
		base := []*Test{test}
		for _, name := range raw.BasedOn {
			rb, basedir, err := findRawTest(dir, name, nil)
			if err != nil {
				return nil, err
			}
			b, err := rawTestToTest(basedir, rb)
			if err != nil {
				return nil, err
			}
			base = append(base, b)
		}
		test, err = Merge(base...)
		if err != nil {
			return nil, err
		}
		// Beautify name and description and force follow redirect
		// policy: BasedOn is not a merge between equal partners.
		test.Description = test.Name + "\n" + test.Description
		test.Name = origname
		test.Request.FollowRedirects = origfollow
	}

	return test, nil
}

// findRawTest tries to find a test with the given name.
// The name is interpreted as a relative path from curdir. If that file has been
// loaded already it is returned from the pool otherwise it is read from the
// filesystem. The directory the file was found is returned as basedir.
// If the data parameter is non nil it will be used instead of the
// data read from the filesystem to allow testing.
func findRawTest(curdir string, name string, data []byte) (raw *rawTest, basedir string, err error) {
	name = path.Join(curdir, name)
	basedir = path.Dir(name)
	// if t, ok := testPool[name]; ok {
	//	return t, basedir, nil
	//}

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

	if raw.Variables == nil {
		raw.Variables = make(map[string]string)
	}
	raw.Variables["TEST_NAME"] = path.Base(name)
	raw.Variables["TEST_DIR"] = path.Dir(name)
	if abspath, err := filepath.Abs(name); err == nil {
		raw.Variables["TEST_PATH"] = filepath.ToSlash(abspath)
	} else {
		raw.Variables["TEST_PATH"] = path.Dir(name) // best effort
	}

	return raw, basedir, nil
}

// loadRawTest unmarshals all to a rawTest.
func loadRawTest(all []byte, filename string) (*rawTest, error) {
	var soup interface{}
	err := hjson.Unmarshal(all, &soup)
	if err != nil {
		return nil, err // TOOD: error message
	}

	rt := &rawTest{}
	err = populate.Strict(rt, soup)
	if err != nil {
		return nil, err // TOOD: error message
	}

	// pp("loadRawTest ", *rt)
	return rt, nil
}

// LoadTest reads a test from filename. Tests and mixins are read relative
// to the directory the test lives in. Unrolling is performed.
//
// The Variables TEST_DIR, TEST_NAME and TEST_PATH are set to the relative
// directory path, the basename and the absolute path of the file the test
// was read.
func LoadTest(filename string) (*Test, error) {
	dir := path.Dir(filename)
	name := path.Base(filename)
	raw, basedir, err := findRawTest(dir, name, nil)
	if err != nil {
		return nil, err
	}
	return rawTestToTest(basedir, raw)
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
		Variables:   make(map[string]string),
	}

	createTests := func(elems []rawElem) ([]*Test, error) {
		tests := []*Test{}
		for i, elem := range elems {
			var raw *rawTest
			var base string
			var err error
			if elem.File != "" {
				raw, base, err = findRawTest(curdir, elem.File, nil)
				if err != nil {
					return nil, fmt.Errorf("test %q: %s", elem.File, err)
				}
			} else if elem.Test.Name != "" { // TODO: Bad check for non-empty Test
				raw, base = &elem.Test, curdir
			} else {
				return nil, fmt.Errorf("both empty") // TODO
			}

			fmt.Println()
			fmt.Println(raw.Name)
			fmt.Println("  raw.Variables:", len(raw.Variables))
			for k, v := range raw.Variables {
				fmt.Println("    ", k, "=", v)
			}
			fmt.Println("  call.Variables:", len(elem.Variables))
			for k, v := range elem.Variables {
				fmt.Println("    ", k, "=", v)
			}

			for varname, varval := range elem.Variables {
				raw.Variables[varname] = varval
			}
			fmt.Println("  --> raw.Variables:", len(raw.Variables))

			t, err := rawTestToTest(base, raw)
			if err != nil {
				return nil, fmt.Errorf("test %d: %s", i, err) // TODO
			}
			tests = append(tests, t)
		}
		return tests, nil
	}

	fmt.Println("Setup")
	suite.Setup, err = createTests(raw.Setup)
	if err != nil {
		return suite, err
	}

	fmt.Println("Main")
	suite.Tests, err = createTests(raw.Tests)
	if err != nil {
		return suite, err
	}
	fmt.Println("Teardown")
	suite.Teardown, err = createTests(raw.Teardown)
	if err != nil {
		return suite, err
	}

	for k, v := range raw.Variables {
		suite.Variables[k] = v
	}
	suite.KeepCookies = raw.KeepCookies
	suite.OmitChecks = raw.OmitChecks

	pp("LoadSuite "+filename, *suite)
	return suite, nil
}

type rawElem struct {
	File      string
	Test      rawTest
	Variables map[string]string
}

// rawSuite is the struct used to deserialize a Suite.
type rawSuite struct {
	Name        string
	Description string
	KeepCookies bool
	OmitChecks  bool
	Setup       []rawElem
	Tests       []rawElem
	Teardown    []rawElem
	Verbosity   int
	Variables   map[string]string
}

// loadRawSuite loads the file with the given filename and decodes into
// a rawSuite.
func loadRawSuite(filename string) (rawSuite, error) {
	all, err := ioutil.ReadFile(filename)
	if err != nil {
		return rawSuite{}, err
	}

	var soup interface{}
	err = hjson.Unmarshal(all, &soup)
	if err != nil {
		err = fmt.Errorf("reading suite %s: %s", filename, err)
		return rawSuite{}, err
	}
	rs := rawSuite{}
	err = populate.Strict(&rs, soup)
	if err != nil {
		err = fmt.Errorf("reading suite %s: %s", filename, err)
		return rawSuite{}, err
	}

	// pp("loadRawSuite", rs)
	return rs, nil
}

func pp(msg string, v interface{}) {
	data, _ := json.MarshalIndent(v, "", "    ")
	fmt.Println(msg, string(data))
}

// beautifyJSONError returns a descriptive error message if err is a
// json5.SyntaxError returned while decoding jsonData which came from
// file.  If err is of any other type err.Error() is returned.
func beautifyJSONError(err error, jsonData []byte, file string) string {
	off := 0

	if se, ok := err.(*json5.SyntaxError); ok {
		off = int(se.Offset)
	} else if fe, ok := err.(*json5.UnmarshalUnknownFieldError); ok {
		off = int(fe.Offset)
	} else {
		return err.Error()
	}

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
