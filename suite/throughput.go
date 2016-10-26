// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Throughput Load-Testing
//
// Load tests in the form of a throuhghput-test can be made through the
// Throughput function. This function will try to create a certain
// throughput load, i.e. a certain average number of request per second.
// The intervalls between request follow an exponential distribution
// which mimics the load generated from real-world, uncorrelated users.
//
// The requests are generated from different Scenarios which contribute
// a certain percentage of requests to the set of all requests. The
// scenarios are basically just suites of tests: One suite might simulate
// the bahaviour of a bot while an other scenario can simulate the
// behaviour of a "normal" user and a third scenario performs actions
// a user with special interests.
//
// The Tests of each suite/scenario are executed, including the checks.
// Note that some checks can produce additional requests which are
// not accounted in the throughput rate of the load test (but do hit the
// target server).
// Checks can be turned off on per scenario basis.
//
// If all tests (and thus request) in a suite/scenario have been
// executed, the suite is repeated. To reach the desired request througput
// rate each scenario is run in multiple parallel threads. New threads are
// started as needed. The number of threads may be limited on a per
// scenario basis.
//
package suite

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vdobler/ht/cookiejar"
	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/internal/bender"
)

// Scenario describes a single scenario to be run as part of a throughput test.
type Scenario struct {
	// Name of this scenario. May differ from the suite.
	Name string

	// RawSuite is the suite to execute repeatedly to generate load.
	*RawSuite

	// Percentage of requests in the throughput test taken from this
	// scenario.
	Percentage int

	// MaxThreads limits the number of threads used to generate load from
	// this scenario. The value 0 indicates unlimited number of threads.
	MaxThreads int

	// OmitChecks turns off checks in this scenario, i.e. only the request
	// is made but no checks are performed on the response.
	OmitChecks bool

	// The logger to use for tests of this scenario.
	Log *log.Logger

	globals map[string]string
	jar     *cookiejar.Jar
}

func (sc *Scenario) setup() *Suite {
	suite := NewFromRaw(sc.RawSuite, sc.globals, sc.jar, sc.Log)
	// Cap tests to setup-tests.
	suite.tests = suite.tests[:len(sc.RawSuite.Setup)]
	i := 0
	executor := func(test *ht.Test) error {
		i++
		test.Reporting.SeqNo = fmt.Sprintf("Setup-%02d", i)
		if test.Status == ht.Bogus || test.Status == ht.Skipped {
			return nil
		}
		if !sc.RawSuite.tests[i-1].IsEnabled() {
			test.Status = ht.Skipped
			return nil
		}
		test.Run()
		if test.Status > ht.Pass {
			return ErrAbortExecution
		}
		return nil
	}
	suite.Iterate(executor)
	sc.jar = suite.Jar
	sc.globals = suite.FinalVariables

	return suite
}

func (sc *Scenario) teardown() *Suite {
	suite := NewFromRaw(sc.RawSuite, sc.globals, sc.jar, sc.Log)
	// Cap tests to setup-tests.
	suite.tests = suite.tests[len(suite.tests)-len(sc.RawSuite.Teardown):]
	i := 0
	executor := func(test *ht.Test) error {
		i++
		test.Reporting.SeqNo = fmt.Sprintf("Teardown-%02d", i)
		if test.Status == ht.Bogus || test.Status == ht.Skipped {
			return nil
		}
		if !sc.RawSuite.tests[i-1].IsEnabled() {
			test.Status = ht.Skipped
			return nil
		}
		test.Run()
		return nil
	}
	suite.Iterate(executor)
	sc.jar = suite.Jar
	sc.globals = suite.FinalVariables

	return suite
}

type pool struct {
	Scenario
	No      int
	Chan    chan bender.Test
	wg      *sync.WaitGroup
	mu      *sync.Mutex
	Threads int
	Misses  int
}

// IDSep is the separator string used in constructing IDs for the individual
// test executed. The test Name are (miss)used to report details:
//
//     <ScenarioNo>/<ThreadNo>/<Repetition>/<TestNo> IDSep <ScenarioName> IDSep <TestName>
//
var IDSep = "\u2237"

