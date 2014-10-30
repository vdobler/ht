// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/vdobler/ht"
)

var cmdExec = &Command{
	Run:   runExecute,
	Usage: "exec [-serial] <suite.ht>...",
	Help: `
Exec loads the given suites, unrolls the tests, prepares
the tests and executes them.
	`,
}

func init() {
	cmdExec.Flag.BoolVar(&serialFlag, "serial", false,
		"run suites one after the other instead of concurrently")
}

var (
	serialFlag bool
)

func runExecute(cmd *Command, suites []*ht.Suite) {

	results := make([]ht.Result, len(suites))

	var wg sync.WaitGroup
	for i := range suites {
		if serialFlag {
			results[i] = suites[i].Execute()
			if results[i].Status != ht.Pass {
				log.Printf("Suite %d %q failed: %s", i+1,
					suites[i].Name,
					results[i].Error.Error())
			}
		} else {
			wg.Add(1)
			go func(i int) {
				results[i] = suites[i].Execute()
				if results[i].Status != ht.Pass {
					log.Printf("Suite %d %q failed: %s", i+1,
						suites[i].Name, results[i].Error.Error())
				}
				wg.Done()
			}(i)
		}
	}
	wg.Wait()

	total, totalPass, totalError, totalSkiped, totalFailed, totalBogus := 0, 0, 0, 0, 0, 0
	for s := range suites {
		t := fmt.Sprintf("Suite %d: %s", s+1, suites[s].Name)
		fmt.Printf("\n%s\nFile %s\nStatus %s\n", ht.BoldBox(t, ""),
			suites[s].Name, results[s].Status)
		for k, r := range results[s].Elements {
			suites[s].Tests[k].PrintReport(os.Stdout, r)
			switch r.Status {
			case ht.Pass:
				totalPass++
			case ht.Error:
				totalError++
			case ht.Skipped:
				totalSkiped++
			case ht.Fail:
				totalFailed++
			case ht.Bogus:
				totalBogus++
			}
			total++
		}
	}
	fmt.Printf("Total %d,  Passed %d, Skipped %d,  Errored %d,  Failed %d,  Bogus %d\n",
		total, totalPass, totalSkiped, totalError, totalFailed, totalBogus)
}
