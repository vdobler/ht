// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/vdobler/ht/ht"
)

var cmdPerf = &Command{
	Run:         runPerf,
	Usage:       "perf [flags] <suite>...",
	Description: "run a performance/load test",
	Flag:        flag.NewFlagSet("run", flag.ContinueOnError),
	Help: `
Perf performs a load test
	`,
}

func init() {
	cmdPerf.Flag.Float64Var(&rateFlag, "rate", 20,
		"set average `rate` of requests per second")
	cmdPerf.Flag.IntVar(&countFlag, "count", 1000,
		"perform `num` requests")
	cmdPerf.Flag.DurationVar(&timeoutFlag, "timeout", 5*time.Minute,
		"set `maximum` duration to to run the load test")
	cmdPerf.Flag.BoolVar(&uniformFlag, "uniform", true,
		"use uniformly distributed requests instead of exponentialy distributed")
	cmdPerf.Flag.BoolVar(&concFlag, "conc", false,
		"do a concurrency test instead of a throughput test")
	addVariablesFlag(cmdPerf.Flag)
	addOnlyFlag(cmdPerf.Flag)
	addSkipFlag(cmdPerf.Flag)
	addVerbosityFlag(cmdPerf.Flag)

}

var (
	rateFlag    float64
	timeoutFlag time.Duration
	countFlag   int
	uniformFlag bool
	concFlag    bool
)

func runPerf(cmd *Command, suites []*ht.Suite) {
	opts := ht.LoadTestOptions{
		Type:    "throughput",
		Rate:    rateFlag,
		Count:   countFlag,
		Timeout: timeoutFlag,
		Uniform: uniformFlag,
	}
	if concFlag {
		opts.Type = "concurrency"
	}
	results, err := ht.PerformanceLoadTest(suites, opts)
	if err != nil {
		log.Fatal(err.Error())
	}
	ltr := ht.AnalyseLoadtest(results)

	fmt.Printf("%s\n", ltr)
}
