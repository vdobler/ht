// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/vdobler/ht/ht"
)

var cmdRun = &Command{
	Run:         runRun,
	Usage:       "run <test>...",
	Description: "run a single test",
	Help: `
Run loads the single test, unrolls it and prepares it
and executes the test (or the first of the unroled tests).
	`,
}

func runRun(cmd *Command, suites []*ht.Suite) {
	if outputDir == "" {
		outputDir = time.Now().Format("2006-01-02_15h04m05s")
	}
	os.MkdirAll(outputDir, 0766)
	var wg sync.WaitGroup
	for i := range suites {
		if serialFlag {
			suites[i].Execute()
			if suites[i].Status > ht.Pass {
				log.Printf("Suite %d %q failed: %s", i+1,
					suites[i].Name,
					suites[i].Error.Error())
			}
		} else {
			wg.Add(1)
			go func(i int) {
				suites[i].Execute()
				if suites[i].Status > ht.Pass {
					log.Printf("Suite %d %q failed: %s", i+1,
						suites[i].Name, suites[i].Error.Error())
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
			suites[s].Name, suites[s].Status)
		for _, r := range suites[s].AllTests() {
			r.PrintReport(os.Stdout)
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

		dirname := outputDir + "/" + sanitizer.Replace(suites[s].Name)

		fmt.Printf("Saveing result of suite %q to folder %q.\n", suites[s].Name, dirname)
		err := os.MkdirAll(dirname, 0766)
		if err != nil {
			log.Panic(err)
		}
		err = suites[s].HTMLReport(dirname)
		if err != nil {
			log.Panic(err)
		}
	}
	fmt.Printf("Total %d,  Passed %d, Skipped %d,  Errored %d,  Failed %d,  Bogus %d\n",
		total, totalPass, totalSkiped, totalError, totalFailed, totalBogus)

}
