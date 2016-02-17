// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func setupPerfSuites(t *testing.T) ([]*Suite, *httptest.Server) {
	ts := httptest.NewServer(http.HandlerFunc(echoHandler))
	// lg := log.New(os.Stdout, "ht ", log.LstdFlags)

	test := &Test{
		Name: "Sleep {{SMIN}}-{{SMAX}}",
		Request: Request{
			Method: "GET",
			URL:    ts.URL + "/",
			Params: URLValues{
				"smin": []string{"{{SMIN}}"},
				"smax": []string{"{{SMAX}}"},
				"bad":  {"5"},
			},
			FollowRedirects: false,
		},
		Checks: []Check{
			StatusCode{200},
		},
		Timeout:   Duration(400 * time.Millisecond),
		Verbosity: 0,
	}
	if testing.Verbose() {
		test.Verbosity = 1
	}

	unroll := map[string][]string{
		"SMIN": {"1", "40", "200", "1", "40"},
		"SMAX": {"30", "90", "400", "30", "90"},
	}
	tests, err := Repeat(test, 5, unroll)

	suite := &Suite{
		Name:  "Throughput",
		Tests: tests,
		Log:   log.New(os.Stdout, "", 0),
	}

	err = suite.Prepare()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	return []*Suite{suite}, ts
}

func TestThroughputLoadTest(t *testing.T) {
	suites, ts := setupPerfSuites(t)
	defer ts.Close()

	options := LoadTestOptions{
		Type:    "throughput",
		Rate:    100,
		Count:   1000,
		Uniform: true,
	}
	if testing.Short() {
		options.Count = 200
	}
	results, err := PerformanceLoadTest(suites, options)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if testing.Verbose() {
		fmt.Printf("Throughput Test:\n")
		ltr := AnalyseLoadtest(results)
		fmt.Println(ltr)
	}
}

func TestConcurrencyLoadTest(t *testing.T) {
	suites, ts := setupPerfSuites(t)
	defer ts.Close()

	options := LoadTestOptions{
		Type:    "concurrency",
		Rate:    100,
		Count:   1000,
		Uniform: true,
	}
	if testing.Short() {
		options.Count = 200
	}

	results, err := PerformanceLoadTest(suites, options)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	if testing.Verbose() {
		fmt.Printf("Concurrency Test:\n")
		ltr := AnalyseLoadtest(results)
		fmt.Println(ltr)
	}
}
