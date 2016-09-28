// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suite

import (
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vdobler/ht/cookiejar"
	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/internal/hjson"
	"github.com/vdobler/ht/populate"
)

func pp(msg string, v interface{}) {
	data, err := hjson.Marshal(v)
	fmt.Println(msg, string(data), err)
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

func LoadMixin(filename string, fs FileSystem) (*Mixin, error) {
	file, err := fs.Load(filename)
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
func LoadRawTest(filename string, fs FileSystem) (*RawTest, error) {
	raw, err := fs.Load(filename)
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
		mixin, err := LoadMixin(mixpath, fs)
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
	Verbosity             int

	tests []*RawTest
}

func (rs *RawSuite) RawTests() []*RawTest {
	return rs.tests
}

func (rs *RawSuite) AddRawTests(ts ...*RawTest) {
	rs.tests = append(rs.tests, ts...)
}

func ParseRawSuite(name string, txt string) (*RawSuite, error) {
	fs, err := NewFileSystem(txt)
	if err != nil {
		return nil, err
	}

	return LoadRawSuite(name, fs)
}

func LoadRawSuite(filename string, fs FileSystem) (*RawSuite, error) {
	raw, err := fs.Load(filename)
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
				rt, err := LoadRawTest(filename, fs)
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

// Execute the raw suite rs and capture the outcome in a Suite.
func (rs *RawSuite) Execute(global map[string]string, jar *cookiejar.Jar, logger *log.Logger) *Suite {
	suite := NewFromRaw(rs, global, jar, logger)
	setup, main := len(rs.Setup), len(rs.Main)
	i := 0
	executor := func(test *ht.Test) error {
		i++
		if i <= setup {
			test.Reporting.SeqNo = fmt.Sprintf("Setup-%02d", i)
		} else if i <= setup+main {
			test.Reporting.SeqNo = fmt.Sprintf("Main-%02d", i-setup)
		} else {
			test.Reporting.SeqNo = fmt.Sprintf("Teardown-%02d", i-setup-main)
		}

		if test.Status == ht.Bogus || test.Status == ht.Skipped {
			return nil
		}

		if !rs.tests[i-1].IsEnabled() {
			test.Status = ht.Skipped
			return nil
		}
		// test.Execution.Verbosity = 2
		test.Run()
		if test.Status > ht.Pass && i <= setup {
			return ErrSkipExecution
		}
		return nil
	}

	// Overall Suite status is computetd from Setup and Main tests only.
	suite.Iterate(executor)
	status := ht.NotRun
	errors := ht.ErrorList{}
	for i := 0; i < setup+main && i < len(suite.Tests); i++ {
		if ts := suite.Tests[i].Status; ts > status {
			status = ts
		}
		if err := suite.Tests[i].Error; err != nil {
			errors = append(errors, err)
		}
	}

	suite.Status = status
	if len(errors) == 0 {
		suite.Error = nil
	} else {
		suite.Error = errors
	}

	return suite
}

// ----------------------------------------------------------------------------
// FileSystem

// FileSystem acts like an in-memory filesystem.
// A nil FileSystem accesses the real OS file system.
type FileSystem map[string]*File

func (fs FileSystem) Load(name string) (*File, error) {
	if len(fs) == 0 {
		return LoadFile(name)
	}
	if f, ok := fs[name]; ok {
		return f, nil
	}
	return nil, fmt.Errorf("file %s not found", name)
}

// NewFileSystem parses txt which must be of the form
//
//     # <filename1>
//     <filecontent1>
//
//     # <filename2>
//     <filecontent2>
//
//     ...
//
// into a new FileSystem.
func NewFileSystem(txt string) (FileSystem, error) {
	parts := strings.Split(txt, "\n#")
	filesystem := make(FileSystem, len(parts))

	for i, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) == 0 {
			continue
		}
		sp := strings.SplitN(part, "\n", 2)
		name := strings.TrimSpace(sp[0])

		if len(sp) != 2 || len(name) == 0 {
			return nil, fmt.Errorf("malformed part %d", i+1)
		}
		if _, ok := filesystem[name]; ok {
			return nil, fmt.Errorf("duplicate name %q", name)
		}
		filesystem[name] = &File{
			Name: name,
			Data: sp[1],
		}
	}

	return filesystem, nil
}

// ----------------------------------------------------------------------------
// RawScenrio & RawLoadtest

type RawScenario struct {
	File       string
	Name       string
	Percentage int
	MaxThreads int
	Variables  map[string]string
	OmitChecks bool

	rawSuite *RawSuite
}

type RawLoadTest struct {
	*File
	Name        string
	Description string
	Scenarios   []RawScenario
	Variables   map[string]string
}

func ParseRawLoadtest(name string, txt string) (*RawLoadTest, error) {
	fs, err := NewFileSystem(txt)
	if err != nil {
		return nil, err
	}

	return LoadRawLoadtest(name, fs)
}

func LoadRawLoadtest(filename string, fs FileSystem) (*RawLoadTest, error) {
	raw, err := fs.Load(filename)
	if err != nil {
		return nil, err
	}

	rlt := &RawLoadTest{}
	err = raw.decodeStrictTo(rlt, nil)
	if err != nil {
		return nil, err // better error message here
	}
	rlt.File = raw // re-set as decodeStritTo clears rs
	dir := rlt.File.Dirname()

	for i, s := range rlt.Scenarios {
		if s.File != "" {
			filename := path.Join(dir, s.File)
			rs, err := LoadRawSuite(filename, fs)
			if err != nil {
				return nil, fmt.Errorf("unable to load suite %s (%d. scenraio): %s",
					filename, i+1, err)
			}
			rlt.Scenarios[i].rawSuite = rs
		} else {
			panic("File must not be empty")
		}
	}

	return rlt, nil
}

func (raw *RawLoadTest) ToScenario(globals map[string]string) []Scenario {
	scenarios := []Scenario{}
	ltscope := newScope(globals, raw.Variables, true)
	for _, rs := range raw.Scenarios {
		callscope := newScope(ltscope, rs.Variables, true)
		scen := Scenario{
			Name:       rs.Name,
			RawSuite:   rs.rawSuite,
			Percentage: rs.Percentage,
			MaxThreads: rs.MaxThreads,
			globals:    callscope,
		}

		scenarios = append(scenarios, scen)
	}

	return scenarios
}
