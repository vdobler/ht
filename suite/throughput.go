// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Throughput Load-Testing
//
// Load tests in the form of a throuhghput-test can be made through the
// Throughput function. This function will try to create a certain
// throughput load, i.e. a certain average number of request per second
// also known as query per second (QPS).
// The intervals between request follow an exponential distribution
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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vdobler/ht/cookiejar"
	"github.com/vdobler/ht/errorlist"
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

	globals map[string]string
	jar     *cookiejar.Jar
}

// setup runs the Setup tests of sc.
func (sc *Scenario) setup(logger ht.Logger) *Suite {
	suite := NewFromRaw(sc.RawSuite, sc.globals, sc.jar, logger)
	// Cap tests to setup-tests.
	suite.tests = suite.tests[:len(sc.RawSuite.Setup)]
	i := 0
	executor := func(test *ht.Test) error {
		i++
		test.SetMetadata("SeqNo", fmt.Sprintf("Setup-%02d", i))
		if test.Result.Status == ht.Bogus || test.Result.Status == ht.Skipped {
			return nil
		}
		if !sc.RawSuite.tests[i-1].IsEnabled() {
			test.Result.Status = ht.Skipped
			return nil
		}
		test.Run()
		if test.Result.Status > ht.Pass {
			return ErrAbortExecution
		}
		return nil
	}
	suite.Iterate(executor)
	sc.jar = suite.Jar
	sc.globals = suite.FinalVariables

	return suite
}