// newThread starts a new thread/goroutine which iterates tests in the pool's
// scenario.
func (p *pool) newThread(stop chan bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.MaxThreads > 0 && p.Threads >= p.MaxThreads {
		if p.Scenario.Log != nil {
			p.Scenario.Log.Printf("No extra thread started (%d already running)\n", p.Threads)
		}
		p.Misses++
		return
	}
	p.Threads++
	if p.Scenario.Log != nil {
		fmt.Printf("Scenario %d %q: Starting new thread %d\n",
			p.No+1, p.Scenario.Name, p.Threads)
	}
	p.wg.Add(1)

	go func(thread int) {
		// Thread-local copy of globals; shared over all repetitions.
		thglobals := make(map[string]string, len(p.globals)+2)
		for n, v := range p.globals {
			thglobals[n] = v
		}
		thglobals["THREAD"] = strconv.Itoa(thread)

		repetition := 1
		done := false
		for !done {
			thglobals["REPETITION"] = strconv.Itoa(repetition)
			executed := make(chan bool)

			t := 0
			executor := func(test *ht.Test) error {
				t++
				test.Name = fmt.Sprintf("%d/%d/%d/%d%s%s%s%s",
					p.No, thread, repetition, t, IDSep,
					p.Scenario.Name, IDSep, test.Name)

				test.Reporting.SeqNo = test.Name

				if !p.Scenario.RawSuite.tests[t-1].IsEnabled() {
					test.Status = ht.Skipped
					return nil
				}
				select {
				case <-stop:
					done = true
					return ErrAbortExecution
				case p.Chan <- bender.Test{Test: test, Done: executed}:
					<-executed
				}

				return nil
			}
			// BUG/TDOD: make a copy of p.jar per thread.
			suite := NewFromRaw(p.Scenario.RawSuite, thglobals, p.jar, p.Log)
			nSetup, nMain := len(p.Scenario.RawSuite.Setup), len(p.Scenario.RawSuite.Main)
			suite.tests = suite.tests[nSetup : nSetup+nMain]
			suite.Iterate(executor)
			repetition += 1
		}
		p.wg.Done()
	}(p.Threads)
}

func makeRequest(scenarios []Scenario, rate float64, requests chan bender.Test, stop chan bool) ([]*pool, error) {
	// Choosing a scenario to contribute to the total set of request is done
	// by looking up a (thead) pool with the desired probability: Pool indixec
	// are distributed in 100 selectors.
	selector := make([]int, 100)

	// Set up a pool for each scenario and start an initial thread per pool.
	pools := make([]*pool, len(scenarios))
	j := 0
	for i, s := range scenarios {
		pool := pool{
			Scenario: s,
			No:       i + 1,
			Chan:     make(chan bender.Test),
			wg:       &sync.WaitGroup{},
			mu:       &sync.Mutex{},
		}
		pools[i] = &pool
		pools[i].newThread(stop)
		for k := 0; k < s.Percentage; k++ {
			if j+k > 99 {
				return nil, fmt.Errorf("suite: sum of Percentage greater than 100%%")
			}
			selector[j+k] = i
		}
		j += s.Percentage
	}
	if j <= 99 {
		return nil, fmt.Errorf("suite: sum of Percentage less than 100%%")
	}

	gracetime := time.Second / time.Duration(5*rate)
	go func() {
		// Select a pool (i.e. a scenario) and take a request from
		// this pool. If pool cannot deliver: Start a new thread in
		// this pool. Repeat until stop signaled.
		for {
			rn := rand.Intn(100)
			pool := pools[selector[rn]]
			var test bender.Test
			select {
			case <-stop:
				return
			case test = <-pool.Chan:
			default:
				pool.newThread(stop)
				time.Sleep(gracetime)
				continue
			}

			requests <- test

		}
	}()

	// Pre-start second thread, just to be ready
	for i := range scenarios {
		pools[i].newThread(stop)
	}

	return pools, nil
}

