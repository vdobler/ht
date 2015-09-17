// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/vdobler/ht/ht"
)

var cmdExec = &Command{
	Run:         runExecute,
	Usage:       "exec [options] <suite>...",
	Description: "generate request and test response",
	Flag:        flag.NewFlagSet("run", flag.ContinueOnError),
	Help: `
Exec loads the given suites, unrolls the tests, prepares the tests and
executes them. The flags -skip and -only allow to fine controll which
tests in the suite(s) are executed.
The exit code is 3 if bogus tests or checks are found, 2 if test errors
are present, 1 if only check failures occured and 0 if everything passed.
	`,
}

func init() {
	cmdExec.Flag.BoolVar(&serialFlag, "serial", false,
		"run suites one after the other instead of concurrently")
	cmdExec.Flag.StringVar(&outputDir, "output", "",
		"save results to `dirname` instead of timestamp")
	cmdExec.Flag.Int64Var(&randomSeed, "seed", 0,
		"use `num` as seed for PRNG (0 will take seed from time)")
	cmdExec.Flag.BoolVar(&skipTLSVerify, "skiptlsverify", false,
		"do not verify TLS certificate chain of servers")
	addVariablesFlag(cmdExec.Flag)
	addOnlyFlag(cmdExec.Flag)
	addSkipFlag(cmdExec.Flag)
	addVerbosityFlag(cmdExec.Flag)
}

var (
	serialFlag    bool
	outputDir     string
	randomSeed    int64
	skipTLSVerify bool
	sanitizer     = strings.NewReplacer(" ", "_", ":", "_", "@", "_at_", "/", "_",
		"*", "_", "?", "_", "#", "_", "$", "_", "<", "_", ">", "_", "~", "_",
		"ä", "ae", "ö", "oe", "ü", "ue", "Ä", "Ae", "Ö", "Oe", "Ü", "Ue",
		"%", "_", "&", "+", "(", "_", ")", "_", "'", "_", "`", "_", "^", "_")
)

func runExecute(cmd *Command, suites []*ht.Suite) {
	if outputDir == "" {
		outputDir = time.Now().Format("2006-01-02_15h04m05s")
	}
	os.MkdirAll(outputDir, 0766)

	if randomSeed == 0 {
		randomSeed = time.Now().UnixNano()
	}
	log.Printf("Seeding random number generator with %d", randomSeed)
	ht.Random = rand.New(rand.NewSource(randomSeed))
	if skipTLSVerify {
		log.Printf("Skipping verification of TLS certificates presented by any server.")
		ht.Transport.TLSClientConfig.InsecureSkipVerify = true
	}

	executeSuites(suites)

	total, totalPass, totalError, totalSkiped, totalFailed, totalBogus := 0, 0, 0, 0, 0, 0
	for s := range suites {
		suites[s].PrintReport(os.Stdout)

		// Statistics
		for _, r := range suites[s].AllTests() {
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
		junit, err := suites[s].JUnit4XML()
		if err != nil {
			log.Panic(err)
		}
		err = ioutil.WriteFile(dirname+"/junit-report.xml", []byte(junit), 0666)
	}
	fmt.Printf("Total %d,  Passed %d, Skipped %d,  Errored %d,  Failed %d,  Bogus %d\n",
		total, totalPass, totalSkiped, totalError, totalFailed, totalBogus)

	if totalBogus > 0 {
		os.Exit(3)
	} else if totalError > 0 {
		os.Exit(2)
	} else if totalFailed > 0 {
		os.Exit(2)
	}

	os.Exit(0)
}

func executeSuites(suites []*ht.Suite) {
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
}
