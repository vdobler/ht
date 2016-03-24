// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"log"
	"os"

	"github.com/vdobler/ht/ht"
)

var cmdWarmup = &Command{
	RunSuites:   runWarmup,
	Usage:       "warmup [options] <suite>...",
	Description: "make HTTP requests without testing",
	Flag:        flag.NewFlagSet("run", flag.ContinueOnError),
	Help: `
Warmup works like 'exec', but no checks are executed and request
errors are ignored.
The exit code is 3 if bogus tests or checks are found, and 0 otherwise.
`,
}

var repsFlag = 1

func init() {
	cmdWarmup.Flag.IntVar(&repsFlag, "reps", 2,
		"execute suites `n` times")
	addVariablesFlag(cmdWarmup.Flag)
	addDfileFlag(cmdWarmup.Flag)
	addOnlyFlag(cmdWarmup.Flag)
	addSkipFlag(cmdWarmup.Flag)
	addVerbosityFlag(cmdWarmup.Flag)
	addSeedFlag(cmdWarmup.Flag)
	addSkiptlsverifyFlag(cmdWarmup.Flag)
}

func runWarmup(cmd *Command, suites []*ht.Suite) {
	prepareExecution()

	totalBogus := 0
	for rep := 1; rep <= repsFlag; rep++ {
		log.Printf("Warmup round %d of %d", rep, repsFlag)
		for _, s := range suites {
			s.OmitChecks = true
			log.Printf("  Suite %s", s.Name)
			s.Execute()
			for _, r := range s.AllTests() {
				if r.Status == ht.Bogus {
					totalBogus++
				}
			}
			if totalBogus > 0 {
				os.Exit(3)
			}
		}
	}
	os.Exit(0)
}