// Throughput runs a throughput load test with an average rate of request/seconds
// for the given duration. The request are generated from the given scenarios.
// It returns a summary of all request, all failed tests collected into a Suite
// and an error indicating if the throughput test reached the targeted rate and
// distribution.
func Throughput(scenarios []Scenario, rate float64, duration, ramp time.Duration) ([]bender.TestData, *Suite, error) {
	for i := range scenarios {
		// Setup must be called for all scenarios!
		fmt.Printf("Scenario %d %q: Running setup\n",
			i+1, scenarios[i].Name)
		ss := (&scenarios[i]).setup()
		if ss.Error != nil {
			return nil, ss, fmt.Errorf("Setup of scenario %d %q failed: %s",
				i+1, scenarios[i].Name, ss.Error)
		}
	}

	fmt.Printf("Starting Throughput test for %s at average of %.1f requests/second\n", duration, rate)
	recorder := make(chan bender.Event)
	data := make([]bender.TestData, 0, 1000)
	dataRecorder := bender.NewDataRecorder(&data)
	failures := make([]*ht.Test, 0, 1000)
	failureRecorder := bender.NewFailureRecorder(&failures)
	go bender.Record(recorder, dataRecorder, failureRecorder)

	request := make(chan bender.Test, len(scenarios))
	stop := make(chan bool)
	intervals := bender.ExponentialIntervalGenerator(rate)
	if ramp > 0 {
		intervals = bender.RampedExponentialIntervalGenerator(rate, ramp)
	}

	pools, err := makeRequest(scenarios, rate, request, stop)
	if err != nil {
		return nil, nil, err
	}

	bender.LoadTestThroughput(intervals, request, recorder)
	time.Sleep(duration)
	close(stop)
	fmt.Println("Finished Throughput test.")
	for _, p := range pools {
		fmt.Printf("Scenario %d %q: Draining pool with %d threads\n",
			p.No, p.Scenario.Name, p.Threads)
		p.wg.Wait()
	}
	close(request)
	time.Sleep(50 * time.Millisecond)

	err = analyseOutcome(data, pools)

	for i := range scenarios {
		if len(scenarios[i].RawSuite.Teardown) == 0 {
			continue
		}
		fmt.Printf("Running Teardown of scenario %d %q\n",
			i+1, scenarios[i].Name)
		(&scenarios[i]).teardown()
	}

	return data, makeFailureSuite(failures), err
}

func makeFailureSuite(failures []*ht.Test) *Suite {
	suite := Suite{
		Name:  "Throughput Failures",
		Tests: failures,
	}
	for i := range suite.Tests {
		suite.Tests[i].Name = suite.Tests[i].Reporting.SeqNo
		suite.Tests[i].Reporting.SeqNo = fmt.Sprintf("Failure-%03d", i)
	}

	return &suite
}

func analyseOutcome(data []bender.TestData, pools []*pool) error {
	errors := ht.ErrorList{}

	N := len(data)
	if N == 0 {
		errors = append(errors, fmt.Errorf("no data recorded"))
		return errors
	}

	err := analyseOverage(data)
	if err != nil {
		errors = append(errors, err)
	}

	err = analyseMisses(data, pools)
	if err != nil {
		errors = append(errors, err)
	}

	derr := analyseDistribution(data, pools)
	if derr != nil {
		errors = append(errors, derr...)
	}

	if len(errors) > 0 {
		return errors
	}
	return nil
}

func analyseMisses(data []bender.TestData, pools []*pool) error {
	errors := ht.ErrorList{}
	N := len(data)
	for i, p := range pools {
		expected := N * p.Scenario.Percentage / 100
		if p.Misses <= expected/50 {
			continue
		}
		errors = append(errors,
			fmt.Errorf("scenario %d %q got %d thread misses",
				i+1, p.Scenario.Name, p.Misses))
	}
	if len(errors) > 0 {
		return errors
	}
	return nil
}

