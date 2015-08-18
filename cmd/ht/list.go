// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"

	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/internal/json5"
)

var cmdList = &Command{
	Run:         runList,
	Usage:       "list [flags] <suite>...",
	Description: "list tests in suits",
	Flag:        flag.NewFlagSet("run", flag.ContinueOnError),
	Help: `
List loads the given suites, unrolls the tests, prepares
the tests and prints the list of tests.
	`,
}

var (
	checkFlag bool
	fullFlag  bool
)

func init() {
	cmdList.Flag.BoolVar(&checkFlag, "check", false,
		"including checks inlisting")
	cmdList.Flag.BoolVar(&fullFlag, "full", false,
		"print complete tests")
}

func runList(cmd *Command, suites []*ht.Suite) {
	// TODO: provide templated output
	for sNo, suite := range suites {
		stitle := fmt.Sprintf("Suite %d: %s", sNo+1, suite.Name)
		fmt.Printf("%s\n", ht.Underline(stitle, "-", ""))
		for tNo, test := range suite.Setup {
			id := fmt.Sprintf("%d.u%d", sNo+1, tNo+1)
			displayTest(id, test)
		}
		for tNo, test := range suite.Tests {
			id := fmt.Sprintf("%d.%d", sNo+1, tNo+1)
			displayTest(id, test)
		}
		for tNo, test := range suite.Teardown {
			id := fmt.Sprintf("%d.d%d", sNo+1, tNo+1)
			displayTest(id, test)
		}
	}
}

func displayTest(id string, test *ht.Test) {
	fmt.Printf("%-6s %s\n", id, test.Name)
	if fullFlag {
		buf, err := json5.MarshalIndent(test, "         ", "    ")
		if err != nil {
			buf = []byte(err.Error())
		}
		fmt.Println("        ", string(buf))
		fmt.Println()
	} else if checkFlag {
		for _, check := range test.Checks {
			name := ht.NameOf(check)
			buf, err := json5.Marshal(check)
			if err != nil {
				buf = []byte(err.Error())
			}
			fmt.Printf("           %-14s %s\n", name, buf)
		}
		fmt.Println()
	}
}
