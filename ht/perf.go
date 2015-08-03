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

// -------------------------------------------------------------------------
//  Load Testing

// LoadTestOptions controls details of a load test.
type LoadTestOptions struct {
	// Type determines wether a "throughput" or "concurrency" test is done.
	Type string

	// Count many request are made during the load test in total.
	Count int

	// Timout determines how long a test may run: The load test is
	// terminated if timeout is exeded, even if not Count many requests
	// heve been made yet.
	Timeout time.Duration

	// Rate is the average rate of requests in [request/sec].
	Rate float64

	// Uniform changes to uniform (equaly spaced) distribution of
	// requests. False uses an exponential distribution.
	Uniform bool
}

// PerformanceLoadTest will perform a load test of the main tests of suites, the details
// of the load test is controlled by opts.
// Errors are reported if any suite's Setup failed.
func PerformanceLoadTest(suites []*Suite, opts LoadTestOptions) ([]Test, error) {
	if opts.Type != "throughput" && opts.Type != "concurrency" {
		return nil, fmt.Errorf("Unknown load tests type %q", opts.Type)
	}
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Minute
	}

	// Setup
	for i, s := range suites {
		s.Status, s.Error = NotRun, nil
		s.ExecuteSetup()
		if s.Status > Pass {
			return nil, fmt.Errorf("Setup of suite %d failed: %s", i, s.Error)
		}
	}

	log.Printf("Running load test with %+v", opts)

	// rc provides a stream of prepared test taken from suites.
	rc := makeTestChannel(suites, opts.Count, opts.Timeout)

	executor := func(now int64, r interface{}) (interface{}, error) {
		t := r.(Test)
		t.CheckResults = make([]CheckResult, len(t.checks)) // TODO: move elsewhere
		t.execute()
		// TODO: result.Response = nil?
		if t.Status == Pass {
			return t, nil
		}
		return t, t.Error
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

	allResults := make([]Test, 0, opts.Count)

	resultRec := func(msg interface{}) {
		if ere, ok := msg.(*bender.EndRequestEvent); ok {
			if result, ok := ere.Response.(Test); ok {
				allResults = append(allResults, result)
			}
		}
	}

	bender.Record(recorder, resultRec)

	// Teardown
	for _, s := range suites {
		s.ExecuteTeardown()
	}

	// for i, r := range allResults {
	// fmt.Printf("%d. %s %s %s %s\n", i, r.Status, r.Response.Duration, r.Name, r.Error)
	// }

	return allResults, nil
}

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
				rc <- *test
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

// String formats r in a useful way.
func (r LoadtestResult) String() string {
	s := fmt.Sprintf("Total   Passed  Failed  Errored Skipped Bogus\n")
	s += fmt.Sprintf("%-7d %-7d %-7d %-7d %-7d %-7d \n", r.Total, r.Passed,
		r.Failed, r.Errored, r.Skipped, r.Bogus)
	s += fmt.Sprintf("Passed  Failed  Both    (average response time)\n")
	s += fmt.Sprintf("%-7d %-7d %-7d [ms]\n", r.PassHist.Average(),
		r.FailHist.Average(), r.BothHist.Average())

	ps := []float64{0, 0.25, 0.50, 0.75, 0.90, 0.95, 0.97,
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
func AnalyseLoadtest(results []Test) LoadtestResult {
	result := LoadtestResult{}

	var max, maxp, maxf, maxe Duration
	for _, r := range results {
		if r.Response.Duration > max {
			max = r.Response.Duration
		}
		switch r.Status {
		case Pass:
			result.Passed++
			if r.Response.Duration > maxp {
				maxp = r.Response.Duration
			}
		case Fail:
			result.Failed++
			if r.Response.Duration > maxf {
				maxf = r.Response.Duration
			}
		case Error:
			result.Errored++
			if r.Response.Duration > maxe {
				maxe = r.Response.Duration
			}
		case Skipped:
			result.Skipped++
		case Bogus:
			result.Bogus++
		}
	}
	result.Total = result.Passed + result.Failed + result.Errored + result.Skipped + result.Bogus

	const millisecond = 1e6
	result.PassHist = hist.NewLogHist(7, int(maxp/millisecond))
	result.FailHist = hist.NewLogHist(7, int(maxf/millisecond))
	result.BothHist = hist.NewLogHist(7, int(max/millisecond))
	for _, r := range results {
		switch r.Status {
		case Pass:
			result.PassHist.Add(int(r.Response.Duration / millisecond))
			result.BothHist.Add(int(r.Response.Duration / millisecond))
		case Fail:
			result.FailHist.Add(int(r.Response.Duration / millisecond))
			result.BothHist.Add(int(r.Response.Duration / millisecond))
		}
	}

	return result
}

// -------------------------------------------------------------------------
//  Benchmarking

// Benchmark executes t count many times and reports the outcome.
// Before doing the measurements warmup many request are made and discarded.
// Conc determines the concurrency level. If conc==1 the given pause
// is made between request. A conc > 1 will execute conc many request
// in paralell (without pauses).
// TODO: move this into an BenmarkOptions
func (t *Test) Benchmark(variables map[string]string, warmup int, count int, pause time.Duration, conc int) []Test {
	for n := 0; n < warmup; n++ {
		if n > 0 {
			time.Sleep(pause)
		}
		t.prepare(variables)
		t.executeRequest()
	}

	results := make([]Test, count)
	origPollMax := t.Poll.Max
	t.Poll.Max = 1

	if conc == 1 {
		// One request after the other, nicely spaced.
		for n := 0; n < count; n++ {
			time.Sleep(pause)
			t.Run(variables)
			results[n] = *t
		}
	} else {
		// Start conc request and restart an other once one finishes.
		rc := make(chan Test, conc)
		for i := 0; i < conc; i++ {
			go func() {
				t.Run(variables)
				rc <- *t
			}()
		}
		for j := 0; j < count; j++ {
			results[j] = <-rc
			go func() {
				t.Run(variables)
				rc <- *t
			}()
		}

	}
	t.Poll.Max = origPollMax

	return results
}
