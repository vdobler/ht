// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/vdobler/ht/ht"
)

var cmdDebug = &Command{
	RunArgs:     runDebug,
	Usage:       "debug <jsontest>",
	Description: "debug a test given on the command line",
	Flag:        flag.NewFlagSet("debug", flag.ContinueOnError),
	Help: `
Debug executes a single test given directly on the command line in
JSON5 format without the need to save the test to a file.
Technically the test is saved to a temporary file in the current
folder and thus may access mixins via the 'BasedOn' mechanism."
	`,
}

func init() {
	addVariablesFlag(cmdDebug.Flag)
	addDfileFlag(cmdDebug.Flag)
	addVerbosityFlag(cmdDebug.Flag)
	addSeedFlag(cmdDebug.Flag)
	addOutputFlag(cmdDebug.Flag)
	addSkiptlsverifyFlag(cmdDebug.Flag)
}

func runDebug(cmd *Command, args []string) {
	if len(args) != 1 {
		log.Println("Not exactly one argument (test) given.")
		os.Exit(9)
	}

	// Write argument to temporary file.
	tmpfile, err := ioutil.TempFile(".", "debug")
	if err != nil {
		log.Panic(err)
	}
	_, err = io.WriteString(tmpfile, args[0])
	if err != nil {
		log.Panic(err)
	}
	err = tmpfile.Close()
	if err != nil {
		log.Panic(err)
	}

	// Read test from temporary file and handle it over to the run command.
	tests, err := ht.LoadTest(tmpfile.Name())
	if err != nil {
		log.Panic(err)
	}
	runRun(cmd, tests[:1])
	os.Remove(tmpfile.Name())
}
