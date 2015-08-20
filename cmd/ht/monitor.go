// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"log"
	"net/http"
	"sync"
	"text/template"
	"time"

	"github.com/vdobler/ht/ht"
)

var cmdMonitor = &Command{
	Run:         runMonitor,
	Usage:       "monitor [flags] <suite>...",
	Description: "periodically run suites",
	Flag:        flag.NewFlagSet("monitor", flag.ContinueOnError),
	Help: `
Monitor runs the suite peridically .... TODO
	`,
}

var (
	everyFlag    time.Duration
	httpFlag     string
	templateFlag string
)

func init() {
	cmdMonitor.Flag.DurationVar(&everyFlag, "every", 60*time.Second,
		"execute once every `persiod`")
	cmdMonitor.Flag.StringVar(&httpFlag, "http", ":9090",
		"http service address")
	cmdMonitor.Flag.StringVar(&templateFlag, "template", "",
		"use alternate template")
	cmdMonitor.Flag.BoolVar(&serialFlag, "serial", false,
		"run suites one after the other instead of concurrently")
	cmdMonitor.Flag.StringVar(&outputDir, "output", "",
		"save results to `dirname` instead of timestamp")
	addVariablesFlag(cmdMonitor.Flag)
	addOnlyFlag(cmdMonitor.Flag)
	addSkipFlag(cmdMonitor.Flag)
	addVerbosityFlag(cmdMonitor.Flag)

	reportTmpl = template.New("Report")
	reportTmpl = template.Must(reportTmpl.Parse(defaultReportTemplate))
}

type monitorResult struct {
	Date                                            time.Time
	NSuites                                         int
	NotRun, Skipped, Passed, Failed, Errored, Bogus int
}

var (
	reportTmpl *template.Template

	mu         sync.Mutex
	lastResult monitorResult
)

func runMonitor(cmd *Command, suites []*ht.Suite) {

	go monitor(suites)

	http.HandleFunc("/", showReports)
	log.Fatal(http.ListenAndServe(httpFlag, nil))
}

var defaultReportTemplate = `NotRun  {{.NotRun}}
Skipped {{.Skipped}}
Pass    {{.Passed}}
Failed  {{.Failed}}
Errored {{.Errored}}
Bogus   {{.Bogus}}
Suites  {{.NSuites}}
Date    {{.Date}}
`

func showReports(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	// TODO: err handling is broken as resp is sent
	r.Header.Set("Content-Type", "text/plain")
	err := reportTmpl.Execute(w, lastResult)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}

func monitor(suites []*ht.Suite) {
	for {
		executeSuites(suites)
		updateResult(suites)
	}
}

func updateResult(suites []*ht.Suite) {
	result := monitorResult{Date: time.Now()}
	for s := range suites {
		// Statistics
		for _, r := range suites[s].AllTests() {
			switch r.Status {
			case ht.Pass:
				result.Passed++
			case ht.Error:
				result.Errored++
			case ht.Skipped:
				result.Skipped++
			case ht.Fail:
				result.Failed++
			case ht.Bogus:
				result.Bogus++
			}
		}

	}
	mu.Lock()
	defer mu.Unlock()
	lastResult = result
	// TODO: keep running average
}

func executeSuites(suites []*ht.Suite) {
	// TODO: same code as exec
	var wg sync.WaitGroup
	for i := range suites {
		if serialFlag {
			suites[i].Execute()
			if suites[i].Status > ht.Pass {
				log.Printf("Suite %d %q failed: %s", i+1,
					suites[i].Name,
					suites[i].Error.Error())
			}
		} else {
			wg.Add(1)
			go func(i int) {
				suites[i].Execute()
				if suites[i].Status > ht.Pass {
					log.Printf("Suite %d %q failed: %s", i+1,
						suites[i].Name, suites[i].Error.Error())
				}
				wg.Done()
			}(i)
		}
	}
	wg.Wait()
}
