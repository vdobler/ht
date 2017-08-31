// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/mock"
	"github.com/vdobler/ht/scope"
	"github.com/vdobler/ht/suite"
)

var cmdMock = &Command{
	RunArgs:     runMock,
	Usage:       "mock <mock>...",
	Description: "run a mock server",
	Flag:        flag.NewFlagSet("stat", flag.ContinueOnError),
	Help: `Mock starts a HTTP server providing the given mocks.
`,
}

var (
	certFile string
	keyFile  string
)

func init() {
	addVarsFlags(cmdMock.Flag)
	cmdMock.Flag.StringVar(&certFile, "cert", "",
		"load server certificate for https mocks from `file`")
	cmdMock.Flag.StringVar(&keyFile, "key", "",
		"load private key for https mocks from `file`")
}

func runMock(cmd *Command, args []string) {
	// Sanity check.
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "No mock given")
		fmt.Fprintf(os.Stderr, "Usage: %s\n", cmd.Usage)
		os.Exit(9)
	}

	monitor := make(chan *ht.Test)

	mocks := []*mock.Mock{}
	for _, arg := range args {
		raw, err := suite.LoadRawMock(arg, nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(8)
		}
		mockScope := scope.New(scope.Variables(variablesFlag), raw.Variables, false)
		mockScope["MOCK_DIR"] = raw.Dirname()
		mockScope["MOCK_NAME"] = raw.Basename()
		m, err := raw.ToMock(mockScope, false)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(8)
		}

		m.Monitor = monitor
		mocks = append(mocks, m)
	}

	logger := log.New(os.Stdout, "", 0)
	nfh := func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Mock not found: 404 for %s %s\n", r.Method, r.URL)
		fmt.Println("===========================================================")
		http.Error(w, "Not found", 404)
	}
	_, err := mock.Serve(mocks, http.HandlerFunc(nfh), logger, certFile, keyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Problems staring server: %s\n", err)
		os.Exit(9)
	}

	for {
		report := <-monitor
		fmt.Println(mock.PrintReport(report))
	}
}
