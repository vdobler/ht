// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/vdobler/ht/cookiejar"
	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/sanitize"
	"github.com/vdobler/ht/scope"
	"github.com/vdobler/ht/suite"
)

var cmdExec = &Command{
	RunSuites:   runExecute,
	Usage:       "exec [options] <suite>...",
	Description: "generate request and test response",
	Flag:        flag.NewFlagSet("run", flag.ContinueOnError),
	Help: `
Exec loads the given suites and executes them.
Variables set with the -D flag overwrite variables read from file with -Dfile.
The current variable assignment at the end of a suite carries over to the next
suite. All suites (which keep cookies) share a common jar if cookies are
loaded via -cookie flag; otherwise each suite has its own cookiejar.

As a convenience exec recognises the /... syntax of the go tool to load all
*.ssuite files below dir: 'ht exec dir/...' is just syntactical suggar for
'ht exec $(find dir -type f -name \*.suite | sort)'.

The exit code is 3 if bogus tests or checks are found, 2 if test errors
are present, 1 if only check failures occurred and 0 if everything passed,
nothing was executed or everything was skipped. Note that the status of
Teardown test are ignored while determining the exit code.

A suite and the used tests may be given as an archive file like this:
<entrypoint>@<archivefile>. Here <entrypoint> is the formal suite filename
in the filesytem file <archivefile>. Archivefiles are collection of HJSON
objects as described in the main help (run '$ ht help').
`,
}

var carryVars bool

func init() {
	addOnlyFlag(cmdExec.Flag)
	addSkipFlag(cmdExec.Flag)

	addTestFlags(cmdExec.Flag)
	addOutputFlag(cmdExec.Flag)

	cmdExec.Flag.BoolVar(&carryVars, "carry", false,
		"carry variables from finished suite to next suite")

}

func runExecute(cmd *Command, suites []*suite.RawSuite) {
	if ssilent {
		silent = true
	}
	prepareHT()
	jar := loadCookies()

	prepareOutputDir()
	var errors ht.ErrorList

	outcome, err := executeSuites(suites, variablesFlag, jar)
	errors = errors.Append(err)
	err = reportOverall(outcome)
	errors = errors.Append(err)
	if errors.AsError() != nil {
		fmt.Fprintln(os.Stderr, "Error encountered during execution:")
		for _, msg := range errors.AsStrings() {
			fmt.Fprintln(os.Stderr, msg)
		}
		os.Exit(7)
	}

	switch outcome.status {
	case ht.NotRun, ht.Skipped, ht.Pass:
		os.Exit(0)
	case ht.Fail:
		os.Exit(1)
	case ht.Error:
		os.Exit(2)
	case ht.Bogus:
		os.Exit(3)
	}

}

func prepareOutputDir() {
	if outputDir == "/dev/null" {
		mute = true
	} else if outputDir == "" {
		outputDir = time.Now().Format("2006-01-02_15h04m05s")
	}

	if !mute {
		err := os.MkdirAll(outputDir, 0766)
		if err != nil {
			log.Panic(err)
		}
	}
}

// accumulator of several suites.
type accumulator struct {
	status  ht.Status
	vars    map[string]string
	cookies map[string]cookiejar.Entry

	total, notrun, skip, pass, err, fail, bogus int

	suites []struct {
		Name, Path, Status string
	}
}

func newAccumulator() *accumulator {
	return &accumulator{
		vars:    make(map[string]string),
		cookies: make(map[string]cookiejar.Entry),
	}
}

func (a *accumulator) update(s *suite.Suite) {
	// Reporting
	a.suites = append(a.suites, struct {
		Name, Path, Status string
	}{
		s.Name,
		sanitize.Filename(s.Name),
		strings.ToUpper(s.Status.String()),
	})

	// Status
	if s.Status > a.status {
		a.status = s.Status
	}

	// Variables
	for name, value := range s.FinalVariables {
		a.vars[name] = value
	}

	// Cookies
	if s.Jar != nil {
		for _, tld := range s.Jar.ETLDsPlus1(nil) {
			for _, cookie := range s.Jar.Entries(tld, nil) {
				id := cookie.ID()
				a.cookies[id] = cookie
			}
		}
	}

	for _, t := range s.Tests {
		switch t.Status {
		case ht.NotRun:
			a.notrun++
		case ht.Skipped:
			a.skip++
		case ht.Pass:
			a.pass++
		case ht.Error:
			a.err++
		case ht.Fail:
			a.fail++
		case ht.Bogus:
			a.bogus++
		}
		a.total++
	}
}

