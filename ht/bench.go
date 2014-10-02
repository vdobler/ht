// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/vdobler/ht"
)

var cmdBench = &Command{
	Run:   runBench,
	Usage: "bench [-warmup n] [-count n] [-pause d] [-concurrent n] <suite.ht>...",
	Help: `
Benchmark the tests by running count many requests seperated by pause
after doing warmup many requests which are not measured.
`,
}

func init() {
	cmdBench.Flag.IntVar(&bcountFlag, "count", 17,
		"number of requests to measure")
	cmdBench.Flag.IntVar(&concurrentFlag, "concurrent", 1,
		"concurrency level")
	cmdBench.Flag.IntVar(&warmupFlag, "warmup", 3,
		"number of request to do before actual mesurement")
	cmdBench.Flag.DurationVar(&pauseFlag, "pause", 10*time.Millisecond,
		"duration to pause between requests")
}

var (
	bcountFlag     int
	warmupFlag     int
	pauseFlag      time.Duration
	concurrentFlag int
)

func runBench(cmd *Command, suites []*ht.Suite) {
	println(warmupFlag, bcountFlag, concurrentFlag)
	for s, suite := range suites {
		result := suite.ExecuteSetup()
		if result.Status != ht.Pass {
			log.Printf("Suite %d %q: Setup failure %q", s, suite.Name,
				result.Error.Error())
			continue
		}
		for _, test := range suite.Tests {
			if test.Poll.Max < 0 {
				continue
			}
			results := test.Benchmark(warmupFlag, bcountFlag, pauseFlag, concurrentFlag)
			fmt.Printf("Suite: %s; Test: %s\n", suite.Name, test.Name)
			printBenchmarkSummary(results)
		}
		suite.ExecuteTeardown()
	}
}

func printBenchmarkSummary(results []ht.Result) {
	durations := []int{}
	avg := 0
	for _, r := range results {
		d := int(r.Duration / time.Millisecond)
		durations = append(durations, d)
		avg += d
	}
	sort.Ints(durations)
	avg /= len(durations)

	p := func(x float64) int {
		i := int(x * float64(len(durations)))
		return durations[i]
	}

	fmt.Printf("N=%d Min=%d 25%%=%d Med=%d 75%%=%d Max=%d Avg=%d [ms]\n",
		len(durations), durations[0], p(0.25), p(0.5), p(0.75),
		durations[len(durations)-1], avg)
}
