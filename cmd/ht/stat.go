// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"

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
 - aveeraged actual request rate (QPS)
 - delay to previous request
 - ...
`,
}

var (
	output string
)

func init() {
	// .IntVar(&recunstructWidth, "width", 64,
	// 	"make recunstructed image `w` pixel wide")
	cmdStat.Flag.StringVar(&output, "output", "throughput.csv",
		"save results to `name`")
}

func runStat(cmd *Command, args []string) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Wrong number of arguments for stat")
		fmt.Fprintf(os.Stderr, "Usage: %s\n", cmd.Usage)
		os.Exit(9)
	}

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

	suite.DataToCSV(data, os.Stdout)

	os.Exit(0)
}
