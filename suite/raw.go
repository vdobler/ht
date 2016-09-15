// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suite

import (
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"sort"
	"strings"

	hjson "github.com/hjson/hjson-go"
	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/populate"
	"github.com/yosuke-furukawa/json5/encoding/json5"
)

func pp(msg string, v interface{}) {
	data, _ := json5.MarshalIndent(v, "", "    ")
	fmt.Println(msg, string(data))
}

func ppvars(msg string, vars map[string]string) {
	keys := []string{}
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Println(msg)
	for _, k := range keys {
		fmt.Printf("  %s = %q\n", k, vars[k])
	}
}

// ----------------------------------------------------------------------------
// Counter

// GetCounter returns a strictly increasing sequence of int values.
var GetCounter <-chan int

var counter int = 1

func init() {
	ch := make(chan int)
	GetCounter = ch
	go func() {
		for {
			ch <- counter
			counter += 1
		}
	}()
}

// ----------------------------------------------------------------------------
//   File

// File is a textual representation of a hjson data read from disk.
type File struct {
	Data string
	Name string
}

// NewFile read the given file and returns it as a File.
func LoadFile(filename string) (*File, error) {
	filename = filepath.ToSlash(filename)
	filename = path.Clean(filename)
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// Make sure this is decodable HJSON.
	var soup interface{}
	err = hjson.Unmarshal(data, &soup)
	if err != nil {
		// TOOD: better error message here
		return nil, fmt.Errorf("file %s not valid hjson: %s", filename, err)
	}

	return &File{
		Data: string(data),
		Name: filepath.ToSlash(filename),
	}, nil

}

func VarReplacer(vars map[string]string) *strings.Replacer {
	oldnew := []string{}
	for k, v := range vars {
		oldnew = append(oldnew, "{{"+k+"}}") // TODO: make configurable ??
		oldnew = append(oldnew, v)
	}

	return strings.NewReplacer(oldnew...)
}

// Substitute vars in r.
func (f *File) Substitute(vars map[string]string) *File {
	replacer := VarReplacer(vars)
	return &File{
		Data: replacer.Replace(f.Data),
		Name: f.Name,
	}
}

func (r *File) Dirname() string {
	return path.Dir(r.Name)
}

func (r *File) Basename() string {
	return path.Base(r.Name)
}

func (r *File) decode() (map[string]interface{}, error) {
	var soup interface{}
	err := hjson.Unmarshal([]byte(r.Data), &soup)
	if err != nil {
		return nil, fmt.Errorf("file %s is not valid hjson: %s", r.Name, err)
	}
	m, ok := soup.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("file %s is not an object (got %T)", r.Name, soup)
	}
	return m, nil
}

func (r *File) decodeLaxTo(x interface{}) error {
	var soup interface{}
	err := hjson.Unmarshal([]byte(r.Data), &soup)
	if err != nil {
		// TODO: linenr.
		return fmt.Errorf("file %s is not valid hjson: %s", r.Name, err)
	}
	m, ok := soup.(map[string]interface{})
	if !ok {
		return fmt.Errorf("file %s is not an object (got %T)", r.Name, soup)
	}
	err = populate.Lax(x, m)
	if err != nil {
		return err // better error message here
	}

	return nil
}

func (r *File) decodeStrictTo(x interface{}, drop []string) error {
	var soup interface{}
	err := hjson.Unmarshal([]byte(r.Data), &soup)
	if err != nil {
		// TODO: linenr.
		return fmt.Errorf("file %s is not valid hjson: %s", r.Name, err)
	}
	m, ok := soup.(map[string]interface{})
	if !ok {
		return fmt.Errorf("file %s is not an object (got %T)", r.Name, soup)
	}
	for _, d := range drop {
		delete(m, d)
	}
	err = populate.Strict(x, m)
	if err != nil {
		return err // better error message here
	}

	return nil
}

// ----------------------------------------------------------------------------
//   Mixin

type Mixin struct {
	*File
}

func LoadMixin(filename string) (*Mixin, error) {
	file, err := LoadFile(filename)
	if err != nil {
		return nil, err
	}

	return &Mixin{File: file}, nil
}

func MakeMixin(filename string, fs map[string]*File) (*Mixin, error) {
	file, ok := fs[filename]
	if !ok {
		return nil, fmt.Errorf("cannot find mixin %s", filename)
	}

	return &Mixin{File: file}, nil
}

// ----------------------------------------------------------------------------
//   RawTest

// RawTest is a raw test its mixins and its variables.
type RawTest struct {
	*File
	Mixins      []*Mixin
	Variables   map[string]string
	contextVars map[string]string
	disabled    bool
}

func (r *RawTest) String() string {
	return r.File.Name
}

func (r *RawTest) Disable() {
	r.disabled = true
}
func (r *RawTest) Enable() {
	r.disabled = false
}
func (r *RawTest) IsEnabled() bool {
	return !r.disabled
}

