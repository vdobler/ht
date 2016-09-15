// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"

	"github.com/vdobler/ht/suite"
)

var cmdLoad = &Command{
	RunSuites:   runLoad,
	Usage:       "load [options] <suite>...",
	Description: "run suites in a load test",
	Flag:        flag.NewFlagSet("load", flag.ContinueOnError),
	Help: `
TODO
	`,
}

var queryPerSecond float64

func init() {
	cmdLoad.Flag.Float64Var(&queryPerSecond, "rate", 5,
		"make `qps` reqest per second")
}

func runLoad(cmd *Command, suites []*suite.RawSuite) {
	prepareHT()
	suite.Throughput(suites, queryPerSecond, variablesFlag)
}
