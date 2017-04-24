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
	"strconv"
	"strings"

	hjson "github.com/hjson/hjson-go"
	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/mock"
	"github.com/vdobler/ht/populate"
	"github.com/vdobler/ht/suite"
)

var cmdMock = &Command{
	RunArgs:     runMock,
	Usage:       "mock <mock>...",
	Description: "mock server",
	Flag:        flag.NewFlagSet("stat", flag.ContinueOnError),
	Help: `
Mock starts a HTTP server providing the given mocks.
`,
}

func init() {
	addVarsFlags(cmdMock.Flag)
	/*
		cmdMock.Flag.StringVar(&output, "output", "throughput.csv",
			"save results to `name`")
		cmdMock.Flag.BoolVar(&logplot, "log", true,
			"show logarithmic scale on plot")
		cmdMock.Flag.IntVar(&plotwidth, "plotwidth", 120,
			"draw plot `num` chars wide")
		cmdMock.Flag.DurationVar(&rampDuration, "ramp", 5*time.Second,
			"ramp duration to ignore while computing average QPS")
	*/
}

func loadMock(name string, replacer *strings.Replacer) (*mock.Mock, error) {
	file, err := suite.LoadFile(name)
	if err != nil {
		return nil, err
	}
	file.Data = replacer.Replace(file.Data)

	var m = &mock.Mock{}

	// TODO: this is a copy of suite.File.decodeStrictTo. Refactor.
	var soup interface{}
	err = hjson.Unmarshal([]byte(file.Data), &soup)
	if err != nil {
		return nil, fmt.Errorf("file %s is not valid hjson: %s", file.Name, err)
	}
	s, ok := soup.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("file %s is not an object (got %T)", file.Name, soup)
	}
	err = populate.Strict(m, s)
	if err != nil {
		return nil, err // better error message here
	}

	return m, nil

}

func runMock(cmd *Command, args []string) {
	// Sanity check.
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "No mock given")
		fmt.Fprintf(os.Stderr, "Usage: %s\n", cmd.Usage)
		os.Exit(9)
	}

	// TODO: refactore with suite.varReplacer
	oldnew := []string{}
	variablesFlag["COUNTER"] = strconv.Itoa(<-suite.GetCounter)
	variablesFlag["RANDOM"] = strconv.Itoa(100000 + suite.RandomIntn(900000))
	for k, v := range variablesFlag {
		oldnew = append(oldnew, "{{"+k+"}}") // TODO: make configurable ??
		oldnew = append(oldnew, v)
	}
	replacer := strings.NewReplacer(oldnew...)

	monitor := make(chan *ht.Test)

	mocks := []*mock.Mock{}
	for _, arg := range args {
		m, err := loadMock(arg, replacer)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(8)
		}
		m.Monitor = monitor
		mocks = append(mocks, m)
	}

	logger := log.New(os.Stdout, "", 0)
	nfh := func(w http.ResponseWriter, r *http.Request) {
		logger.Printf("404 %s %s", r.Method, r.URL)
		http.Error(w, "Not found", 404)
	}
	_, err := mock.Serve(mocks, http.HandlerFunc(nfh), logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Problems staring server: %s\n", err)
		os.Exit(9)
	}

	for {
		report := <-monitor
		fmt.Println()
		report.PrintReport(os.Stdout)
	}
}
