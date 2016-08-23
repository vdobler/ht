// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// ht generates HTTP requests and checks the received responses.
//
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/vdobler/ht/internal/json5"
	"github.com/vdobler/ht/suite"
)

// A Command is one of the subcommands of ht.
type Command struct {
	// One of RunSuites, RunTest and RunArgs must be provided by the command.
	RunSuites func(cmd *Command, suites []*suite.RawSuite)
	RunTests  func(cmd *Command, tests []*suite.RawTest)
	RunArgs   func(cmd *Command, tests []string)

	Usage       string        // must start with command name
	Description string        // short description for 'ht help'
	Help        string        // the output of 'ht help <cmd>'
	Flag        *flag.FlagSet // the flags for this command
}

// Name returns the command's name: the first word in the usage line.
func (c *Command) Name() string {
	name := c.Usage
	i := strings.Index(name, " ")
	if i >= 0 {
		name = name[:i]
	}
	return name
}

func (c *Command) usage() {
	fmt.Fprintf(os.Stderr, "usage: %s\n\n", c.Usage)
	fmt.Fprintf(os.Stderr, "%s\n", c.Help)
}

// Commands lists the available commands and help topics.
// The order here is the order in which they are printed by 'go help'.
var commands []*Command

func init() {
	commands = []*Command{
		cmdVersion,
		cmdHelp,
		cmdDoc,
		// cmdRecord,
		cmdList,
		// cmdQuick,
		cmdRun,
		cmdExec,
		// cmdWarmup,
		// cmdDebug,
		// cmdBench,
		// cmdMonitor,
		cmdFingerprint,
		cmdReconstruct,
		// cmdPerf,
	}
}

// usage prints usage information.
func usage() {
	formatedCmdList := ""

	for _, cmd := range commands {
		formatedCmdList += fmt.Sprintf("    %-12s %s\n",
			cmd.Name(), cmd.Description)
	}

	fmt.Printf(`Ht is a tool to generate http request and test the response.

Usage:

    ht <command> [flags...] <suite>...

The commands are:
%s
Run  ht help <command> to display the usage of <command>.

Tests IDs have the following format <suite>.<type><test> with <suite> and
<test> the sequential numbers of the suite and the test inside the suite.
Type is either empty, "u" for setUp test or "d" for tearDown tests. <test>
maybe a single number like "3" or a range like "3-7".
`, formatedCmdList)
}

func main() {
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		usage()
		os.Exit(9)
	}

	for _, cmd := range commands {
		if cmd.Name() != args[0] {
			continue
		}

		cmd.Flag.Usage = func() { cmd.usage() }
		err := cmd.Flag.Parse(args[1:])
		if err != nil {
			os.Exit(9)
		}
		fillVariablesFlagFrom(variablesFile)
		args = cmd.Flag.Args()
		switch {
		case cmd.RunSuites != nil:
			suites := loadSuites(args)
			cmd.RunSuites(cmd, suites)
		case cmd.RunTests != nil:
			tests := loadTests(args)
			cmd.RunTests(cmd, tests)
		default:
			cmd.RunArgs(cmd, args)
		}
		return
	}

	fmt.Fprintf(os.Stderr, "ht: unknown subcommand %q\nRun 'ht help' for usage.\n",
		args[0])
	os.Exit(9)
}

func loadSuites(args []string) []*suite.RawSuite {
	var suites []*suite.RawSuite

	// Handle -only and -skip flags.
	// only, skip := splitTestIDs(onlyFlag), splitTestIDs(skipFlag)

	// Input and setup suites from command line arguments.
	for _, arg := range args {
		s, err := suite.LoadRawSuite(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot read suite %q: %s\n", arg, err)
			os.Exit(8)
		}
		// for varName, varVal := range variablesFlag {
		// 	suite.Variables[varName] = varVal
		// }
		err = s.Validate(variablesFlag)
		if err != nil {
			fmt.Printf("%#v\n", err)
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(8)
		}
		// setVerbosity(s)
		suites = append(suites, s)
	}

	/*
		// Disable tests based on the -only and -skip flags.
		for sNo, s := range suites {
			for tNo, test := range s.Tests {
				shouldRun(test, fmt.Sprintf("%d.%d", sNo+1, tNo+1), only, skip)
			}
		}
	*/
	return suites
}

/*
// set (-verbosity) or increase (-v ... -vvvv) test verbosities of s.
func setVerbosity(s *suite.RawSuite) {
	for i := range s.Tests {
		if verbosity != -99 {
			s.Tests[i].Verbosity = verbosity
		} else if vvvv {
			s.Tests[i].Verbosity += 4
		} else if vvv {
			s.Tests[i].Verbosity += 3
		} else if vv {
			s.Tests[i].Verbosity += 2
		} else if v {
			s - Tests[i].Verbosity += 1
		}
	}
}
*/
// loadTests loads single Tests and combines them into an artificial
// Suite, ready for execution. Unrolling happens, but only the first
// unrolled test gets included into the suite.
func loadTests(args []string) []*suite.RawTest {
	tt := []*suite.RawTest{}
	// Input and setup tests from command line arguments.
	for _, arg := range args {
		test, err := suite.LoadRawTest(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot read test %q: %s\n", arg, err)
			os.Exit(8)
		}
		tt = append(tt, test)
	}

	return tt
}

// add current working direcory to end of include path slice if not already
// there.
func addCWD(i *cmdlIncl) {
	for _, p := range *i {
		if p == "." {
			return
		}
	}
	*i = append(*i, ".")
}

// fillVariablesFlagFrom reads in the file variablesFile and sets the
// jet unset variables. This means that the resulting variable/values in
// variablesFlag looks like the variablesFile was loaded first and the
// -D flags overwrite the ones loaded from file.
func fillVariablesFlagFrom(variablesFile string) {
	if variablesFile == "" {
		return
	}
	data, err := ioutil.ReadFile(variablesFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read variable file %q: %s\n", variablesFile, err)
		os.Exit(8)
	}
	v := map[string]string{}
	err = json5.Unmarshal(data, &v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot unmarshal variable file %q: %s\n", variablesFile, err)
		os.Exit(8)
	}
	for n, k := range v {
		if _, ok := variablesFlag[n]; !ok {
			variablesFlag[n] = k
		}
	}
}
