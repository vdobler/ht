// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestExecuteSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping execution of showcase suite in short mode.")
		return
	}
	suite, err := LoadSuite("../showcase/showcase.suite")
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}

	if *verboseTest {
		suite.Log = log.New(os.Stdout, "", log.LstdFlags)
	} else {
		for _, test := range suite.AllTests() {
			test.Verbosity = 0
		}
	}

	err = suite.Prepare()
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
	if testing.Short() {
		t.Skip("Skipping execution without network in short mode.")
	}

	// Start showcase demo server
	cmd := exec.Command("go", "run", "../showcase/showcase.go")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Cannot run showcase server: %s", err)
	}
	time.Sleep(20 * time.Millisecond)

	suite.Execute()
	if suite.Status != Pass {
		for _, tr := range suite.AllTests() {
			if *verboseTest {
				fmt.Println(tr.Status, ": ", tr.Name)
				if tr.Error != nil {
					fmt.Println("  Error: ", tr.Error)
				}
			}
		}
	}

	if *verboseTest {
		fmt.Printf("\nDefault Text Output:\n")
		suite.PrintReport(os.Stdout)
		junit, err := suite.JUnit4XML()
		if err != nil {
			t.Fatalf("Unexpected error: %+v", err)
		}
		fmt.Printf("\nJUnit 4 XML Output:\n%s", junit)
		sr := NewSuiteResult()
		sr.Account(suite, true, true)
		fmt.Println(sr.Matrix())
		fmt.Printf("Default KPI: %.3f   JustBad KPI: %.3f    KPI: %.3f\n",
			sr.KPI(DefaultPenaltyFunc), sr.KPI(JustBadPenaltyFunc),
			sr.KPI(AllWrongPenaltyFunc))
	}

	cmd.Wait()
}