func reportOverall(a *accumulator) error {
	var errors ht.ErrorList
	var err error
	// Save consolidated variables if required.
	if vardump != "" && !mute {
		err = saveVariables(a.vars, vardump)
		errors = errors.Append(err)
	}

	// Save consolidated cookies if required.
	if cookiedump != "" && !mute {
		err = saveCookies(a.cookies, cookiedump)
		errors = errors.Append(err)
	}

	if !ssilent {
		fmt.Println()
		fmt.Printf("Total %d,  Passed %d,  Skipped %d,  Errored %d,  Failed %d,  Bogus %d\n",
			a.total, a.pass, a.skip, a.err, a.fail, a.bogus)
		fmt.Println(strings.ToUpper(a.status.String()))
	}

	return errors.AsError()
}

func saveVariables(vars map[string]string, filename string) error {
	if mute {
		return nil
	}
	b, err := json.MarshalIndent(vars, "    ", "")
	if err != nil {
		return nil
	}
	return ioutil.WriteFile(filename, b, 0666)
}

func saveCookiesFromJar(jar *cookiejar.Jar, filename string) error {
	if jar == nil {
		return nil
	}

	cookies := make(map[string]cookiejar.Entry)
	for _, tld := range jar.ETLDsPlus1(nil) {
		for _, cookie := range jar.Entries(tld, nil) {
			id := cookie.ID()
			cookies[id] = cookie
		}
	}
	return saveCookies(cookies, filename)
}

func saveCookies(cookies map[string]cookiejar.Entry, filename string) error {
	if mute {
		return nil
	}
	b, err := json.MarshalIndent(cookies, "    ", "")
	if err != nil {
		return nil
	}
	return ioutil.WriteFile(filename, b, 0666)
}

// TODO: handle errors, preparse template, cleanup mess, learn to program
func saveOverallReport(dirname string, accum *accumulator) error {
	if mute {
		return nil
	}

	tmplSrc := `<!DOCTYPE html>
<html>
<head>
  <meta http-equiv="content-type" content="text/html; charset=UTF-8" />
  <style>
   .PASS { color: green; }
   .FAIL { color: red; }
   .ERROR { color: magenta; }
   .NOTRUN { color: grey; }
  </style>
  <title>Overall Results</title>
</head>
<body>
  <h1>Overall Result</h1>
  <ul>
    {{range .}}
    <li>
      <h2>
        <a href="./{{.Path}}/_Report_.html" >
          <span class="{{.Status}}">{{.Status}}</span> {{.Name}}
        </a>
      </h2>
    </li>
    {{end}}
  </ul>
</body>
</html>
`
	templ := template.Must(template.New("report").Parse(tmplSrc))
	buf := &bytes.Buffer{}
	err := templ.Execute(buf, accum.suites)
	if err != nil {
		return err
	}

	file := dirname + "/_Report_.html"
	if err := ioutil.WriteFile(file, buf.Bytes(), 0666); err != nil {
		log.Panicf("Failed to write file: %q with error %s", file, err)
	}
	cwd, err := os.Getwd()
	if err == nil {
		reportURL := "file://" + path.Join(cwd, file)
		fmt.Printf("Overall: %s\n", reportURL)
	}
	return err
}

func prepareHT() {
	// Set several parameters of package ht.
	if randomSeed == 0 {
		randomSeed = time.Now().UnixNano()
	}
	scope.ResetCounter <- counterSeed
	scope.Random = rand.New(rand.NewSource(randomSeed))
	ht.PhantomJSExecutable = phantomjs
	if !silent {
		fmt.Printf("Seeding random number generator with %d.\n", randomSeed)
		fmt.Printf("Resetting global counter to %d.\n", counterSeed)
		fmt.Printf("Using %q as PhantomJS executable.\n", phantomjs)
	}
	if skipTLSVerify {
		if !silent {
			fmt.Println("Skipping verification of TLS certificates presented by any server.")
		}
		ht.Transport.TLSClientConfig.InsecureSkipVerify = true
	}

	if !silent {
		// Log variables and values sorted by variable name.
		varnames := make([]string, 0, len(variablesFlag))
		for v := range variablesFlag {
			varnames = append(varnames, v)
		}
		sort.Strings(varnames)
		for _, v := range varnames {
			fmt.Printf("Variable %s = %q\n", v, variablesFlag[v])
		}
	}
}

