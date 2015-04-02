// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/vdobler/ht/check"
)

func TestThroughput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(echoHandler))
	defer ts.Close()
	// lg := log.New(os.Stdout, "ht ", log.LstdFlags)

	test := &Test{
		Name: "Sleep {{SMIN}}-{{SMAX}}",
		Request: Request{
			Method: "GET",
			URL:    ts.URL + "/",
			Params: url.Values{
				"smin": []string{"{{SMIN}}"},
				"smax": []string{"{{SMAX}}"},
				"bad":  {"5"},
			},
			FollowRedirects: false,
		},
		Checks: []check.Check{
			check.StatusCode{200},
		},
		Timeout: 800 * time.Millisecond,
	}
	unroll := map[string][]string{
		"SMIN": []string{"1", "40", "200", "1", "40"},
		"SMAX": []string{"30", "90", "300", "30", "90"},
	}
	tests, err := Repeat(test, 5, unroll)

	suite := &Suite{
		Name:  "Throughput",
		Tests: tests,
		// Log: lg,
	}

	err = suite.Prepare()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	options := LoadTestOptions{
		Type:    "throughput",
		Rate:    100,
		Count:   1000,
		Uniform: true,
	}
	results, err := LoadTest([]*Suite{suite}, options)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	fmt.Printf("Throughput Test:\n")
	AnalyseLoadtest(results)

	options.Type = "concurrency"
	options.Count = 400

	results, err = LoadTest([]*Suite{suite}, options)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	fmt.Printf("Concurrency Test:\n")
	ltr := AnalyseLoadtest(results)
	fmt.Printf("Loadtest Result:\n%s\n", ltr)
}
