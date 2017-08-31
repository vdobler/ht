// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"strings"
)

// cmdlVar captures name=value pairs settable on the command line
// via the -D flag. For this cmdlVar satisfies the flag.Value interface.
type cmdlVar map[string]string

func (v *cmdlVar) String() string { return "" }
func (v *cmdlVar) Set(s string) error {
	part := strings.SplitN(s, "=", 2)
	if len(part) != 2 {
		return fmt.Errorf("Bad argument '%s' to -D commandline parameter", s)
	}
	(*v)[part[0]] = part[1]
	return nil
}

// ----------------------------------------------------------------------------
// Common flags

var (
	variablesFlag    = make(cmdlVar) // flag -D
	variablesFile    string          // -Dfile
	onlyFlag         string          // flag -only
	skipFlag         string          // flag -skip
	verbosity        int             // flag -verbosity
	outputDir        string          // flag -output
	randomSeed       int64           // flag -seed
	counterSeed      int             // flag -counter
	skipTLSVerify    bool            // flag -skiptlsverify
	phantomjs        string          // flag -phantomjs
	v, vv, vvv, vvvv bool            // flag -v, -vv, -vvv, -vvvv
	silent, ssilent  bool            // flag -s, -ss
	mute             bool            // flage -mute
	vardump          string          // flag -vardump
	cookiedump       string          // flag -cookiedump
	cookie           string          // flag -cookie
	port             string          // flag -port

)

func addVarsFlags(fs *flag.FlagSet) {
	addVariablesFlag(fs)
	addDfileFlag(fs)
}

func addTestFlags(fs *flag.FlagSet) {
	addVarsFlags(fs)
	addVerbosityFlag(fs)
	addSeedFlag(fs)
	addCounterFlag(fs)
	addSkiptlsverifyFlag(fs)
	addPhantomJSFlag(fs)
	addDumpFlag(fs)
	addCookieFlag(fs)
}

func addDfileFlag(fs *flag.FlagSet) {
	fs.StringVar(&variablesFile, "Dfile", "",
		"read variables from `file.json`")
}

func addOutputFlag(fs *flag.FlagSet) {
	fs.StringVar(&outputDir, "output", "",
		"save results to `dirname` instead of timestamp")
}

func addSeedFlag(fs *flag.FlagSet) {
	fs.Int64Var(&randomSeed, "seed", 0,
		"use `num` as seed for PRNG (0 will take seed from time)")
}

func addCounterFlag(fs *flag.FlagSet) {
	fs.IntVar(&counterSeed, "counter", 1,
		"use `num` as start value for COUNTER variables")
}

func addSkiptlsverifyFlag(fs *flag.FlagSet) {
	fs.BoolVar(&skipTLSVerify, "skiptlsverify", false,
		"do not verify TLS certificate chain of servers")
}

func addPhantomJSFlag(fs *flag.FlagSet) {
	fs.StringVar(&phantomjs, "phantomjs", "phantomjs",
		"PhantomJS executable")
}

func addVariablesFlag(fs *flag.FlagSet) {
	fs.Var(&variablesFlag, "D", "set `parameter=value`")
}

func addOnlyFlag(fs *flag.FlagSet) {
	fs.StringVar(&onlyFlag, "only", "", "run only tests given by `testID`")
}

func addSkipFlag(fs *flag.FlagSet) {
	fs.StringVar(&skipFlag, "skip", "", "skip tests identified by `testID`")
}

func addVerbosityFlag(fs *flag.FlagSet) {
	fs.IntVar(&verbosity, "verbosity", -99, "set verbosity to `level`")
	fs.BoolVar(&v, "v", false, "increase verbosity by 1")
	fs.BoolVar(&vv, "vv", false, "increase verbosity by 2")
	fs.BoolVar(&vvv, "vvv", false, "increase verbosity by 3")
	fs.BoolVar(&vvvv, "vvvv", false, "increase verbosity by 4")
	fs.BoolVar(&silent, "s", false, "make ht itself more silent")
	fs.BoolVar(&ssilent, "ss", false, "make ht itself really silent")
	fs.BoolVar(&mute, "mute", false, "disable result generation")
}

func addDumpFlag(fs *flag.FlagSet) {
	fs.StringVar(&vardump, "vardump", "",
		"save variables to `vars.json` after completion")
}

func addCookieFlag(fs *flag.FlagSet) {
	fs.StringVar(&cookiedump, "cookiedump", "",
		"save cookies of all suites to `cookies.json`")
	fs.StringVar(&cookie, "cookies", "",
		"read initial cookies for each suite from `cookies.json`")
}

func addPortFlag(fs *flag.FlagSet) {
	fs.StringVar(&port, "port", ":8888", "http service address, e.g. ")
}
