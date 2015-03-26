// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"log"
	"time"

	"github.com/pinterest/bender"
	"github.com/vdobler/ht/hist"
)

// makeTestChannel returns a channel on which the non-disabled main
// tests of the given suites are sent. The test from the given suites are
// interweaved. All given suites must have at least one non-disabled
// main test.
//
// At most count tests are sent for at most the given duration.
func makeTestChannel(suites []*Suite, count int, duration time.Duration) chan interface{} {
	rc := make(chan interface{})
	go func() {
		n := 0
		s := 0
		idx := make([]int, len(suites))
		start := time.Now()
	loop:
		for {
			suite := suites[s%len(suites)]
			for {
				i := idx[s]
				test := suite.Tests[i%len(suite.Tests)]
				idx[s]++
				if test.Poll.Max < 0 {
					continue // Test is disabled.
				}
				err := test.prepare(suite.Variables)
				if err != nil {
					log.Printf("Failed to prepare test %q of suite %q: %s", err)
					continue
				}
				rc <- test
				n++
				if n >= count || time.Since(start) > duration {
					break loop
				}
			}
			s++
		}
		close(rc)
	}()
	return rc
}

// LoadTestOptions controls details of a load test.
type LoadTestOptions struct {
	// Type determines wether a "throughput" or "concurrency" test is done.
	Type string

	// Count many request are made during the load test.
	Count int

	// Timout determines how long a test may run: The load test is
	// terminated if timeout is exeded, even if not Count many requests
	// heve been made.
	Timeout time.Duration

	// Rate of requests in [request/sec].
	Rate float64

	// Uniform changes to uniform (equaly spaced) distribution of
	// requests. False uses an exponential distribution.
	Uniform bool
}

// LoadTest will perform a load test of the main tests of suites, the details
// of the load test is controlled by opts.
// Errors are reported if any suite's Setup failed.
func LoadTest(suites []*Suite, opts LoadTestOptions) ([]TestResult, error) {
	if opts.Type != "throughput" && opts.Type != "concurrency" {
		return nil, fmt.Errorf("Unknown load tests type %q", opts.Type)
	}
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Minute
	}

	// Setup
	for i, s := range suites {
		result := s.ExecuteSetup()
		if result.Status != Pass {
			return nil, fmt.Errorf("Setup of suite %d failed: %s", i, result.Error)
		}
	}

	log.Printf("Running load testwith %+v", opts)

	// rc provides a stream of prepared test taken from suites.
	rc := makeTestChannel(suites, opts.Count, opts.Timeout)

	executor := func(now int64, r interface{}) (interface{}, error) {
		t := r.(*Test)
		result := TestResult{
			CheckResults: make([]CheckResult, len(t.Checks)),
		}
		t.execute(&result)
		// TODO: result.Response = nil?
		if result.Status == Pass {
			return result, nil
		}
		return result, result.Error
	}

	recorder := make(chan interface{}, 128)

	if opts.Type == "throughput" {
		var ig bender.IntervalGenerator
		if opts.Uniform {
			ig = bender.UniformIntervalGenerator(opts.Rate)
		} else {
			ig = bender.ExponentialIntervalGenerator(opts.Rate)
		}
		bender.LoadTestThroughput(ig, rc, executor, recorder)
	} else {
		sem := bender.NewWorkerSemaphore()
		go func() { sem.Signal(int(opts.Rate)) }()
		bender.LoadTestConcurrency(sem, rc, executor, recorder)
	}

	allResults := make([]TestResult, 0, opts.Count)

	resultRec := func(msg interface{}) {
		if ere, ok := msg.(*bender.EndRequestEvent); ok {
			if result, ok := ere.Response.(TestResult); ok {
				allResults = append(allResults, result)
			}
		}
	}

	bender.Record(recorder, resultRec)

	// Teardown
	for _, s := range suites {
		s.ExecuteTeardown()
	}

	return allResults, nil
}

// LoadtestResult captures aggregated values of a load test.
type LoadtestResult struct {
	Started  time.Time
	Total    int
	Passed   int
	Failed   int
	Errored  int
	Skipped  int
	Bogus    int
	PassHist *hist.LogHist
	FailHist *hist.LogHist
	BothHist *hist.LogHist
}

func (r LoadtestResult) String() string {
	s := fmt.Sprintf("Total   Passed  Failed  Errored Skipped Bogus\n")
	s += fmt.Sprintf("%-7d %-7d %-7d %-7d %-7d %-7d \n", r.Total, r.Passed,
		r.Failed, r.Errored, r.Skipped, r.Bogus)
	s += fmt.Sprintf("Passed  Failed  Both    (average response time)\n")
	s += fmt.Sprintf("%-7d %-7d %-7d [ms]\n", r.PassHist.Average(),
		r.FailHist.Average(), r.BothHist.Average())

	ps := []float64{0, 0.25, 0.50, 0.75, 0.80, 0.85, 0.90, 0.95, 0.97,
		0.98, 0.99, 0.995, 0.999, 1}
	cps := make([]float64, len(ps))
	for i, p := range ps {
		cps[i] = 100 * p
	}

	s += fmt.Sprintf("Percentil %4.1f\n", cps)
	s += fmt.Sprintf("Passed    %4d  [ms]\n", r.PassHist.Percentils(ps))
	s += fmt.Sprintf("Failed    %4d  [ms]\n", r.FailHist.Percentils(ps))

	return s
}

// AnalyseLoadtest computes aggregate statistics of the given results.
func AnalyseLoadtest(results []TestResult) LoadtestResult {
	result := LoadtestResult{}

	var max, maxp, maxf, maxe time.Duration
	for _, r := range results {
		if r.Duration > max {
			max = r.Duration
		}
		switch r.Status {
		case Pass:
			result.Passed++
			if r.Duration > maxp {
				maxp = r.Duration
			}
		case Fail:
			result.Failed++
			if r.Duration > maxf {
				maxf = r.Duration
			}
		case Error:
			result.Errored++
			if r.Duration > maxe {
				maxe = r.Duration
			}
		case Skipped:
			result.Skipped++
		case Bogus:
			result.Bogus++
		}
	}
	result.Total = result.Passed + result.Failed + result.Errored + result.Skipped + result.Bogus

	result.PassHist = hist.NewLogHist(7, int(maxp/time.Millisecond))
	result.FailHist = hist.NewLogHist(7, int(maxf/time.Millisecond))
	result.BothHist = hist.NewLogHist(7, int(max/time.Millisecond))
	for _, r := range results {
		switch r.Status {
		case Pass:
			result.PassHist.Add(int(r.Duration / time.Millisecond))
			result.BothHist.Add(int(r.Duration / time.Millisecond))
		case Fail:
			result.FailHist.Add(int(r.Duration / time.Millisecond))
			result.BothHist.Add(int(r.Duration / time.Millisecond))
		}
	}

	return result
}

/*
func NewHistogramRecorder(h *LogHist) bender.Recorder {
	return func(msg interface{}) {
		switch msg := msg.(type) {
		case *bender.EndRequestEvent:
			elapsed := int((msg.End - msg.Start) / int64(time.Millisecond))
			h.Add(elapsed)
		}
	}
}
*/
