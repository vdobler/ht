// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	"github.com/vdobler/ht/ht"
)

var cmdList = &Command{
	Run:         runList,
	Usage:       "list <suite.ht>...",
	Description: "list tests in suits",
	Help: `
List loads the given suites, unrolls the tests, prepares
the tests and prints the list of tests.
	`,
}

func runList(cmd *Command, suites []*ht.Suite) {
	// TODO: provide templated output
	for sNo, suite := range suites {
		stitle := fmt.Sprintf("Suite %d: %s", sNo+1, suite.Name)
		fmt.Printf("%s\n", ht.Underline(stitle, "-", ""))
		for tNo, test := range suite.Setup {
			id := fmt.Sprintf("%d.u%d", sNo+1, tNo+1)
			fmt.Printf("%-6s %s\n", id, test.Name)
		}
		for tNo, test := range suite.Tests {
			id := fmt.Sprintf("%d.%d", sNo+1, tNo+1)
			fmt.Printf("%-6s %s\n", id, test.Name)
		}
		for tNo, test := range suite.Teardown {
			id := fmt.Sprintf("%d.d%d", sNo+1, tNo+1)
			fmt.Printf("%-6s %s\n", id, test.Name)
		}
	}
}