// NewRawTest reads workingdir/filename and produces a new RawTest.
func LoadRawTest(filename string) (*RawTest, error) {
	// workingdir = filepath.ToSlash(workingdir)
	// filename = filepath.Join(workingdir, filename)

	raw, err := LoadFile(filename)
	if err != nil {
		return nil, err
	}

	// Unmarshal to find the Mixins and Variables
	x := &struct {
		Mixin     []string
		Variables map[string]string
	}{}
	err = raw.decodeLaxTo(x)
	if err != nil {
		return nil, err // better error message here
	}

	// Load all mixins from disk.
	testdir := raw.Dirname()
	mixins := make([]*Mixin, len(x.Mixin))
	for i, file := range x.Mixin {
		mixpath := path.Join(testdir, file)
		mixin, err := LoadMixin(mixpath)
		if err != nil {
			return nil, fmt.Errorf("cannot read mixin %s in test %s: %s",
				file, filename, err)
		}
		mixins[i] = mixin
	}

	return &RawTest{
		File:      raw,
		Mixins:    mixins,
		Variables: x.Variables,
	}, nil
}

func MakeRawTest(filename string, fs map[string]*File) (*RawTest, error) {
	raw, ok := fs[filename]
	if !ok {
		return nil, fmt.Errorf("cannot find %s", filename)
	}

	// Unmarshal to find the Mixins and Variables
	x := &struct {
		Mixin     []string
		Variables map[string]string
	}{}
	err := raw.decodeLaxTo(x)
	if err != nil {
		return nil, err // better error message here
	}

	// Load all mixins from disk.
	testdir := raw.Dirname()
	mixins := make([]*Mixin, len(x.Mixin))
	for i, file := range x.Mixin {
		mixpath := path.Join(testdir, file)
		mixin, err := MakeMixin(mixpath, fs)
		if err != nil {
			return nil, fmt.Errorf("cannot read mixin %s in test %s: %s",
				file, filename, err)
		}
		mixins[i] = mixin
	}

	return &RawTest{
		File:      raw,
		Mixins:    mixins,
		Variables: x.Variables,
	}, nil
}

func mergeVariables(global, local map[string]string) map[string]string {
	varset := make(map[string]string)

	// Globals can be used in local values.
	replacer := VarReplacer(global)
	for k, v := range local {
		varset[k] = replacer.Replace(v)
	}
	// Add globals (overwriting locals).
	for k, v := range global {
		varset[k] = v
	}

	return varset
}

// ToTest produces a ht.Test from rt.
func (rt *RawTest) ToTest(variables map[string]string) (*ht.Test, error) {
	bogus := &ht.Test{Status: ht.Bogus}
	replacer := VarReplacer(variables)

	// Make substituted a copy of rt with variables substituted.
	// Dropping the Variabels field as this is no longer useful.
	substituted := &RawTest{
		File: &File{
			Data: replacer.Replace(rt.File.Data),
			Name: rt.File.Name,
		},
		Mixins: make([]*Mixin, len(rt.Mixins)),
	}
	for i := range rt.Mixins {
		substituted.Mixins[i] = &Mixin{
			File: &File{
				Data: replacer.Replace(rt.Mixins[i].File.Data),
			},
		}
	}

	test, err := substituted.toTest(variables)
	if err != nil {
		return bogus, fmt.Errorf("cannot produce Test from %s: %s", rt, err)
	}

	mixins := make([]*ht.Test, len(substituted.Mixins))
	for i, rawmix := range substituted.Mixins {
		mix, err := rawmix.toTest()
		if err != nil {
			return bogus, fmt.Errorf("cannot produce mixin from %s: %s",
				rawmix, err)
		}
		mixins[i] = mix
	}

	origname, origdescr, origfollow := test.Name, test.Description, test.Request.FollowRedirects
	all := append([]*ht.Test{test}, mixins...)
	merged, err := ht.Merge(all...)
	if err != nil {
		return bogus, err
	}
	// Beautify name and description and force follow redirect
	// policy: BasedOn is not a merge between equal partners.
	merged.Description = origdescr
	merged.Name = origname
	merged.Request.FollowRedirects = origfollow

	return merged, nil
}

func (m *Mixin) toTest() (*ht.Test, error) {
	rt := &RawTest{
		File: &File{
			Data: m.File.Data,
			Name: m.Name,
		},
	}
	return rt.toTest(nil)
}

func (r *RawTest) toTest(variables map[string]string) (*ht.Test, error) {
	m, err := r.File.decode()
	if err != nil {
		return nil, err
	}

	delete(m, "Mixin")
	// delete(m, "Variables")
	test := &ht.Test{}

	err = populate.Strict(test, m)
	if err != nil {
		return nil, err // better error message here
	}

	test.Variables = make(map[string]string, len(variables))
	for n, v := range variables {
		test.Variables[n] = v
	}

	return test, nil
}

