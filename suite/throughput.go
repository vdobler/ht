// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Throughput Load-Testing
//
// Load tests in the form of a throuhghput-test can be made through the
// Throughput function. This function will try to create a certain
// throughput load, i.e. a certain average number of request per second
// also known as query per second (QPS).
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
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
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

	globals map[string]string
	jar     *cookiejar.Jar
}

// setup runs the Setup tests of sc.
func (sc *Scenario) setup(logger *log.Logger) *Suite {
	suite := NewFromRaw(sc.RawSuite, sc.globals, sc.jar, logger)
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

// teardown runs the Teardown tests of sc.
func (sc *Scenario) teardown(logger *log.Logger) *Suite {
	suite := NewFromRaw(sc.RawSuite, sc.globals, sc.jar, logger)
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
var IDSep = "\u2237"

// newThread starts a new thread/goroutine which iterates tests in the pool's
// scenario.
func (p *pool) newThread(stop chan bool, logger *log.Logger) {
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
				break
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

				// TODO: Put this stuff into SeqNo only??
				test.Name = fmt.Sprintf("%d/%d/%d/%d%s%s%s%s",
					p.No+1, thread, repetition, t, IDSep,
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
// The request are drawn randoemly from the given scenarios (while each suite
// the scenario consists of executes linearely on each thread).
// The thread pool of the scenarios is returned for cleanup purpose.
func makeRequest(scenarios []Scenario, rate float64, requests chan bender.Test, stop chan bool, logger *log.Logger) ([]*pool, error) {
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
				logger.Println("Request generation stopped.")
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

	logger.Println("Request generation started.")
	return pools, nil
}

// Throughput runs a throughput load test with request taken from the given
// scenarios.
// During the ramp the rquest rate is linearely increased until it reaches
// the desired rate of requests/second (QPS). This rate is kept for the
// rest of the loadtest i.e. for duration-ramp.
//
// Setup and Teardown tests in the scenarios are executed once for each
// scenario before and after starting the loadtest. Note that loadtesting
// differs from suite execution here: Cookies set during Setup are _not_
// propagated to the Main tests (but Setup and Teardown share a cookie jar).
//
// The actual load is  generated from the Main tests. A scenario can be
// executed multiple times in parallel threads.  After full execution of all
// Main tests each thread starts over and re-executes the Main tests again
// by generating a new Suite.
//
// Setup tests may populate the set of variables used for the Main (and of
// course Teardown) test. Additionaly the two variables THREAD and REPETITION
// are set for each round of Main tests. The distinction between thread and
// repetition looks strange given that each repetition is isolated and
// independent from the thread it occors, but it is not: Repetitions are
// unbound and an endurance test running for 8 hours might repeate the Main
// test severla thousend times but maybe just using 15 threads which allows
// to prepare the system under test e.g. with 15 numbered user-accounts.
// It will aggregate all executed Test with a Status higher (bader) or equal
// to collectFrom.
//
// It returns a summary of all request, the aggregated tests and an error
// indicating if the throughput test reached the targeted rate and the desired
// distribution between scenarios.
func Throughput(scenarios []Scenario, rate float64, duration, ramp time.Duration, collectFrom ht.Status, csvout io.Writer) ([]TestData, *Suite, error) {
	bufferedStdout := bufio.NewWriterSize(os.Stdout, 1)
	logger := log.New(bufferedStdout, "", 256)

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

	logger.Printf("Starting Throughput test for %s (ramp %s) at average of %.1f requests/second \n", duration, ramp, rate)
	bufferedStdout.Flush()

	recorder := make(chan bender.Event)
	data := make([]TestData, 0, 1000)
	collectedTests := make([]*ht.Test, 0, 1000)
	csvWriter := csv.NewWriter(csvout)
	defer csvWriter.Flush()
	recordingDone := make(chan bool)
	go bender.Record(recorder, recordingDone,
		newRecorder(&data, &collectedTests, collectFrom, csvWriter))

	request := make(chan bender.Test, 2*len(scenarios))
	stop := make(chan bool)
	intervals := bender.ExponentialIntervalGenerator(rate)
	if ramp > 0 {
		intervals = bender.RampedExponentialIntervalGenerator(rate, ramp)
	}

	pools, err := makeRequest(scenarios, rate, request, stop, logger)
	if err != nil {
		return nil, nil, err
	}

	bender.LoadTestThroughput(intervals, request, recorder)
	time.Sleep(duration)
	close(stop)
	logger.Println("Finished Throughput test.")
	for _, p := range pools {
		logger.Printf("Scenario %d %q: Draining pool with %d threads\n",
			p.No+1, p.Scenario.Name, p.Threads)
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
	bufferedStdout.Flush()
	err = analyseOutcome(data, pools)

	return data, makeCollectedSuite(collectedTests, collectFrom), err
}

func makeCollectedSuite(tests []*ht.Test, from ht.Status) *Suite {
	suite := Suite{
		Name:  fmt.Sprintf("Throughput Test with Status >= %s", from),
		Tests: tests,
	}
	for i := range suite.Tests {
		parts := strings.Split(suite.Tests[i].Reporting.SeqNo, IDSep)
		nums := strings.Split(parts[0], "/")
		scen, _ := strconv.Atoi(nums[0])
		thrd, _ := strconv.Atoi(nums[1])
		rept, _ := strconv.Atoi(nums[2])
		test, _ := strconv.Atoi(nums[3])

		suite.Tests[i].Name = suite.Tests[i].Reporting.SeqNo
		suite.Tests[i].Reporting.SeqNo = fmt.Sprintf("Scen%02d-Thread%02d-Rep%02d-Test%02d",
			scen, thrd, rept, test)
	}

	return &suite
}

func analyseOutcome(data []TestData, pools []*pool) error {
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

func analyseMisses(data []TestData, pools []*pool) error {
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

func analyseDistribution(data []TestData, pools []*pool) ht.ErrorList {
	errors := ht.ErrorList{}
	N := len(data)

	cnt := make(map[int]int)
	repPerThread := make(map[int]map[int]int)
	fullExecution := make(map[int]int) // number of full rounds
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
		fullExecution[sn] = fullExecution[sn] + r - 1
	}

	// Check scenario percentages
	for i, p := range pools {
		actual := cnt[i]
		fmt.Printf("Scenario %d %q: %d requests = %.1f%% (target %d%%), %d threads created, %d thread misses, repetitions",
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

// TODO: handle errors
func splitID(id string) ([]string, []string) {
	part := strings.SplitN(id, IDSep, 3)
	nums := strings.SplitN(part[0], "/", 4)
	return part, nums
}

// DataToCSV prints data as a CVS table to out after sorting data by Started.
func DataToCSV(data []TestData, out io.Writer) error {
	if len(data) == 0 {
		return nil
	}
	sort.Sort(ByStarted(data))
	rateWindow := time.Second
	if fullDuration := data[len(data)-1].Started.Sub(data[0].Started); fullDuration <= 5*time.Second {
		rateWindow = 500 * time.Millisecond
	} else if fullDuration <= 20*time.Second {
		rateWindow = 2 * time.Second
	} else if fullDuration <= 60*time.Second {
		rateWindow = 3 * time.Second
	} else {
		rateWindow = 5 * time.Second
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
		r = append(r, d.Started.Format("2006-01-02T15:04:05.99999Z07:00"))
		r = append(r, fmt.Sprintf("%.3f", dToMs(d.Started.Sub(first))))
		r = append(r, fmt.Sprintf("%.1f", effectiveRate(i, data, rateWindow)))
		r = append(r, d.Status.String())
		r = append(r, fmt.Sprintf("%.3f", dToMs(d.ReqDuration)))
		r = append(r, fmt.Sprintf("%.3f", dToMs(d.TestDuration)))
		r = append(r, fmt.Sprintf("%.1f", dToMs(d.Wait)))
		r = append(r, fmt.Sprintf("%.1f", dToMs(d.Overage)))
		concTot, concOwn := concurrencyLevel(i, data)
		r = append(r, fmt.Sprintf("%d", concTot))
		r = append(r, fmt.Sprintf("%d", concOwn))

		part, nums := splitID(d.ID)
		r = append(r, part...)
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
	part := strings.SplitN(data[i].ID, IDSep, 3)
	test := part[2]
	i--

	for ; i >= 0; i-- {
		end := data[i].Started.Add(data[i].ReqDuration)
		if end.Before(t0) {
			continue
		}
		total++
		part := strings.SplitN(data[i].ID, IDSep, 3)
		if test == part[2] {
			own++
		}

	}

	return total, own
}

// ----------------------------------------------------------------------------
// Recorder

type TestData struct {
	Started      time.Time
	Status       ht.Status
	ReqDuration  time.Duration
	TestDuration time.Duration
	ID           string
	Error        error
	Wait         time.Duration
	Overage      time.Duration
}

type ByStarted []TestData

func (s ByStarted) Len() int           { return len(s) }
func (s ByStarted) Less(i, j int) bool { return s[i].Started.Before(s[j].Started) }
func (s ByStarted) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func newRecorder(data *[]TestData, tests *[]*ht.Test, from ht.Status, w *csv.Writer) bender.Recorder {
	cnt := 0
	start := time.Now()
	r := make([]string, 0, 14)
	header := []string{
		"Started",
		"Elapsed",
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
	w.Write(header) // TODO: handle error; also below

	return func(e bender.Event) {
		if e.Typ != bender.EndRequestEvent {
			return
		}

		// Data Recorder
		d := TestData{
			Started:      e.Test.Started,
			Status:       e.Test.Status,
			ReqDuration:  time.Duration(e.Test.Response.Duration),
			TestDuration: time.Duration(e.Test.Duration),
			ID:           e.Test.Reporting.SeqNo,
			Error:        e.Test.Error,
			Wait:         time.Duration(e.Wait),
			Overage:      time.Duration(e.Overage),
		}
		*data = append(*data, d)

		// Test Recorder
		if e.Test.Status >= from {
			*tests = append(*tests, e.Test)
		}

		// CSV Recording
		r = r[:0]
		r = append(r, e.Test.Started.Format("2006-01-02T15:04:05.99999Z07:00"))
		r = append(r, fmt.Sprintf("%.3f", dToMs(e.Test.Started.Sub(start))))
		r = append(r, e.Test.Status.String())
		r = append(r, fmt.Sprintf("%.3f", dToMs(e.Test.Response.Duration)))
		r = append(r, fmt.Sprintf("%.3f", dToMs(e.Test.Duration)))
		r = append(r, fmt.Sprintf("%.1f", dToMs(time.Duration(e.Wait))))
		r = append(r, fmt.Sprintf("%.1f", dToMs(time.Duration(e.Overage))))
		part, nums := splitID(e.Test.Reporting.SeqNo)
		r = append(r, part...)
		r = append(r, nums...)

		if e.Test.Error != nil {
			r = append(r, e.Test.Error.Error())
		} else {
			r = append(r, "")
		}

		w.Write(r)
		if cnt%5 == 0 {
			w.Flush()
		}
		cnt++
	}
}