func loadCookies() *cookiejar.Jar {
	if cookie == "" {
		return nil
	}
	buf, err := ioutil.ReadFile(cookie)
	if err != nil {
		log.Panicf("Cannot read cookie file: %s", err)
	}

	cookies := make(map[string]cookiejar.Entry)
	err = json.Unmarshal(buf, &cookies)
	if err != nil {
		log.Panicf("Cannot decode cookie file: %s", err)
	}
	cs := make([]cookiejar.Entry, 0, len(cookies))
	for _, c := range cookies {
		cs = append(cs, c)
	}

	jar, _ := cookiejar.New(nil)
	jar.LoadEntries(cs)
	return jar
}

// execute suites one by one saving each suite to disk once finished.
// Returns the accumulated overall result.
func executeSuites(suites []*suite.RawSuite, variables map[string]string, jar *cookiejar.Jar) (*accumulator, error) {
	bufferedStdout := bufio.NewWriterSize(os.Stdout, 256)
	defer bufferedStdout.Flush()
	logger := log.New(bufferedStdout, "", 0)
	errors := ht.ErrorList{}
	var err error

	if !mute {
		if outputDir == "" {
			outputDir = time.Now().Format("2006-01-02_15h04m05s")
		}
		err = os.MkdirAll(outputDir, 0766)
		errors = errors.Append(err)
	}

	accum := newAccumulator()
	for i, s := range suites {
		if !ssilent {
			logger.Println("Starting Suite", i+1, s.Name, s.File.Name)
		}
		outcome := s.Execute(variables, jar, logger)
		bufferedStdout.Flush()

		accum.update(outcome)

		if carryVars {
			variables = outcome.FinalVariables // carry over variables ???
		}
		if !silent {
			err = outcome.PrintReport(os.Stdout)
		} else if !ssilent {
			err = outcome.PrintShortReport(os.Stdout)
			fmt.Println()
		}
		errors = errors.Append(err)

		err = saveSingle(outputDir, outcome)
		errors = errors.Append(err)
		if len(suites) > 1 {
			err := saveOverallReport(outputDir, accum)
			errors = errors.Append(err)
		}

	}
	return accum, errors.AsError()
}

// saveSingle takes care of dumping the suite s into a subfolder of
// outputdir. It will produce:
//   _Report_.html  with accomaning files for the response bodies
//   junit-report.xml
//   result.txt
//   variables.json
//   cookies.json
func saveSingle(outputDir string, s *suite.Suite) error {
	if mute || outputDir == "/dev/null" {
		return nil
	}

	dirname := path.Join(outputDir, sanitize.Filename(s.Name))
	fmt.Printf("Saving result of suite %q to folder %q.\n", s.Name, dirname)
	err := os.MkdirAll(dirname, 0766)
	if err != nil {
		return err
	}

	errors := ht.ErrorList{}
	err = suite.HTMLReport(dirname, s)
	errors = errors.Append(err)

	file, err := os.Create(path.Join(dirname, "result.txt"))
	errors = errors.Append(err)
	if err == nil {
		err = s.PrintReport(file)
		errors = errors.Append(err)
		errors = errors.Append(file.Close())
	}

	cwd, err := os.Getwd()
	errors = errors.Append(err)
	reportURL := "file://" + path.Join(cwd, dirname, "_Report_.html")
	fmt.Printf("See %s\n", reportURL)

	junit, err := s.JUnit4XML()
	errors = errors.Append(err)
	if err == nil {
		err = ioutil.WriteFile(path.Join(dirname, "junit-report.xml"),
			[]byte(junit), 0666)
		errors = errors.Append(err)
	}

	// TODO: handle errors
	err = saveVariables(s.FinalVariables, path.Join(dirname, "variables.json"))
	errors = errors.Append(err)
	err = saveCookiesFromJar(s.Jar, path.Join(dirname, "cookies.json"))
	errors = errors.Append(err)

	return errors.AsError()
}
