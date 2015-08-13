// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"strings"
)

// Variables which can be set via the command line. Statisfied flag.Value interface.
type cmdlVar map[string]string

func (v cmdlVar) String() string { return "" }
func (v cmdlVar) Set(s string) error {
	part := strings.SplitN(s, "=", 2)
	if len(part) != 2 {
		return fmt.Errorf("Bad argument '%s' to -D commandline parameter", s)
	}
	v[part[0]] = part[1]
	return nil
}

// Includepath which can be set via the command line. Statisfied flag.Value interface.
type cmdlIncl []string

func (i *cmdlIncl) String() string { return "" }
func (i *cmdlIncl) Set(s string) error {
	s = strings.TrimRight(s, "/")
	*i = append(*i, s)
	return nil
}

// The common flags.
var (
	variablesFlag cmdlVar = make(cmdlVar) // flag -D
	onlyFlag      string
	skipFlag      string
	verbosity     int
)

func addVariablesFlag(fs *flag.FlagSet) {
	fs.Var(variablesFlag, "D", "set `parameter=value`")
}

func addOnlyFlag(fs *flag.FlagSet) {
	fs.StringVar(&onlyFlag, "only", "", "run only tests given by `testID`")
}

func addSkipFlag(fs *flag.FlagSet) {
	fs.StringVar(&skipFlag, "skip", "", "skip tests identified by `testID`")
}

func addVerbosityFlag(fs *flag.FlagSet) {
	fs.IntVar(&verbosity, "verbosity", -99, "verbosity to `level`")
}
