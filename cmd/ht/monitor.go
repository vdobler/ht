// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/vdobler/ht/ht"
)

var cmdMonitor = &Command{
	RunSuites:   runMonitor,
	Usage:       "monitor [flags] <suite>...",
	Description: "periodically run suites",
	Flag:        flag.NewFlagSet("monitor", flag.ContinueOnError),
	Help: `
Monitor runs the given suites periodically and reports the various KPIs
via a simple web GUI whose address can be set with the -http flag.
If the suite(s) take longer than the interval given by -every the
actual intervall will be longer; i.e. the next run is not started
until the last has finished.
This feature is experimental.
	`,
}

var (
	everyFlag     time.Duration
	httpFlag      string
	templateFlag  string
	averageFlag   string
	averages      []int
	maxAverages   int
	inclSetupFlag bool
)

func init() {
	cmdMonitor.Flag.DurationVar(&everyFlag, "every", 120*time.Second,
		"execute once every `persiod`")
	cmdMonitor.Flag.StringVar(&httpFlag, "http", ":9090",
		"http service address")
	// cmdMonitor.Flag.StringVar(&templateFlag, "template", "", "use alternate template")
	cmdMonitor.Flag.BoolVar(&serialFlag, "serial", false,
		"run suites one after the other instead of concurrently")
	cmdMonitor.Flag.StringVar(&outputDir, "output", "",
		"save results to `dirname` instead of timestamp")
	cmdMonitor.Flag.StringVar(&averageFlag, "average", "1,3,9,15",
		"calculate running average over `n,m,..` runs")
	cmdMonitor.Flag.BoolVar(&inclSetupFlag, "includesetup", false,
		"include setup tests in reported numbers")
	addOnlyFlag(cmdMonitor.Flag)
	addSkipFlag(cmdMonitor.Flag)

	addTestFlags(cmdMonitor.Flag)

	reportTmpl = template.New("Report")
	reportTmpl = template.Must(reportTmpl.Parse(defaultReportTemplate))
}

type monitorResult struct {
	sync.Mutex

	data []*ht.SuiteResult
}

func (m *monitorResult) update(sr *ht.SuiteResult) {
	m.data = append(m.data, sr)
	n := maxAverages
	if len(m.data) > n {
		shift := len(m.data) - n
		for i := 0; i < n; i++ {
			m.data[i] = m.data[i+shift]
		}
		m.data = m.data[:n]
	}
}

func (m *monitorResult) N() int {
	return len(m.data)
}

func (m *monitorResult) Last() *ht.SuiteResult {
	if i := len(m.data); i > 0 {
		return m.data[i-1]
	}
	return ht.NewSuiteResult()
}

func (m *monitorResult) LastN(n int) *ht.SuiteResult {
	last := len(m.data)
	if n > last {
		n = last
	}
	if n == 0 {
		return ht.NewSuiteResult()
	}

	sr := ht.NewSuiteResult()
	for i := last - n; i < last; i++ {
		sr.Merge(m.data[i])
	}
	return sr
}

var (
	result     monitorResult
	reportTmpl *template.Template
)

func runMonitor(cmd *Command, suites []*ht.Suite) {
	prepareHT()
	maxAverages = 1
	for _, avg := range strings.Split(averageFlag, ",") {
		i, err := strconv.Atoi(avg)
		if err != nil {
			log.Fatalf("Cannot parse average flag: %s", err.Error())
		}
		averages = append(averages, i)
		if i > maxAverages {
			maxAverages = i
		}
	}

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
	result.Lock()
	defer result.Unlock()

	r.Header.Set("Content-Type", "text/plain")

	for _, avg := range averages {
		if avg > result.N() {
			fmt.Fprintf(w, "Not jet %d runs to average.\n", avg)
			continue
		}

		lastN := result.LastN(avg)

		fmt.Fprintf(w, "Average over latest %d runs from %s, took %s for %d tests:\n",
			avg,
			lastN.Started.Format(time.RFC1123),
			lastN.Duration.String(),
			lastN.Tests())
		fmt.Fprintf(w, "Error Index: %.1f%%    Real Bad: %.1f%%    All Wrong: %.1f%%\n",
			100*lastN.KPI(ht.DefaultPenaltyFunc),
			100*lastN.KPI(ht.JustBadPenaltyFunc),
			100*lastN.KPI(ht.AllWrongPenaltyFunc))
		fmt.Fprintf(w, "Details:\n%s\n\n\n", lastN.Matrix())
	}
}

func monitor(suites []*ht.Suite) {
	for {
		started := time.Now()
		executeSuites(suites)
		updateResult(suites)
		took := time.Since(started)
		remaining := everyFlag - took
		if remaining > 0 {
			log.Printf("Sleeping %s", remaining)
			time.Sleep(remaining)
		} else {
			log.Printf("Execution delayed by %s", -remaining)
		}
	}
}

func updateResult(suites []*ht.Suite) {
	result.Lock()
	defer result.Unlock()

	sr := ht.NewSuiteResult()
	for _, s := range suites {
		sr.Account(s, inclSetupFlag, false)
	}
	result.update(sr)
}