func analyseDistribution(data []bender.TestData, pools []*pool) ht.ErrorList {
	errors := ht.ErrorList{}
	N := len(data)

	cnt := make(map[int]int)
	repPerThread := make(map[int]map[int]int)
	repMax := make(map[int]int)
	for _, d := range data {
		parts := strings.SplitN(d.ID, IDSep, 3)
		nums := strings.Split(parts[0], "/")
		// Count suite.
		sn, _ := strconv.Atoi(nums[0])
		sn--
		cnt[sn] = cnt[sn] + 1
		// Record repetitions
		t, _ := strconv.Atoi(nums[1])
		r, _ := strconv.Atoi(nums[2])
		reps := repPerThread[sn]
		if len(reps) == 0 {
			reps = make(map[int]int)
		}
		reps[t] = r
		repPerThread[sn] = reps
		if max := repMax[sn]; r > max {
			repMax[sn] = r
		}
	}

	// Check scenario percentages
	for i, p := range pools {
		actual := cnt[i]
		fmt.Printf("Scenario %d %q: %d request = %.1f%% (target %d%%), %d threads created, %d thread misses, repetitions",
			i+1, p.Scenario.Name,
			actual, float64(100*actual)/float64(N),
			p.Scenario.Percentage,
			p.Threads, p.Misses)
		for t := 1; t <= p.Threads; t++ {
			fmt.Printf(" %d", repPerThread[i][t])
		}
		fmt.Println()
		low := N * (p.Scenario.Percentage - 5) / 100
		high := N * (p.Scenario.Percentage + 5) / 100
		if low <= actual && actual <= high {
			continue
		}
		errors = append(errors,
			fmt.Errorf("scenario %d %q contributed %.1f%% (want %d%%)",
				i+1, p.Scenario.Name, float64(100*actual)/float64(N),
				p.Scenario.Percentage))
	}

	// Check each scenario is repeated at least three times which mean it was
	// executed fully at least twice.
	for i, p := range pools {
		if repMax[i] < 3 {
			errors = append(errors,
				fmt.Errorf("scenario %d %q fully executed only %d times",
					i+1, p.Scenario.Name, repMax[i]-1))
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

func analyseOverage(data []bender.TestData) error {
	N := len(data)
	s := time.Duration(0)
	for _, v := range data[N/2 : N] {
		s += v.Overage
	}
	mean := s / time.Duration(N-N/2)

	if mean > 1*time.Millisecond {
		return fmt.Errorf("mean overage of last half = %s is too big", mean)
	}

	return nil
}

func dToMs(d time.Duration) float64 { return float64(d/1000) / 1000 }

// DataPrint prints data to out in human readable tabular format.
func DataPrint(data []bender.TestData, out io.Writer) {
	timeLayout := "2006-01-02T15:04:05.999"

	fmt.Fprintln(out, "Started                  Status   Duration        Health  S/T/R/N ID                 Error")
	fmt.Fprintln(out, "===========================================================================================")
	for _, d := range data {
		emsg := ""
		if d.Error != nil {
			emsg = d.Error.Error()
		}
		health := fmt.Sprintf("[%.1f %.1f]", dToMs(d.Wait), dToMs(d.Overage))
		fmt.Fprintf(out, "%-24s %-8s %8.2f  %12s  %s  %s  \n",
			d.Started.Format(timeLayout), d.Status,
			dToMs(d.ReqDuration),
			health,
			d.ID, emsg)
	}

}

// DataToCSV prints data as a CVS table to out after sorting data by Started.
func DataToCSV(data []bender.TestData, out io.Writer) error {
	sort.Sort(bender.ByStarted(data))
	rateWindow := time.Second
	if fullDuration := data[len(data)-1].Started.Sub(data[0].Started); fullDuration < 5*time.Second {
		rateWindow = 500 * time.Millisecond
	} else if fullDuration > 30*time.Second {
		rateWindow = 2 * time.Second
	}
	writer := csv.NewWriter(out)
	defer writer.Flush()

	header := []string{
		"Number",
		"Started",
		"Elapsed",
		"Rate",
		"Status",
		"ReqDuration",
		"TestDuration",
		"Wait",
		"Overage",
		"ID",
		"Suite",
		"Test",
		"SuiteNo",
		"ThreadNo",
		"Repetition",
		"TestNo",
		"Error",
	}
	err := writer.Write(header)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}

	first := data[0].Started

	r := make([]string, 0, 16)
	for i, d := range data {
		r = append(r, fmt.Sprintf("%d", i))
		r = append(r, d.Started.Format("2006-01-02T15:04:05.99999Z07:00"))
		r = append(r, fmt.Sprintf("%.3f", dToMs(d.Started.Sub(first))))
		r = append(r, fmt.Sprintf("%.1f", effectiveRate(i, data, rateWindow)))
		r = append(r, d.Status.String())
		r = append(r, fmt.Sprintf("%.3f", dToMs(d.ReqDuration)))
		r = append(r, fmt.Sprintf("%.3f", dToMs(d.TestDuration)))
		r = append(r, fmt.Sprintf("%.1f", dToMs(d.Wait)))
		r = append(r, fmt.Sprintf("%.1f", dToMs(d.Overage)))

		part := strings.SplitN(d.ID, IDSep, 3)
		r = append(r, part...)
		nums := strings.SplitN(part[0], "/", 4)
		r = append(r, nums...)
		if d.Error != nil {
			r = append(r, d.Error.Error())
		} else {
			r = append(r, "")
		}

		err := writer.Write(r)
		if err != nil {
			return err
		}

		r = r[:0]
	}
	writer.Flush()

	return nil
}

// effectiveRate returns the number of request in data in a window
// around sample i.
func effectiveRate(i int, data []bender.TestData, window time.Duration) float64 {
	t0 := data[i].Started
	a, b := i, i
	for a > 0 && t0.Sub(data[a].Started) < window/2 {
		a--
	}
	for b < len(data)-1 && data[b].Started.Sub(t0) < window/2 {
		b++
	}
	n := float64(b-a+1) / float64(window/time.Second)

	if a == 0 || b == len(data)-1 {
		// Window is capped. Compensate n for to narrow window.
		effectiveWindow := data[b].Started.Sub(data[a].Started)
		if effectiveWindow == 0 {
			effectiveWindow = window
		}
		n *= float64(window) / float64(effectiveWindow)
	}

	return float64(n)
}
