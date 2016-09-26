// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suite

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vdobler/ht/cookiejar"
	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/internal/bender"
)

// Scenario to run in a throughput test.
type Scenario struct {
	// RawSuite is the suite to execute repeatedly to generate lod.
	*RawSuite

	// Percentage of requests in the load test taken from this Scenario.
	Percentage int

	Log *log.Logger

	globals map[string]string
	jar     *cookiejar.Jar
}

func (sc *Scenario) Setup(globals map[string]string) *Suite {
	suite := NewFromRaw(sc.RawSuite, globals, sc.jar, sc.Log)
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

func (sc *Scenario) Teardown() *Suite {
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
	Threads int
	wg      *sync.WaitGroup
	mu      *sync.Mutex
}

var IDSep = "\u2237"

func (p *pool) newThread(stop chan bool) {
	p.mu.Lock()
	p.Threads++
	fmt.Printf("%s, Thread %d: Start\n",
		p.Scenario.RawSuite.Name, p.Threads)
	p.wg.Add(1)

	go func(thread int) {
		// Thread-local copy of globals; shared over all repetitions.
		thglobals := make(map[string]string, len(p.globals)+2)
		for n, v := range p.globals {
			thglobals[n] = v
		}
		thglobals["THREAD"] = strconv.Itoa(thread)

		n := 1
		done := false
		for !done {
			thglobals["REPETITION"] = strconv.Itoa(n)
			executed := make(chan bool)

			t := 0
			executor := func(test *ht.Test) error {
				t++
				test.Name = fmt.Sprintf("%d/%d/%d/%d%s%s%s%s",
					p.No, thread, n, t, IDSep,
					p.Scenario.RawSuite.Name, IDSep, test.Name)

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
			n += 1
		}
		p.wg.Done()
	}(p.Threads)
	p.mu.Unlock()

}

func makeRequest(scenarios []Scenario, rate float64, globals map[string]string, requests chan bender.Test, stop chan bool) ([]*pool, error) {
	pools := make([]*pool, len(scenarios))
	selector := make([]int, 100)
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

func Throughput(scenarios []Scenario, rate float64, duration time.Duration, globals map[string]string) ([]bender.TestData, *Suite, error) {
	for i := range scenarios {
		// Setup must be called for all scenarios!
		fmt.Printf("Running Setup of scenario %d %s\n", i+1, scenarios[i].RawSuite.Name)
		ss := (&scenarios[i]).Setup(globals)
		if ss.Error != nil {
			return nil, ss, fmt.Errorf("Setup of scenario %d %q failed: %s",
				i+1, scenarios[i].RawSuite.Name, ss.Error)
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

	pools, err := makeRequest(scenarios, rate, globals, request, stop)
	if err != nil {
		return nil, nil, err
	}

	bender.LoadTestThroughput(intervals, request, recorder)
	time.Sleep(duration)
	close(stop)
	fmt.Println("Finished Throughput test.")
	for _, p := range pools {
		fmt.Printf("Waiting for pool %d (%d threads) %s\n",
			p.No, p.Threads, p.Scenario.RawSuite.Name)
		p.wg.Wait()
	}
	close(request)
	time.Sleep(50 * time.Millisecond)

	err = analyseOverage(data)
	for i := range scenarios {
		if len(scenarios[i].RawSuite.Teardown) == 0 {
			continue
		}
		fmt.Printf("Running Teardwon of scenario %d %s\n", i+1, scenarios[i].RawSuite.Name)
		(&scenarios[i]).Teardown()
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

func DataToCSV(data []bender.TestData, out io.Writer) error {
	writer := csv.NewWriter(out)
	defer writer.Flush()

	header := []string{
		"Number",
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
	err := writer.Write(header)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}

	first := data[0].Started
	for _, d := range data {
		if d.Started.Before(first) {
			first = d.Started
		}
	}

	r := make([]string, 0, 16)
	for i, d := range data {
		r = append(r, fmt.Sprintf("%d", i))
		r = append(r, d.Started.Format("2006-01-02T15:04:05.99999Z07:00"))
		r = append(r, fmt.Sprintf("%.3f", dToMs(d.Started.Sub(first))))
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

	return nil
}
