// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
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
	averageFlag  int
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
	cmdMonitor.Flag.IntVar(&averageFlag, "average", 5,
		"calculate running average over `n` runs")
	addVariablesFlag(cmdMonitor.Flag)
	addOnlyFlag(cmdMonitor.Flag)
	addSkipFlag(cmdMonitor.Flag)
	addVerbosityFlag(cmdMonitor.Flag)

	reportTmpl = template.New("Report")
	reportTmpl = template.Must(reportTmpl.Parse(defaultReportTemplate))
}

type monitorResult struct {
	sync.Mutex

	data []*ht.SuiteResult
}

func (m *monitorResult) update(sr *ht.SuiteResult) {
	m.data = append(m.data, sr)
	n := averageFlag
	if len(m.data) > n {
		shift := len(m.data) - n
		for i := 0; i < n; i++ {
			m.data[i] = m.data[i+shift]
		}
		m.data = m.data[:n]
	}
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

	last := result.Last()
	lastN := result.LastN(averageFlag)

	fmt.Fprintf(w, "Latest run from %s, took %s for %d tests:\n",
		last.Started.Format(time.RFC1123), last.Duration.String(),
		last.Tests())
	fmt.Fprintf(w, "%s\n", last.Matrix())
	fmt.Fprintf(w, "Default KPI: %.4f    Binary KPI: %.4f    Fail is Fail KPI: %.4f\n\n\n",
		last.KPI(ht.DefaultPenaltyFunc), last.KPI(ht.BinaryPenaltyFunc),
		last.KPI(ht.FailIsFailPenaltyFunc))

	fmt.Fprintf(w, "Average over last %d runs from %s, took in total %s for %d tests:\n",
		len(result.data), lastN.Started.Format(time.RFC1123),
		lastN.Duration.String(), lastN.Tests())
	fmt.Fprintf(w, "%s\n", lastN.Matrix())
	fmt.Fprintf(w, "Default KPI: %.4f    Binary KPI: %.4f    Fail is Fail KPI: %.4f\n\n\n",
		lastN.KPI(ht.DefaultPenaltyFunc), lastN.KPI(ht.BinaryPenaltyFunc),
		lastN.KPI(ht.FailIsFailPenaltyFunc))

}

func monitor(suites []*ht.Suite) {
	for {
		executeSuites(suites)
		updateResult(suites)
	}
}

func updateResult(suites []*ht.Suite) {
	result.Lock()
	defer result.Unlock()

	sr := ht.NewSuiteResult()
	for _, s := range suites {
		sr.Account(s, true, false) // TODO: expose bools?
	}
	result.update(sr)
}
