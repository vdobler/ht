// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"strconv"
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

// cmdlIncl captures a list of include paths settable on the command line
// via the -I flag. For this cmdlIncl satisfies the flag.Value interface.
// Currently unused.
type cmdlIncl []string

func (i *cmdlIncl) String() string { return "" }
func (i *cmdlIncl) Set(s string) error {
	s = strings.TrimRight(s, "/")
	*i = append(*i, s)
	return nil
}

// cmdlLimit captures quantile=timelimit pairs settable on the command line
// via the -L flag during benchmarking
type cmdlLimit map[float64]int

func (l *cmdlLimit) String() string { return "" }
func (l *cmdlLimit) Set(s string) error {
	parts := strings.SplitN(s, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("Bad argument '%s' to -L command line parameter", s)
	}
	quantile, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return fmt.Errorf("Cannot parse '%s' as float in -L command line parameter",
			parts[0])
	}
	if quantile < 0 || quantile > 1 {
		return fmt.Errorf("Quantile '%s' out of range [0,1] in -L command line parameter",
			parts[0])
	}
	limit, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return fmt.Errorf("Cannot parse '%s' as int in -L command line parameter",
			parts[1])
	}
	if limit < 0 || limit > 2000000000 {
		return fmt.Errorf("Limit '%s' out of range in -L command line parameter",
			parts[1])
	}

	(*l)[quantile] = int(limit)
	return nil
}

// The common flags.
var (
	variablesFlag cmdlVar   = make(cmdlVar)   // flag -D
	rtLimits      cmdlLimit = make(cmdlLimit) // flag -L
	onlyFlag      string
	skipFlag      string
	verbosity     int
)

func addVariablesFlag(fs *flag.FlagSet) {
	fs.Var(&variablesFlag, "D", "set `parameter=value`")
}

func addLimitFlag(fs *flag.FlagSet) {
	fs.Var(&rtLimits, "L", "set responste time limit of `quantile=millisecond`")
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
