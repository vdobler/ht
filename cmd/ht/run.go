// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"

	"github.com/vdobler/ht/ht"
)

var cmdRun = &Command{
	Run:         runRun,
	Usage:       "run <test>...",
	Description: "run a single test",
	Flag:        flag.NewFlagSet("run", flag.ContinueOnError),
	Help: `
Run loads the single test, unrolls it and prepares it
and executes the test (or the first of the unroled tests).
	`,
}

func init() {
	cmdRun.Flag.StringVar(&outputDir, "output", "",
		"save results to `dirname` instead of timestamp")
	addVariablesFlag(cmdRun.Flag)
	addVerbosityFlag(cmdRun.Flag)
}

func runRun(cmd *Command, suites []*ht.Suite) {
	runExecute(cmd, suites)
}
