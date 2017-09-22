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
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/internal/hjson"
	"github.com/vdobler/ht/populate"
	"github.com/vdobler/ht/suite"

	_ "github.com/go-sql-driver/mysql"
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
	eol := strings.Index(c.Help, "\n")
	fmt.Fprintf(os.Stderr, "%s\n\n", c.Help[:eol])
	fmt.Fprintf(os.Stderr, "Usage:\n\n")
	fmt.Fprintf(os.Stderr, "    ht %s\n", c.Usage)
	fmt.Fprintf(os.Stderr, "%s\n", c.Help[eol+1:])
}

// Commands lists the available commands and help topics.
// The order here is the order in which they are printed by 'go help'.
var commands []*Command

func init() {
	commands = []*Command{
		cmdVersion,
		cmdHelp,
		cmdDoc,
		cmdExample,
		cmdRecord,
		cmdList,
		cmdQuick,
		cmdRun,
		cmdExec,
		// cmdBench,
		// cmdMonitor,
		cmdFingerprint,
		cmdReconstruct,
		cmdLoad,
		cmdStat,
		cmdMock,
		cmdGUI,
	}
}

// usage prints usage information.
func usage() {
	formatedCmdList := ""

	for _, cmd := range commands {
		formatedCmdList += fmt.Sprintf("    %-12s %s\n",
			cmd.Name(), cmd.Description)
	}

	fmt.Printf(`ht is a tool to generate HTTP request and test the response.

Usage:

    ht <command> [flags...] <args depending on command>...

The commands are:
%s
Run 'ht help <command>' to display the usage of <command> and
run 'ht help help' to see what other help you can get.
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
			if err == flag.ErrHelp {
				cmd.Flag.PrintDefaults()
				os.Exit(0)
			}
			os.Exit(9)
		}
		fillVariablesFlagFrom(variablesFile)
		args = cmd.Flag.Args()
		switch {
		case cmd.RunSuites != nil:
			suites := loadSuites(args)
			cmd.RunSuites(cmd, suites)
		case cmd.RunTests != nil:
			tests, err := loadTests(args)
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(8)
			}
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

// For any entry in args of the form <dirname>/... look for any *.suite file
// below <dirname> and expand the arglist.
func expandTrippleDots(args []string) []string {
	expanded := []string{}

	// walking the directory, capturing all *.suite falls while swallowing
	// all errors.
	walk := func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && len(info.Name()) > 6 && strings.HasSuffix(path, ".suite") {
			expanded = append(expanded, path)
		}
		return nil
	}

	for _, arg := range args {
		if !strings.HasSuffix(arg, "/...") {
			expanded = append(expanded, arg)
			continue
		}
		arg := arg[:len(arg)-4] // strip /...
		finfo, err := os.Stat(arg)
		if err != nil || !finfo.IsDir() {
			// Not a directory? Don't process and fail later.
			expanded = append(expanded, arg)
			continue
		}
		filepath.Walk(arg, walk)
	}
	return expanded
}

func filesystemFor(arg string) (suite.FileSystem, string) {
	i := strings.Index(arg, "@")
	if i == -1 {
		return nil, arg // Not an archive, use real file system from OS.
	}

	blob, err := ioutil.ReadFile(arg[i+1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot load %q: %s\n", arg[i+1:], err)
		os.Exit(9)
	}
	fs, err := suite.NewFileSystem(string(blob))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot load %q: %s\n", arg[i+1:], err)
		os.Exit(9)
	}
	arg = arg[:i]
	return fs, arg
}

func loadSuites(args []string) []*suite.RawSuite {
	args = expandTrippleDots(args)

	var suites []*suite.RawSuite

	// Handle -only and -skip flags.
	only, skip := splitTestIDs(onlyFlag), splitTestIDs(skipFlag)

	// Input and setup suites from command line arguments.
	exit := false
	for _, arg := range args {
		// Process arguments of the form <name>@<archive>.
		fs, arg := filesystemFor(arg)
		s, err := suite.LoadRawSuite(arg, fs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot read suite %q: %s\n", arg, err)
			exit = true
			continue
		}
		// for varName, varVal := range variablesFlag {
		// 	suite.Variables[varName] = varVal
		// }
		err = s.Validate(variablesFlag)
		if err != nil {
			if el, ok := err.(ht.ErrorList); ok {
				for _, msg := range el.AsStrings() {
					fmt.Fprintln(os.Stderr, msg)
				}
			} else {
				fmt.Fprintln(os.Stderr, err.Error())
			}
			exit = true
		}
		// setVerbosity(s)
		suites = append(suites, s)
	}
	if exit {
		os.Exit(8)
	}

	// Merge only into skip.
	if len(only) > 0 {
		for sNo := range suites {
			for tNo := range suites[sNo].RawTests() {
				id := fmt.Sprintf("%d.%d", sNo+1, tNo+1)
				if !only[id] {
					skip[id] = true
				}
			}
		}
	}

	// Disable tests based on the -only and -skip flags.
	for sNo := range suites {
		for tNo, rt := range suites[sNo].RawTests() {
			id := fmt.Sprintf("%d.%d", sNo+1, tNo+1)
			if skip[id] {
				rt.Disable()
				fmt.Printf("Skipping test %s %q\n", id, rt.Name)
			}
		}
	}

	// Propagate verbosity from command line to suite/test.
	for _, s := range suites {
		setVerbosity(s)
	}

	return suites
}

func splitTestIDs(f string) map[string]bool {
	ids := make(map[string]bool)
	if len(f) == 0 {
		return ids
	}
	fp := strings.Split(f, ",")
	for _, x := range fp {
		xp := strings.SplitN(x, ".", 2)
		s, t := "1", xp[0]
		if len(xp) == 2 {
			s, t = xp[0], xp[1]
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
			id := fmt.Sprintf("%d.%d", sNo, tNo)
			ids[id] = true
		}
	}
	return ids
}

func mustAtoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
		os.Exit(9)
	}
	return n
}

// set (-verbosity) or increase (-v ... -vvvv) test verbosities of s.
func setVerbosity(rs *suite.RawSuite) {
	if verbosity != -99 {
		rs.Verbosity = verbosity
	} else if vvvv {
		rs.Verbosity += 4
	} else if vvv {
		rs.Verbosity += 3
	} else if vv {
		rs.Verbosity += 2
	} else if v {
		rs.Verbosity += 1
	}
}

// loadTests loads single Tests and combines them into an artificial
// Suite, ready for execution. Unrolling happens, but only the first
// unrolled test gets included into the suite.
func loadTests(args []string) ([]*suite.RawTest, error) {
	tt := []*suite.RawTest{}
	// Input and setup tests from command line arguments.
	for _, arg := range args {
		fs, arg := filesystemFor(arg)
		test, err := suite.LoadRawTest(arg, fs)
		if err != nil {
			return nil, fmt.Errorf("Cannot read test %q: %s\n", arg, err)
		}
		tt = append(tt, test)
	}

	return tt, nil
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
	v := map[string]interface{}{}
	err = hjson.Unmarshal(data, &v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot unmarshal variable file %q: %s\n", variablesFile, err)
		os.Exit(8)
	}
	vv := map[string]string{}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Malformed variable file %q: %s\n", variablesFile, err)
		os.Exit(8)
	}

	err = populate.Strict(&vv, v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Malformed variable file %q: %s\n", variablesFile, err)
		os.Exit(8)
	}

	for n, k := range vv {
		if _, ok := variablesFlag[n]; !ok {
			variablesFlag[n] = k
		}
	}
}
