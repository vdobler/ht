// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"

	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/suite"
)

var cmdList = &Command{
	RunSuites:   runList,
	Usage:       "list [flags] <suite>...",
	Description: "list tests in suits",
	Flag:        flag.NewFlagSet("run", flag.ContinueOnError),
	Help: `
List loads the given suites, unrolls the tests, prepares
the tests and prints the list of tests.
	`,
}

var (
	fullFlag bool
)

func init() {
	cmdList.Flag.BoolVar(&fullFlag, "full", false,
		"print more details")
}

func runList(cmd *Command, suites []*suite.RawSuite) {
	// TODO: provide templated output
	for sNo, s := range suites {
		fmt.Println()
		stitle := fmt.Sprintf("Suite %d: %s (%s)", sNo+1, s.Name, s.File.Name)
		fmt.Printf("%s\n", ht.Underline(stitle, "-", ""))
		for tNo, test := range s.RawTests() {
			typ := "Main"
			if tNo < len(s.Setup) {
				typ = "Setup"
			} else if tNo >= len(s.Setup)+len(s.Main) {
				typ = "Teardown"
			}
			id := fmt.Sprintf("%d.%d %-8s", sNo+1, tNo+1, typ)
			displayTest(id, test)
		}
	}
}

func displayTest(id string, test *suite.RawTest) {
	fmt.Printf("%-6s %s", id, test.File.Name)
	if fullFlag {
		ht, err := test.ToTest(variablesFlag)
		if err != nil {
			fmt.Printf("  oops: %s\n", err)
			return
		}
		method := "GET"
		if ht.Request.Method != "" {
			method = ht.Request.Method
		}
		fmt.Printf("  %q  %s %s\n", ht.Name, method, ht.Request.URL)
	}
	fmt.Println()
}
