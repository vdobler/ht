// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"math/rand"
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

	err = suite.Compile()
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
	AnalyseLoadtest(results)
}

func TestLogHist(t *testing.T) {
	if testing.Short() {
		t.Skip("LogHist is skipped in short mode.")
	}
	tests := []struct{ v, b int }{
		{0, 0}, {1, 1}, {19, 19}, {63, 63}, {64, 64}, {65, 64}, {66, 65},
		{117, 90}, {118, 91}, {127, 95}, {128, 96}, {129, 96}, {130, 96},
		{131, 96}, {132, 97}, {255, 127}, {256, 128}, {263, 128}, {264, 129},
		{2047, 223}, {2048, 224},
	}

	h := NewLogHist(3000)

	for _, tc := range tests {
		if got := h.bucket(tc.v); got != tc.b {
			t.Errorf("bucket(%d)=%d, want %d", tc.v, got, tc.b)
		}
	}

	peak := []float64{500, 1500, 2500}
	for i := 0; i < 50000; i++ {
		p := rand.Intn(3)
		r := int(rand.NormFloat64()*100 + peak[p])
		if r < 0 {
			r = 0
		} else if r >= 3000 {
			r = 2999
		}
		h.Add(r)
	}

	fmt.Printf("%d %d %d %d %d %d %d %d %d %d %d %d\n", h.Percentil(0),
		h.Percentil(0.10), h.Percentil(0.25),
		h.Percentil(0.50), h.Percentil(0.75), h.Percentil(0.90),
		h.Percentil(0.95), h.Percentil(0.98), h.Percentil(0.99),
		h.Percentil(0.998), h.Percentil(0.999), h.Percentil(1))

	h.dump()
	println()
	h.dumplin()
	h.PercentilPlot("xxx.png")
}
