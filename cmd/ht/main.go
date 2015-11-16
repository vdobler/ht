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
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/internal/json5"
)

// A Command is one of the subcommands of ht.
type Command struct {
	RunSuites func(cmd *Command, suites []*ht.Suite)
	RunTests  func(cmd *Command, tests []*ht.Test)
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
var commands = []*Command{
	cmdVersion,
	cmdList,
	cmdRun,
	cmdExec,
	cmdWarmup,
	cmdBench,
	cmdMonitor,
	cmdFingerprint,
	// cmdPerf,
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

	if args[0] == "help" {
		help(args[1:])
		return
	}

	var suites []*ht.Suite
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
			suites = loadSuites(args)
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

// The help command.
func help(args []string) {
	if len(args) == 0 {
		usage()
		os.Exit(9)
	}
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: ht help <command>\n\nToo many arguments given.\n")
		os.Exit(9)
	}

	arg := args[0]

	for _, cmd := range commands {
		if cmd.Name() == arg {
			fmt.Printf(`Usage:

    ht %s
%s
Flags:
`, cmd.Usage, cmd.Help)
			cmd.Flag.PrintDefaults()
			os.Exit(0)
		}
	}

	fmt.Fprintf(os.Stderr, "Unknown help topic %#q.  Run 'ht help'.\n", arg)
	os.Exit(9) // failed at 'go help cmd'
}

func loadSuites(args []string) []*ht.Suite {
	var suites []*ht.Suite

	logger := log.New(os.Stdout, "", log.LstdFlags)

	// Handle -only and -skip flags.
	only, skip := splitTestIDs(onlyFlag), splitTestIDs(skipFlag)

	// Input and setup suites from command line arguments.
	for _, s := range args {
		suite, err := ht.LoadSuite(s)
		if err != nil {
			log.Fatalf("Cannot read suite %q: %s", s, err)
		}
		for varName, varVal := range variablesFlag {
			suite.Variables[varName] = varVal
		}
		suite.Log = logger
		err = suite.Prepare()
		if err != nil {
			log.Fatal(err.Error())
		}
		if verbosity != -99 {
			for i := range suite.Setup {
				suite.Setup[i].Verbosity = verbosity
			}
			for i := range suite.Tests {
				suite.Tests[i].Verbosity = verbosity
			}
			for i := range suite.Teardown {
				suite.Teardown[i].Verbosity = verbosity
			}
		}
		suites = append(suites, suite)
	}

	// Disable tests based on the -only and -skip flags.
	for sNo, suite := range suites {
		for tNo, test := range suite.Setup {
			shouldRun(test, fmt.Sprintf("%d.U%d", sNo+1, tNo+1), only, skip)
		}
		for tNo, test := range suite.Tests {
			shouldRun(test, fmt.Sprintf("%d.%d", sNo+1, tNo+1), only, skip)
		}
		for tNo, test := range suite.Teardown {
			shouldRun(test, fmt.Sprintf("%d.D%d", sNo+1, tNo+1), only, skip)
		}
	}

	return suites
}

// loadTests loads single Tests and combines them into an artificial
// Suite, ready for execution. Unrolling happens, but only the first
// unrolled test gets included into the suite.
func loadTests(args []string) []*ht.Test {
	tt := []*ht.Test{}
	// Input and setup tests from command line arguments.
	for _, t := range args {
		tests, err := ht.LoadTest(t)
		if err != nil {
			log.Fatalf("Cannot read test %q: %s", t, err)
		}
		tt = append(tt, tests[0])
	}

	return tt
}

// shouldRun disables t if needed.
func shouldRun(t *ht.Test, id string, only, skip map[string]struct{}) {
	if _, ok := skip[id]; ok {
		t.Poll.Max = -1
		log.Printf("Skipping test %s %q", id, t.Name)
		return
	}
	if _, ok := only[id]; !ok && len(only) > 0 {
		t.Poll.Max = -1
		log.Printf("Not running test %s %q", id, t.Name)
		return
	}
}

func splitTestIDs(f string) (ids map[string]struct{}) {
	ids = make(map[string]struct{})
	if len(f) == 0 {
		return
	}
	fp := strings.Split(f, ",")
	for _, x := range fp {
		xp := strings.SplitN(x, ".", 2)
		s, t := "1", xp[0]
		if len(xp) == 2 {
			s, t = xp[0], xp[1]
		}
		typ := ""
		switch t[0] {
		case 'U', 'u', 'S', 's':
			typ = "U"
			t = t[1:]
		case 'D', 'd', 'T', 't':
			typ = "D"
			t = t[1:]
		default:
			typ = ""
		}

		sNo := mustAtoi(s)
		beg, end := 1, 99
		if i := strings.Index(t, "-"); i > -1 {
			if i > 0 {
				beg = mustAtoi(t[:i])
			}
			if i < len(t)-1 {
				end = mustAtoi(t[i+1:])
			}
		} else {
			beg = mustAtoi(t)
			end = beg
		}
		for tNo := beg; tNo <= end; tNo++ {
			id := fmt.Sprintf("%d.%s%d", sNo, typ, tNo)
			ids[id] = struct{}{}
		}
	}
	return ids
}

func mustAtoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		log.Fatalf("%s", err.Error())
	}
	return n
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
// jet unset variables. This meass that the resulting variable/values in
// variablesFlag looks like the variablesFile was loaded first and the
// -D flags overwrite the ones loaded from file.
func fillVariablesFlagFrom(variablesFile string) {
	if variablesFile == "" {
		return
	}
	data, err := ioutil.ReadFile(variablesFile)
	if err != nil {
		log.Fatalf("Cannot read variable file %q: %s", variablesFile, err)
	}
	v := map[string]string{}
	err = json5.Unmarshal(data, &v)
	if err != nil {
		log.Fatalf("Cannot unmarshal variable file %q: %s", variablesFile, err)
	}
	for n, k := range v {
		if _, ok := variablesFlag[n]; !ok {
			variablesFlag[n] = k
		}
	}
}
