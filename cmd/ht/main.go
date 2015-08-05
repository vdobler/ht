// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// ht generates HTTP requests and checks the received responses.
//
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/vdobler/ht/ht"
)

// A Command is one of the subcommands of ht.
type Command struct {
	// Run the command.
	// The args are the arguments after the command name.
	Run func(cmd *Command, suites []*ht.Suite)

	Usage       string       // must start with command name
	Description string       // short description for ' go help'
	Help        string       // the output of 'ht help <cmd>'
	Flag        flag.FlagSet // the flags for this command
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
	os.Exit(2)
}

// Commands lists the available commands and help topics.
// The order here is the order in which they are printed by 'go help'.
var commands = []*Command{
	cmdList,
	cmdRun,
	cmdExec,
	cmdBench,
	// cmdPerf,
}

func usage() {
	formatedCmdList := ""

	for _, cmd := range commands {
		formatedCmdList += fmt.Sprintf("    %-8s %s\n",
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
	os.Exit(2)
}

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

func main() {
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		usage()
	}

	if args[0] == "help" {
		help(args[1:])
		return
	}
	var suites []*ht.Suite
	for _, cmd := range commands {
		if cmd.Name() == args[0] {
			cmd.Flag.Usage = func() { cmd.usage() }
			cmd.Flag.Parse(args[1:])
			args = cmd.Flag.Args()
			if cmd.Name() == "run" {
				suites = loadTests(args)
			} else {
				suites = loadSuites(args)
			}
			cmd.Run(cmd, suites)
			return
		}
	}

	fmt.Fprintf(os.Stderr, "go: unknown subcommand %q\nRun 'go help' for usage.\n",
		args[0])
	os.Exit(2)
}

// The help command.
func help(args []string) {
	if len(args) == 0 {
		usage() // TODO: this is not a failure
	}
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: ht help <command>\n\nToo many arguments given.\n")
		os.Exit(2)
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
			return
		}
	}

	fmt.Fprintf(os.Stderr, "Unknown help topic %#q.  Run 'ht help'.\n", arg)
	os.Exit(2) // failed at 'go help cmd'
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

// loadTests loads single Test and combines them into an artificial
// Suite, ready for execution. Unrolling happens, but only the first
// unrolled test gets included into the suite.
func loadTests(args []string) []*ht.Suite {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	suite := &ht.Suite{
		Name: "Autogenerated suite for run",
		Log:  logger,
	}

	// Input and setup tests from command line arguments.
	for _, t := range args {
		tests, err := ht.LoadTest(t)
		if err != nil {
			log.Fatalf("Cannot read test %q: %s", t, err)
		}
		suite.Tests = append(suite.Tests, tests[0])
	}

	err := suite.Prepare()
	if err != nil {
		log.Fatal(err.Error())
	}
	if verbosity != -99 {
		for i := range suite.Tests {
			suite.Tests[i].Verbosity = verbosity
		}
	}

	for varName, varVal := range variablesFlag {
		suite.Variables[varName] = varVal
	}

	suites := []*ht.Suite{suite}
	return suites
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