// ----------------------------------------------------------------------------
//   RawSuite

type SuiteElement struct {
	File      string
	Variables map[string]string
}

// RawSuite
type RawSuite struct {
	*File
	Name, Description     string
	Setup, Main, Teardown []SuiteElement
	KeepCookies           bool
	OmitChecks            bool
	Variables             map[string]string

	tests []*RawTest
}

func (rs *RawSuite) RawTests() []*RawTest {
	return rs.tests
}

func (rs *RawSuite) AddRawTests(ts ...*RawTest) {
	rs.tests = append(rs.tests, ts...)
}

func LoadRawSuite(filename string) (*RawSuite, error) {
	raw, err := LoadFile(filename)
	if err != nil {
		return nil, err
	}

	rs := &RawSuite{}
	err = raw.decodeStrictTo(rs, nil)
	if err != nil {
		return nil, err // better error message here
	}
	rs.File = raw // re-set as decodeStritTo clears rs
	dir := rs.File.Dirname()
	load := func(elems []SuiteElement, which string) error {
		for i, elem := range elems {
			if elem.File != "" {
				filename := path.Join(dir, elem.File)
				rt, err := LoadRawTest(filename)
				if err != nil {
					return fmt.Errorf("unable to load %s (%d. %s): %s",
						filename, i+1, which, err)
				}
				rt.contextVars = elem.Variables
				rs.tests = append(rs.tests, rt)
			} else {
				panic("File must not be empty")
			}
		}
		return nil
	}
	err = load(rs.Setup, "Setup")
	if err != nil {
		return nil, err
	}
	err = load(rs.Main, "Main")
	if err != nil {
		return nil, err
	}
	err = load(rs.Teardown, "Teardown")
	if err != nil {
		return nil, err
	}

	return rs, nil
}

func MakeRawSuite(suite *File, fs map[string]*File) (*RawSuite, error) {
	raw := suite
	rs := &RawSuite{}
	err := raw.decodeStrictTo(rs, nil)
	if err != nil {
		return nil, err // better error message here
	}
	rs.File = raw // re-set as decodeStritTo clears rs

	dir := rs.File.Dirname()
	load := func(elems []SuiteElement, which string) error {
		for i, elem := range elems {
			if elem.File != "" {
				filename := path.Join(dir, elem.File)
				rt, err := MakeRawTest(filename, fs)
				if err != nil {
					return fmt.Errorf("unable to load %s (%d. %s): %s",
						filename, i+1, which, err)
				}
				rt.contextVars = elem.Variables
				rs.tests = append(rs.tests, rt)
			} else {
				panic("File must not be empty")
			}
		}
		return nil
	}
	err = load(rs.Setup, "Setup")
	if err != nil {
		return nil, err
	}
	err = load(rs.Main, "Main")
	if err != nil {
		return nil, err
	}
	err = load(rs.Teardown, "Teardown")
	if err != nil {
		return nil, err
	}

	return rs, nil
}

func updateVariables(test *ht.Test, variables map[string]string) {
	if test.Status != ht.Pass {
		return
	}

	for varname, value := range test.Extract() {
		if old, ok := variables[varname]; ok {
			fmt.Printf("Updating variable %q from %q to %q\n",
				varname, old, value)
		} else {
			fmt.Printf("Setting new variable %q to %q\n",
				varname, value)
		}
		variables[varname] = value
	}
}

func updateSuite(test *ht.Test, suite *Suite) {
	if test.Status <= suite.Status {
		return
	}

	suite.Status = test.Status
	if test.Error != nil {
		suite.Error = test.Error
	}
}

func (rs *RawSuite) Validate(variables map[string]string) error {
	fmt.Println("Validation Suite", rs.Name)
	el := ht.ErrorList{}
	for i, rt := range rs.tests {
		fmt.Printf("Validating Test %d %q\n", i, rt)
		_, err := rt.ToTest(variables)
		if err != nil {
			fmt.Printf("Test %q (%s): %s\n",
				rt.Name, rt.File.Name, err)
			el = append(el, err)
		}
	}
	if len(el) > 0 {
		return el
	}

	return nil
}

// ----------------------------------------------------------------------------
//

/*

{
  <theSuite>
}

# <testormixiname>
{
  <test1>
}

*/

func ParseRawSuite(txt string) (*RawSuite, error) {
	parts := strings.Split(txt, "\n#")

	suite := &File{
		Name: "suite",
		Data: parts[0],
	}

	filesystem := make(map[string]*File, len(parts))
	for _, part := range parts[1:] {
		sp := strings.SplitN(part, "\n", 2)
		if len(sp) != 2 {
			panic(part)
		}
		name := strings.TrimSpace(sp[0])
		filesystem[name] = &File{
			Name: name,
			Data: sp[1],
		}
	}

	return MakeRawSuite(suite, filesystem)
}
