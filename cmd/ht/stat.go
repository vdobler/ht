// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/vdobler/ht/internal/asciistat"
	"github.com/vdobler/ht/suite"
)

var cmdStat = &Command{
	RunArgs:     runStat,
	Usage:       "stat <live.csv>",
	Description: "statistical analysis of a load test result",
	Flag:        flag.NewFlagSet("stat", flag.ContinueOnError),
	Help: `
Stat takes as input the live.csv output file of a load test (generated from
executing ht load) and produces an augmented output file:
 - sorted on start of the tests
 - added colum for actual request rate (QPS)
 - delay to previous request
`,
}

var (
	output    string // -output flag
	logplot   bool   // -log flag
	plotwidth int    // -plotwidth flag
)

func init() {
	// .IntVar(&recunstructWidth, "width", 64,
	// 	"make recunstructed image `w` pixel wide")
	cmdStat.Flag.StringVar(&output, "output", "throughput.csv",
		"save results to `name`")
	cmdStat.Flag.BoolVar(&logplot, "log", true,
		"show logarithmic scale on plot")
	cmdStat.Flag.IntVar(&plotwidth, "plotwidth", 120,
		"draw plot `num` chars wide")
	cmdStat.Flag.DurationVar(&rampDuration, "ramp", 5*time.Second,
		"ramp duration to ignore while computing average QPS")

}

func runStat(cmd *Command, args []string) {
	// Sanity check.
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Wrong number of arguments for stat")
		fmt.Fprintf(os.Stderr, "Usage: %s\n", cmd.Usage)
		os.Exit(9)
	}

	// Load data from file.
	file, err := os.Open(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read file %q: %s\n", args[0], err)
		os.Exit(9)
	}
	buffile := bufio.NewReader(file)

	data, err := suite.ReadLiveCSV(buffile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot load data: %s\n", err)
		os.Exit(9)
	}
	N := len(data)
	if N < 2 {
		fmt.Fprintf(os.Stderr, "Got only %d datapoints.\n", N)
		os.Exit(9)
	}

	// Output augmented data.
	if output != "" {
		ofile, err := os.Open(output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot open file %q for output: %s\n",
				output, err)
			os.Exit(9)
		}
		suite.DataToCSV(data, ofile)
		ofile.Close()
		fmt.Printf("Wrote augmented data to %q.\n", output)
	}

	// Compute average QPS and concurrency after ramp.
	rampend := 0
	start := data[0].Started
	for rampend < N && data[rampend].Started.Sub(start) < rampDuration {
		rampend++
	}
	if rampend < N-20 {
		reqDurationSum := time.Duration(0)
		for i := rampend; i < N; i++ {
			reqDurationSum += data[i].ReqDuration
		}
		flatDuration := data[len(data)-1].Started.Sub(data[rampend].Started)
		numRequest := len(data) - rampend // might be off by 1 but who cares
		qps := float64(numRequest) / float64(flatDuration/time.Second)
		fmt.Printf("Average request rate (QPS) after ramp: %.1f\n", qps)

		conc := float64(reqDurationSum) / float64(flatDuration)
		fmt.Printf("Average number of concurrent request: %.1f\n", conc)
	}

	// Print statistics.
	printStatistics(data)

	os.Exit(0)
}

func printStatistics(data []suite.TestData) {
	perTestData := []asciistat.Data{}

	// Per test
	perTest := make(map[string][]suite.TestData)
	for _, d := range data {
		parts := strings.Split(d.ID, suite.IDSep)
		testname := parts[2]
		perTest[testname] = append(perTest[testname], d)
	}

	sortedNames := []string{}
	nameLength := 0
	for name := range perTest {
		sortedNames = append(sortedNames, name)
		full := fmt.Sprintf("%s:", name)
		if len(full) > nameLength {
			nameLength = len(full)
		}
	}
	sort.Strings(sortedNames)
	for _, test := range sortedNames {
		st := statsFor(perTest[test])
		h := fmt.Sprintf("%s", test)
		printStat(os.Stdout, h, nameLength, st)
		perTestData = append(perTestData,
			asciistat.Data{Name: test, Values: st.data})
	}

	// All requests
	st := statsFor(data)
	printStat(os.Stdout, "All request:", nameLength, st)
	perTestData = append(perTestData,
		asciistat.Data{Name: "All requests", Values: st.data})
	asciistat.Plot(os.Stdout, perTestData, "ms", logplot, plotwidth)

}

func printStat(out io.Writer, headline string, hwidth int, st sdata) {
	fmt.Fprintf(out, "%-*s Status   Total=%d  Pass=%d (%.1f%%), Fail=%d (%.1f%%), Error=%d (%.1f%%), Bogus=%d (%.1f%%)\n",
		hwidth, headline, st.n,
		st.good, 100*float64(st.good)/float64(st.n),
		st.fail, 100*float64(st.fail)/float64(st.n),
		st.erred, 100*float64(st.erred)/float64(st.n),
		st.bogus, 100*float64(st.bogus)/float64(st.n),
	)
	fmt.Fprintf(out, "%-*s Duration 0%%=%.1fms, 25%%=%.1fms, 50%%=%.1fms, 75%%=%.1fms, 90%%=%.1fms, 95%%=%.1fms, 99%%=%.1fms, 100%%=%.1fms\n",
		hwidth, "",
		float64(st.min/1000)/1000,
		float64(st.q25/1000)/1000,
		float64(st.median/1000)/1000,
		float64(st.q75/1000)/1000,
		float64(st.q90/1000)/1000,
		float64(st.q95/1000)/1000,
		float64(st.q99/1000)/1000,
		float64(st.max/1000)/1000,
	)
}
