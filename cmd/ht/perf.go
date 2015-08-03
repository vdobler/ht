// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"time"

	"github.com/vdobler/ht/ht"
)

var cmdPerf = &Command{
	Run:         runPerf,
	Usage:       "perf [-conc] [-rate rps] [-timeout t] [-count n] [-uniform] <suite.ht>...",
	Description: "run a performance/load test",
	Help: `
Perf performs a load test
	`,
}

func init() {
	cmdPerf.Flag.Float64Var(&rateFlag, "rate", 20,
		"average rate of requests per second")
	cmdPerf.Flag.IntVar(&countFlag, "count", 1000,
		"number of requests to perform")
	cmdPerf.Flag.DurationVar(&timeoutFlag, "timeout", 5*time.Minute,
		"maximum time to run the load test")
	cmdPerf.Flag.BoolVar(&uniformFlag, "uniform", true,
		"use uniformly distributed requests instead of exponentialy ditrubuted")
	cmdPerf.Flag.BoolVar(&concFlag, "conc", false,
		"do a concurrency test instead of a througput test")
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
	results, err := ht.LoadTest(suites, opts)
	if err != nil {
		log.Fatal(err.Error())
	}
	ltr := ht.AnalyseLoadtest(results)

	fmt.Printf("%s\n", ltr)
}
