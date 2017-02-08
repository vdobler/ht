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
	prepareHT()
	jar := loadCookies()

	outcome := executeSuites(suites, variablesFlag, jar)
	saveOutcome(outcome)
}

func saveOutcome(outcome []*suite.Suite) {
	if outputDir == "" {
		outputDir = time.Now().Format("2006-01-02_15h04m05s")
	}
	os.MkdirAll(outputDir, 0766)
	total, totalPass, totalError, totalSkiped, totalFailed, totalBogus := 0, 0, 0, 0, 0, 0
	for _, s := range outcome {
		s.PrintReport(os.Stdout)
	}

	overallStatus := ht.NotRun
	overallVars := make(map[string]string)
	overallCookies := make(map[string]cookiejar.Entry)

	for _, s := range outcome {
		// Statistics
		if s.Status > overallStatus {
			overallStatus = s.Status
		}
		for _, r := range s.Tests {
			switch r.Status {
			case ht.Pass:
				totalPass++
			case ht.Error:
				totalError++
			case ht.Skipped:
				totalSkiped++
			case ht.Fail:
				totalFailed++
			case ht.Bogus:
				totalBogus++
			}
			total++
		}

		dirname := outputDir + "/" + sanitize.Filename(s.Name)
		fmt.Printf("Saving result of suite %q to folder %q.\n", s.Name, dirname)
		err := os.MkdirAll(dirname, 0766)
		if err != nil {
			log.Panic(err)
		}
		err = suite.HTMLReport(dirname, s)
		if err != nil {
			log.Panic(err)
		}

		cwd, err := os.Getwd()
		if err == nil {
			reportURL := "file://" + path.Join(cwd, dirname, "_Report_.html")
			fmt.Printf("See %s\n", reportURL)
		}
		junit, err := s.JUnit4XML()
		if err != nil {
			log.Panic(err)
		}
		err = ioutil.WriteFile(dirname+"/junit-report.xml", []byte(junit), 0666)
		if err != nil {
			log.Panic(err)
		}

		// Consolidate all variables.
		saveVariables(s.FinalVariables, path.Join(dirname, "variables.json"))
		for name, value := range s.FinalVariables {
			overallVars[name] = value
		}
		// Consolidate cookies.
		if jar := s.Jar; jar != nil {
			cookies := make(map[string]cookiejar.Entry)
			for _, tld := range jar.ETLDsPlus1(nil) {
				for _, cookie := range jar.Entries(tld, nil) {
					id := cookie.ID()
					overallCookies[id] = cookie
					cookies[id] = cookie
				}
			}
			saveCookies(cookies, path.Join(dirname, "cookies.json"))
		}
	}

	// Save consolidated variables if required.
	if vardump != "" {
		if err := saveVariables(overallVars, vardump); err != nil {
			log.Panic(err)
		}
	}

	// Save consolidated cookies if required.
	if cookiedump != "" {
		if err := saveCookies(overallCookies, cookiedump); err != nil {
			log.Panic(err)
		}
	}

	// Save a overall report iff more than one suite was involved.
	if len(outcome) > 1 {
		if err := saveOverallReport(outputDir, outcome); err != nil {
			log.Panic(err)
		}
	}

	fmt.Println()
	fmt.Printf("Total %d,  Passed %d,  Skipped %d,  Errored %d,  Failed %d,  Bogus %d\n",
		total, totalPass, totalSkiped, totalError, totalFailed, totalBogus)

	switch overallStatus {
	case ht.NotRun:
		fmt.Println("NOTRUN")
		os.Exit(0)
	case ht.Skipped:
		fmt.Println("SKIPPED")
		os.Exit(0)
	case ht.Pass:
		fmt.Println("PASS")
		os.Exit(0)
	case ht.Fail:
		fmt.Println("FAIL")
		os.Exit(1)
	case ht.Error:
		fmt.Println("ERROR")
		os.Exit(2)
	case ht.Bogus:
		fmt.Println("BOGUS")
		os.Exit(3)
	}
	panic(fmt.Sprintf("Ooops: Unknown overall status %d", overallStatus))
}

func saveVariables(vars map[string]string, filename string) error {
	b, err := json.MarshalIndent(vars, "    ", "")
	if err != nil {
		return nil
	}
	return ioutil.WriteFile(filename, b, 0666)
}

func saveCookies(cookies map[string]cookiejar.Entry, filename string) error {
	b, err := json.MarshalIndent(cookies, "    ", "")
	if err != nil {
		return nil
	}
	return ioutil.WriteFile(filename, b, 0666)
}

func saveOverallReport(dirname string, outcome []*suite.Suite) error {
	type Data struct {
		Name, Path, Status string
	}
	data := []Data{}
	for _, s := range outcome {
		data = append(data,
			Data{
				Name:   s.Name,
				Path:   sanitize.Filename(s.Name),
				Status: strings.ToUpper(s.Status.String()),
			})
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
	err := templ.Execute(buf, data)
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
	fmt.Printf("Seeding random number generator with %d.\n", randomSeed)
	ht.Random = rand.New(rand.NewSource(randomSeed))
	if skipTLSVerify {
		fmt.Println("Skipping verification of TLS certificates presented by any server.")
		ht.Transport.TLSClientConfig.InsecureSkipVerify = true
	}
	ht.PhantomJSExecutable = phantomjs
	fmt.Printf("Using %q as PhantomJS executable.\n", phantomjs)

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

func executeSuites(suites []*suite.RawSuite, variables map[string]string, jar *cookiejar.Jar) []*suite.Suite {
	bufferedStdout := bufio.NewWriterSize(os.Stdout, 256)
	defer bufferedStdout.Flush()
	logger := log.New(bufferedStdout, "", 0)

	outcome := make([]*suite.Suite, len(suites))
	for i, s := range suites {
		logger.Println("Starting Suite", i+1, s.Name, s.File.Name)
		outcome[i] = s.Execute(variables, jar, logger)
		if carryVars {
			variables = outcome[i].FinalVariables // carry over variables ???
		}
		bufferedStdout.Flush()
	}
	return outcome
}
