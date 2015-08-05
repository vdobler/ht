// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"time"

	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/loghist"
)

var cmdBench = &Command{
	Run:         runBench,
	Usage:       "bench [flags] <suite>...",
	Description: "benchmark requests",
	Help: `
Benchmark the tests by running count many requests seperated by pause
after doing warmup many requests which are not measured.
`,
}

var (
	bcountFlag     int
	warmupFlag     int
	pauseFlag      time.Duration
	concurrentFlag int
	runTests       bool
)

func init() {
	cmdBench.Flag.IntVar(&bcountFlag, "count", 17,
		"measure `n` requests")
	cmdBench.Flag.IntVar(&concurrentFlag, "concurrent", 1,
		"run `conc` many request in parallel")
	cmdBench.Flag.IntVar(&warmupFlag, "warmup", 3,
		"warmup system with `n` unmeasured requests")
	cmdBench.Flag.DurationVar(&pauseFlag, "pause", 10*time.Millisecond,
		"sleep `duration` between requests")
	cmdBench.Flag.BoolVar(&runTests, "check", false,
		"execute checks defined in test")
	addVariablesFlag(&cmdBench.Flag)
	addOnlyFlag(&cmdBench.Flag)
	addSkipFlag(&cmdBench.Flag)
	addVerbosityFlag(&cmdBench.Flag)

}

func runBench(cmd *Command, suites []*ht.Suite) {
	println(warmupFlag, bcountFlag, concurrentFlag)
	for s, suite := range suites {
		suite.ExecuteSetup()
		if suite.Status != ht.Pass && suite.Status != ht.Skipped {
			log.Printf("Suite %d %q: Setup failure %q", s, suite.Name,
				suite.Error.Error())
			continue
		}
		for _, test := range suite.Tests {
			if test.Poll.Max < 0 {
				continue
			}
			if !runTests {
				test.Checks = nil
			}
			results := test.Benchmark(suite.Variables,
				warmupFlag, bcountFlag, pauseFlag, concurrentFlag)
			fmt.Printf("Suite: %s; Test: %s\n", suite.Name, test.Name)
			printBenchmarkSummary(results)
		}
		suite.ExecuteTeardown()
	}
}

func printBenchmarkSummary(results []ht.Test) {
	max := 0
	for _, r := range results {
		if d := int(r.Duration / 1e6); d > max {
			max = d
		}
	}
	h := loghist.New(7, max)
	for _, r := range results {
		h.Add(int(r.Duration / 1e6))
	}

	ps := []float64{0, 0.25, 0.50, 0.75, 0.80, 0.85, 0.90, 0.95, 0.97, 0.98, 0.99, 1}
	cps := make([]int, len(ps))
	for i, p := range ps {
		cps[i] = int(100*p + 0.2)
	}

	fmt.Printf("Percentil %4d \n", cps)
	fmt.Printf("Resp.Time %4d  [ms]\n", h.Quantiles(ps))
}