// teardown runs the Teardown tests of sc.
func (sc *Scenario) teardown(logger ht.Logger) *Suite {
	suite := NewFromRaw(sc.RawSuite, sc.globals, sc.jar, logger)
	// Cap tests to setup-tests.
	suite.tests = suite.tests[len(suite.tests)-len(sc.RawSuite.Teardown):]
	i := 0
	executor := func(test *ht.Test) error {
		i++
		test.SetMetadata("SeqNo", fmt.Sprintf("Teardown-%02d", i))
		if test.Result.Status == ht.Bogus || test.Result.Status == ht.Skipped {
			return nil
		}
		if !sc.RawSuite.tests[i-1].IsEnabled() {
			test.Result.Status = ht.Skipped
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

// A pool is kinda thread pool for the given scenario.
type pool struct {
	Scenario
	No      int // Sequence number of the scenario
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
var IDSep = "\u2055" // "\u203b" "\u2237"

// newThread starts a new thread/goroutine which iterates tests in the pool's
// scenario.
func (p *pool) newThread(stop chan bool, logger ht.Logger) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.MaxThreads > 0 && p.Threads >= p.MaxThreads {
		if p.Scenario.Verbosity >= 2 {
			logger.Printf("Scenario %d %q: No extra thread started (%d already running)\n",
				p.No+1, p.Scenario.Name, p.Threads)
		}
		p.Misses++
		return
	}
	p.Threads++
	if p.Scenario.Verbosity >= 1 {
		logger.Printf("Scenario %d %q: Starting new thread %d\n",
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
			select {
			case <-stop:
				done = true
			default:
			}
			if done {
				break
			}
			thglobals["REPETITION"] = strconv.Itoa(repetition)
			executed := make(chan bool)

			t := 0
			executor := func(test *ht.Test) error {
				t++

				ti := TestIdentifier{
					Scenario:     p.No + 1,
					Thread:       thread,
					Repetition:   repetition,
					Test:         t,
					ScenarioName: p.Scenario.Name,
					TestName:     test.Name,
				}
				test.Name = ti.String()
				test.SetMetadata("Identifier", ti)

				if !p.Scenario.RawSuite.tests[t-1].IsEnabled() {
					test.Result.Status = ht.Skipped
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
			suite := NewFromRaw(p.Scenario.RawSuite, thglobals, nil, logger)

			nSetup, nMain := len(p.Scenario.RawSuite.Setup), len(p.Scenario.RawSuite.Main)
			suite.tests = suite.tests[nSetup : nSetup+nMain]
			suite.Iterate(executor)
			if p.Scenario.Verbosity >= 2 {
				logger.Printf("Scenario %d %q: Finished repetition %d of thread %d: %s\n",
					p.No+1, p.Scenario.Name, repetition, thread, suite.Status)
			}

			repetition += 1
		}
		if p.Scenario.Verbosity >= 1 {
			logger.Printf("Scenario %d %q: Done with thread %d",
				p.No+1, p.Scenario.Name, thread)
		}
		p.wg.Done()
	}(p.Threads)
}

// makeRequests is responsible for generating a stream of request and providing
// these requests on the requests channel until the stop channel is closed.
// The request are drawn randomly from the given scenarios (while each suite
// the scenario consists of executes linearely on each thread).
// The thread pool of the scenarios is returned for cleanup purpose.
func makeRequest(scenarios []Scenario, rate float64, requests chan bender.Test, stop chan bool, logger ht.Logger) ([]*pool, error) {
	// Choosing a scenario to contribute to the total set of request is done
	// by looking up a (thread) pool with the desired probability: Pool indices
	// are distributed in 100 selectors.
	selector := make([]int, 100)

	// Set up a pool for each scenario and start an initial thread per pool.
	pools := make([]*pool, len(scenarios))
	cummulatedPercentage := 0
	for i, s := range scenarios {
		pool := pool{
			Scenario: s,
			No:       i,
			Chan:     make(chan bender.Test, 2),
			wg:       &sync.WaitGroup{},
			mu:       &sync.Mutex{},
		}
		pools[i] = &pool
		pools[i].newThread(stop, logger)
		for k := 0; k < s.Percentage; k++ {
			if cummulatedPercentage+k > 99 {
				return nil, fmt.Errorf("suite: sum of Percentage greater than 100%%")
			}
			selector[cummulatedPercentage+k] = i
		}
		cummulatedPercentage += s.Percentage
	}
	if cummulatedPercentage <= 99 {
		return nil, fmt.Errorf("suite: sum of Percentage less than 100%%")
	}

	gracetime := time.Second / time.Duration(5*rate)
	go func() {
		// Select a pool (i.e. a scenario) and take a request from
		// this pool. If pool cannot deliver: Start a new thread in
		// this pool. Repeat until stop signaled.
		counter := 0
		for {
			rn := rand.Intn(100)
			pool := pools[selector[rn]]
			var test bender.Test
			select {
			case <-stop:
				logger.Printf("Request generation stopped.\n")
				return
			case test = <-pool.Chan:
				counter++
				requests <- test
			default:
				pool.newThread(stop, logger)
				time.Sleep(gracetime)
			}
		}
	}()

	// Pre-start second thread, just to be ready
	for i := range scenarios {
		pools[i].newThread(stop, logger)
	}

	logger.Printf("Request generation started.\n")
	return pools, nil
}

// ThroughputOptions collects options which control execution of a throughput
// load test.
type ThroughputOptions struct {
	// Rate is the target rate of request per second (QPS).
	Rate float64

	// Duration of the whole throughput test.
	Duration time.Duration

	// Ramp duration during which the actual rate (QPS) is linearily
	// increased to the target rate.
	Ramp time.Duration

	// MaxErrorRate is the maximal tolerable error rate over the last
	// 50 request. If the error rate exceeds this limit the throughput
	// test finishes early. Values <= 0 disable aborting on errors.
	MaxErrorRate float64

	// CollectFrom limit collection of tests to those test with a
	// status equal or bader.
	CollectFrom ht.Status
}

// Throughput runs a throughput load test with requests taken from the given
// scenarios.
// During the ramp the request rate is linearely increased until it reaches
// the desired rate of requests/second (QPS). This rate is kept for the
// rest of the loadtest i.e. for duration-ramp.
//
// Setup and Teardown tests in the scenarios are executed once for each
// scenario before and after starting the loadtest. Note that loadtesting
// differs from suite execution here: Cookies set during Setup are _not_
// propagated to the Main tests (but Setup and Teardown share a cookie jar).
//
// The actual load is generated from the Main tests. A scenario can be
// executed multiple times in parallel threads. After full execution of all
// Main tests each thread starts over and re-executes the Main tests again
// by generating a new Suite.
//
// Setup tests may populate the set of variables used for the Main (and of
// course Teardown) test. Additionally the two variables THREAD and REPETITION
// are set for each round of Main tests. The distinction between thread and
// repetition looks strange given that each repetition is isolated and
// independent from the thread it occors, but it is not: Repetitions are
// unbound and an endurance test running for 8 hours might repeate the Main
// test several thousend times but maybe just using 15 threads which allows
// to prepare the system under test e.g. with 15 numbered user-accounts.
// It will aggregate all executed Test with a Status higher (bader) or equal
// to CollectFrom.
//
// It returns a summary of all request, the aggregated tests and an error
// indicating if the throughput test reached the targeted rate and the desired
// distribution between scenarios.
func Throughput(scenarios []Scenario, opts ThroughputOptions, csvout io.Writer, logger ht.Logger) ([]TestData, *Suite, error) {
	// bufferedStdout := bufio.NewWriterSize(os.Stdout, 1)
	// logger := log.New(bufferedStdout, "", 256)
	if csvout == nil {
		// TODO: Instead of discarding writes it might be more sensible
		// and effective to not write to csvout at all if nil. As the
		// csv encoding takes some time this would allow to safe valuable
		// computing power.
		csvout = ioutil.Discard
	}
	if logger == nil {
		logger = log.New(ioutil.Discard, "", 0)
	}

	// Make sure all request come from some scenario.
	sum := 0
	for i := range scenarios {
		sum += scenarios[i].Percentage
	}
	if sum != 100 {
		return nil, nil, fmt.Errorf("Sum of Percentage = %d%% (must be 100)", sum)
	}

	// Execute Teardown code on any case.
	defer func() {
		for i := range scenarios {
			if len(scenarios[i].RawSuite.Teardown) == 0 {
				continue
			}
			logger.Printf("Scenario %d %q: Running Teardown\n",
				i+1, scenarios[i].Name)
			(&scenarios[i]).teardown(logger)
		}
	}()

	// Execute Setup code.
	for i := range scenarios {
		// Setup must be called for all scenarios!
		logger.Printf("Scenario %d %q: Running setup\n",
			i+1, scenarios[i].Name)
		ss := (&scenarios[i]).setup(logger)
		if ss.Error != nil {
			return nil, ss, fmt.Errorf("Setup of scenario %d %q failed: %s",
				i+1, scenarios[i].Name, ss.Error)
		}
	}

	logger.Printf("Starting Throughput test for %s (ramp %s) at average of %.1f requests/second \n",
		opts.Duration, opts.Ramp, opts.Rate)
	// bufferedStdout.Flush()

	recorder := make(chan bender.Event)
	data := make([]TestData, 0, 1000)
	collectedTests := make([]*ht.Test, 0, 1000)
	csvWriter := csv.NewWriter(csvout)
	maxer := 2.0 // more than 100% --> ignore errors
	if opts.MaxErrorRate > 0 {
		maxer = opts.MaxErrorRate
	}
	statusRing := NewStatusRing(50, maxer)
	defer csvWriter.Flush()
	recordingDone := make(chan bool)
	go bender.Record(recorder, recordingDone,
		newRecorder(&data, &collectedTests, opts.CollectFrom, csvWriter, statusRing))

	request := make(chan bender.Test, 2*len(scenarios))
	stop := make(chan bool)
	intervals := bender.ExponentialIntervalGenerator(opts.Rate)
	if opts.Ramp > 0 {
		intervals = bender.RampedExponentialIntervalGenerator(opts.Rate, opts.Ramp)
	}

	pools, err := makeRequest(scenarios, opts.Rate, request, stop, logger)
	if err != nil {
		return nil, nil, err
	}

	bender.LoadTestThroughput(intervals, request, recorder)
	started, loopCnt := time.Now(), 0
	elapsed := time.Duration(0)
	statusCounts := make([]int, int(ht.Bogus)+1)
	for elapsed < opts.Duration {
		elapsed = time.Since(started)
		elins := ((elapsed + 500 + time.Millisecond) / time.Second) * time.Second
		if loopCnt%10 == 0 {
			logger.Printf("Throughput test running for %s (%d%% completed)",
				elins, 100*elapsed/opts.Duration)
		}
		if bad := statusRing.Status(statusCounts); bad {
			logger.Printf("Throughput test aborted after %s (%d%% completed): Too many errors",
				elins, 100*elapsed/opts.Duration)
			for s, n := range statusCounts {
				logger.Printf("  Status %-7s: %d", ht.Status(s), n)
			}
			break
		}
		loopCnt++
		time.Sleep(1 * time.Second)
	}
	close(stop)
	logger.Printf("Finished Throughput test.\n")
	for _, p := range pools {
		p.mu.Lock()
		logger.Printf("Scenario %d %q: Draining pool with %d threads\n",
			p.No+1, p.Scenario.Name, p.Threads)
		p.mu.Unlock()
		go func(p *pool) {
			for {
				t, ok := <-p.Chan
				if ok {
					t.Done <- true
				} else {
					break
				}
			}
		}(p)
		p.wg.Wait()
	}
	close(request)
	<-recordingDone // Explicit syncronisation of test output recording.
	for _, p := range pools {
		close(p.Chan)
	}
	time.Sleep(50 * time.Millisecond)
	// bufferedStdout.Flush()
	err = analyseOutcome(data, pools, logger)

	return data, makeCollectedSuite(collectedTests, opts.CollectFrom), err
}

// TestIdentifier identifies a single Test, i.e. a single request in a
// throughput test. All number fields are 1-based.
type TestIdentifier struct {
	Scenario     int
	Thread       int
	Repetition   int
	Test         int
	ScenarioName string
	TestName     string
}

// NumericalID formats ti like "3/5/2/7".
func (ti TestIdentifier) NumericalID() string {
	return fmt.Sprintf("%d/%d/%d/%d",
		ti.Scenario, ti.Thread, ti.Repetition, ti.Test)
}

// String returns a human readable representation of ti.
func (ti TestIdentifier) String() string {
	return fmt.Sprintf("%s%s%s%s%s",
		ti.NumericalID(), IDSep, ti.ScenarioName, IDSep, ti.TestName)
}

// TestIDFromString is the inverse of TestIdentifier.String.
func TestIDFromString(s string) (TestIdentifier, error) {
	// Must be kept in sync with TestIdentifier.String().
	// TODO: sprinkle with error handling
	parts := strings.SplitN(s, IDSep, 3)
	nums := strings.Split(parts[0], "/")
	se, _ := strconv.Atoi(nums[0])
	th, _ := strconv.Atoi(nums[1])
	re, _ := strconv.Atoi(nums[2])
	te, _ := strconv.Atoi(nums[3])

	return TestIdentifier{
		Scenario:     se,
		Thread:       th,
		Repetition:   re,
		Test:         te,
		ScenarioName: parts[1],
		TestName:     parts[2],
	}, nil
}

func makeCollectedSuite(tests []*ht.Test, from ht.Status) *Suite {
	suite := Suite{
		Name:  fmt.Sprintf("Throughput Test with Status >= %s", from),
		Tests: tests,
	}
	for i := range suite.Tests {
		ti := suite.Tests[i].GetMetadata("Identifier").(TestIdentifier)
		scen := ti.Scenario
		thrd := ti.Thread
		rept := ti.Repetition
		test := ti.Test

		suite.Tests[i].SetMetadata("SeqNo",
			fmt.Sprintf("Scen%02d-Thread%02d-Rep%02d-Test%02d",
				scen, thrd, rept, test))
	}

	return &suite
}

func analyseOutcome(data []TestData, pools []*pool, logger ht.Logger) error {
	errors := errorlist.List{}

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

	derr := analyseDistribution(data, pools, logger)
	if derr != nil {
		errors = append(errors, derr...)
	}

	if len(errors) > 0 {
		return errors
	}
	return nil
}

func analyseMisses(data []TestData, pools []*pool) error {
	errors := errorlist.List{}
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

func analyseDistribution(data []TestData, pools []*pool, logger ht.Logger) errorlist.List {
	errors := errorlist.List{}
	N := len(data)

	cnt := make(map[int]int)
	repPerThread := make(map[int]map[int]int)
	fullExecution := make(map[int]int) // number of full rounds
	for _, d := range data {
		// Count suite.
		sn := d.ID.Scenario - 1
		cnt[sn] = cnt[sn] + 1
		// Record repetitions
		t := d.ID.Thread
		r := d.ID.Repetition

		reps := repPerThread[sn]
		if len(reps) == 0 {
			reps = make(map[int]int)
		}
		reps[t] = r
		repPerThread[sn] = reps
		fullExecution[sn] = fullExecution[sn] + r - 1
	}

	// Check scenario percentages: Each scenario should contribute
	// approximately as much as given in its Percentage field.
	// Statistical fluctuations happen and are larger if only a few
	// request are made.
	tolerance := percentageTolerance(N)
	for i, p := range pools {
		actual := cnt[i]
		logger.Printf("Scenario %d %q: %d request = %.1f%% (target %d%%)",
			i+1, p.Scenario.Name, actual, float64(100*actual)/float64(N),
			p.Scenario.Percentage)
		reps := make([]string, p.Threads)
		for t := 1; t <= p.Threads; t++ {
			reps[t-1] = fmt.Sprintf("%d", repPerThread[i][t])
		}
		logger.Printf("Scenario %d %q: %d threads created, %d thread misses, repetitions %s",
			i+1, p.Scenario.Name, p.Threads, p.Misses, strings.Join(reps, " "))
		logger.Printf("\n")
		low := N * (p.Scenario.Percentage - tolerance) / 100
		if low <= 0 {
			low = 1
		}
		high := N * (p.Scenario.Percentage + tolerance) / 100
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
		if fullExecution[i] < 3 {
			errors = append(errors,
				fmt.Errorf("scenario %d %q fully executed only %d times",
					i+1, p.Scenario.Name, fullExecution[i]))
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

func percentageTolerance(N int) int {
	if N < 400 {
		return 10
	} else if N < 800 {
		return 5
	}
	return 3
}

func analyseOverage(data []TestData) error {
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

// Convert a duration d (in ns) to millisecond with microsecond precission.
func dToMs(d time.Duration) float64 { return float64(d/1000) / 1000 }

// DataPrint prints data to out in human readable tabular format.
func DataPrint(data []TestData, out io.Writer) {
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

// DataToCSV prints data as a CSV table to out after sorting data by Started.
func DataToCSV(data []TestData, out io.Writer) error {
	if len(data) == 0 {
		return nil
	}
	sort.Sort(ByStarted(data))
	rateWindow := 5 * time.Second
	if fullDuration := data[len(data)-1].Started.Sub(data[0].Started); fullDuration <= 5*time.Second {
		rateWindow = 500 * time.Millisecond
	} else if fullDuration <= 20*time.Second {
		rateWindow = 2 * time.Second
	} else if fullDuration <= 60*time.Second {
		rateWindow = 3 * time.Second
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
		"ConcTot",
		"ConcOwn",
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

	r := make([]string, 0, 19)
	for i, d := range data {
		r = append(r, fmt.Sprintf("%d", i))
		r = append(r, d.Started.Format("2006-01-02T15:04:05.999999Z07:00"))
		r = append(r, fmt.Sprintf("%.2f", dToMs(d.Started.Sub(first))))
		r = append(r, fmt.Sprintf("%.1f", effectiveRate(i, data, rateWindow)))
		r = append(r, d.Status.String())
		r = append(r, fmt.Sprintf("%.2f", dToMs(d.ReqDuration)))
		r = append(r, fmt.Sprintf("%.2f", dToMs(d.TestDuration)))
		r = append(r, fmt.Sprintf("%.1f", dToMs(d.Wait)))
		r = append(r, fmt.Sprintf("%.1f", dToMs(d.Overage)))
		concTot, concOwn := concurrencyLevel(i, data)
		r = append(r, fmt.Sprintf("%d", concTot))
		r = append(r, fmt.Sprintf("%d", concOwn))

		r = append(r, d.ID.NumericalID())
		r = append(r, d.ID.ScenarioName)
		r = append(r, d.ID.TestName)
		r = append(r, fmt.Sprintf("%d", d.ID.Scenario))
		r = append(r, fmt.Sprintf("%d", d.ID.Thread))
		r = append(r, fmt.Sprintf("%d", d.ID.Repetition))
		r = append(r, fmt.Sprintf("%d", d.ID.Test))

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
func effectiveRate(i int, data []TestData, window time.Duration) float64 {
	t0 := data[i].Started
	a, b := i, i
	for a > 0 && t0.Sub(data[a].Started) < window/2 {
		a--
	}
	for b < len(data)-1 && data[b].Started.Sub(t0) < window/2 {
		b++
	}
	n := float64(time.Second) * float64(b-a+1) / float64(window)
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

// concurrencyLevel computes how many request in total and of the same Test
// are inflight when the i'th request started.
func concurrencyLevel(i int, data []TestData) (int, int) {
	total, own := 1, 1

	t0 := data[i].Started
	test := data[i].ID.Test // TODO: Should this be TesName to collect over different scenarios?
	i--

	for ; i >= 0; i-- {
		end := data[i].Started.Add(data[i].ReqDuration)
		if end.Before(t0) {
			continue
		}
		total++
		if test == data[i].ID.Test {
			own++
		}

	}

	return total, own
}

// ----------------------------------------------------------------------------
// Recorder

// TestData collects main information about a single Test executed during
// a throughput test.
type TestData struct {
	Started      time.Time
	Status       ht.Status
	ReqDuration  time.Duration
	TestDuration time.Duration
	ID           TestIdentifier
	Error        error
	Wait         time.Duration
	Overage      time.Duration
}

// fields serialises data into a set of strings suitable (i.e. in appropriate
// units) to write to a CSV file. Must stay in sync with testdataFromFields.
func (data TestData) fields() []string {
	r := make([]string, 13)
	r[0] = data.Started.Format("2006-01-02T15:04:05.999999Z07:00")
	r[1] = data.Status.String()
	r[2] = fmt.Sprintf("%.3f", dToMs(data.ReqDuration))
	r[3] = fmt.Sprintf("%.3f", dToMs(data.TestDuration))
	r[4] = fmt.Sprintf("%.1f", dToMs(time.Duration(data.Wait)))
	r[5] = fmt.Sprintf("%.1f", dToMs(time.Duration(data.Overage)))
	r[6] = fmt.Sprintf("%d", data.ID.Scenario)
	r[7] = fmt.Sprintf("%d", data.ID.Thread)
	r[8] = fmt.Sprintf("%d", data.ID.Repetition)
	r[9] = fmt.Sprintf("%d", data.ID.Test)
	r[10] = data.ID.ScenarioName
	r[11] = data.ID.TestName
	if data.Error != nil {
		r[12] = data.Error.Error()
	}
	return r
}

func testdataFromFields(line []string) (TestData, error) {
	data := TestData{}
	var err error
	data.Started, err = time.Parse("2006-01-02T15:04:05.999999Z07:00", line[0])
	if err != nil {
		return data, err
	}

	data.Status = ht.StatusFromString(line[1])
	if data.Status < 0 {
		return data, fmt.Errorf("unknown status %q", line[1])
	}

	msToD := func(s string) time.Duration {
		f, e := strconv.ParseFloat(s, 64)
		if e != nil && err == nil {
			err = e
		}
		return time.Duration(f * 1e6)
	}
	sToN := func(s string) int {
		n, e := strconv.Atoi(s)
		if e != nil && err == nil {
			err = e
		}
		return n
	}
	data.ReqDuration = msToD(line[2])
	data.TestDuration = msToD(line[3])
	data.Wait = msToD(line[4])
	data.Overage = msToD(line[5])

	data.ID.Scenario = sToN(line[6])
	data.ID.Thread = sToN(line[7])
	data.ID.Repetition = sToN(line[8])
	data.ID.Test = sToN(line[9])
	data.ID.ScenarioName = line[10]
	data.ID.TestName = line[11]

	if line[12] != "" {
		data.Error = errors.New(line[12])
	}

	return data, err
}

// ByStarted sorts []TestData by the Started field.
type ByStarted []TestData

func (s ByStarted) Len() int           { return len(s) }
func (s ByStarted) Less(i, j int) bool { return s[i].Started.Before(s[j].Started) }
func (s ByStarted) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func newRecorder(data *[]TestData, tests *[]*ht.Test, from ht.Status, w *csv.Writer, sr *StatusRing) bender.Recorder {
	cnt := 0
	header := []string{
		"Started",      // 0
		"Status",       // 1
		"ReqDuration",  // 2
		"TestDuration", // 3
		"Wait",         // 4
		"Overage",      // 5
		"Scenario",     // 6
		"Thread",       // 7
		"Repetition",   // 8
		"Test",         // 9
		"ScenName",     // 10
		"TestName",     // 11
		"Error",
	}
	w.Write(header) // TODO: handle error; also below

	return func(e bender.Event) {
		if e.Typ != bender.EndRequestEvent {
			return
		}

		// Data Recorder
		d := TestData{
			Started:      e.Test.Result.Started,
			Status:       e.Test.Result.Status,
			ReqDuration:  time.Duration(e.Test.Response.Duration),
			TestDuration: time.Duration(e.Test.Result.Duration),
			ID:           e.Test.GetMetadata("Identifier").(TestIdentifier),
			Error:        e.Test.Result.Error,
			Wait:         time.Duration(e.Wait),
			Overage:      time.Duration(e.Overage),
		}
		*data = append(*data, d)

		// Test Recorder
		if e.Test.Result.Status >= from {
			*tests = append(*tests, e.Test)
		}

		// CSV Recording
		w.Write(d.fields())
		if cnt%5 == 0 {
			w.Flush()
		}
		cnt++

		// StatusRing
		sr.Store(e.Test.Result.Status)
	}
}

// ReadLiveCSV unmarshals the csv generated during a throughput test back
// to TestData.
func ReadLiveCSV(r io.Reader) ([]TestData, error) {
	csvreader := csv.NewReader(r)

	all, err := csvreader.ReadAll()
	if err != nil {
		return nil, err
	}

	all = all[1:] // strip header
	data := make([]TestData, 0, len(all))
	for i, line := range all {
		d, err := testdataFromFields(line)
		if err != nil {
			return data, fmt.Errorf("line %d: %s", i+2, err)
		}
		data = append(data, d)
	}

	return data, nil
}

// ----------------------------------------------------------------------------
// Ring of Status

// A StatusRing is a ring buffer for ht.Status value counts.
type StatusRing struct {
	mu     *sync.Mutex
	values []ht.Status
	next   int
	cnt    []int
	limit  float64
	full   bool
	bad    bool // true once limit is exceeded
}

// NewStatusRing returns a StatusRing of size n and the given error limit.
func NewStatusRing(n int, limit float64) *StatusRing {
	sr := StatusRing{
		mu:     &sync.Mutex{},
		values: make([]ht.Status, n),
		cnt:    make([]int, int(ht.Bogus)+1),
		limit:  limit,
	}
	sr.cnt[int(ht.NotRun)] = n
	return &sr
}

// Store v in the ring buffer sr.
func (sr *StatusRing) Store(v ht.Status) {
	sr.mu.Lock()
	old := int(sr.values[sr.next])
	vv := int(v)

	sr.cnt[old]--
	sr.cnt[vv]++
	sr.values[sr.next] = v
	sr.next++
	if sr.next >= len(sr.values) {
		sr.full = true
		sr.next = 0
	}

	if sr.full && !sr.bad {
		if float64(sr.cnt[int(ht.Error)])/float64(len(sr.values)) > sr.limit {
			sr.bad = true
		}
	}

	sr.mu.Unlock()
}

// Status fills out with current counts and returns whether the error limit
// is execed. After execution out[0] contains the number of ht.NotRun
// values stored, out[1] the number of ht.Skipped values and so on.
func (sr *StatusRing) Status(out []int) bool {
	sr.mu.Lock()
	copy(out, sr.cnt)
	b := sr.bad
	sr.mu.Unlock()
	return b
}
