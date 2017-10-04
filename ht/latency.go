// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// latency.go contains checks against response time latency.

package ht

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vdobler/ht/cookiejar"
	"github.com/vdobler/ht/errorlist"
)

func init() {
	RegisterCheck(&Latency{})
}

// ----------------------------------------------------------------------------
// Latency

// Latency provides checks against percentils of the response time latency.
type Latency struct {
	// N is the number if request to measure. It should be much larger
	// than Concurrent. Default is 50.
	N int `json:",omitempty"`

	// Concurrent is the number of concurrent requests in flight.
	// Defaults to 2.
	Concurrent int `json:",omitempty"`

	// Limits is a string of the following form:
	//    "50% ≤ 150ms; 80% ≤ 200ms; 95% ≤ 250ms; 0.9995 ≤ 0.9s"
	// The limits above would require the median of the response
	// times to be <= 150 ms and would allow only 1 request in 2000 to
	// exced 900ms.
	// Note that it must be the ≤ sign (U+2264), a plain < or a <=
	// is not recognized.
	Limits string `json:",omitempty"`

	// IndividualSessions tries to run the concurrent requests in
	// individual sessions: A new one for each of the Concurrent many
	// requests (not N many sessions).
	// This is done by using a fresh cookiejar so it won't work if the
	// request requires prior login.
	IndividualSessions bool `json:",omitempty"`

	// If SkipChecks is true no checks are performed i.e. only the
	// requests are executed.
	SkipChecks bool `json:",omitempty"`

	// DumpTo is the filename where the latencies are reported.
	// The special values "stdout" and "stderr" are recognized.
	DumpTo string `json:",omitempty"`

	limits []latLimit
}

type latLimit struct {
	q   float64
	max time.Duration
}

// Execute implements Check's Execute method.
func (L *Latency) Execute(t *Test) error {
	var dumper io.Writer
	switch L.DumpTo {
	case "":
		dumper = ioutil.Discard
	case "stdout":
		dumper = os.Stdout
	case "stderr":
		dumper = os.Stderr
	default:
		file, err := os.OpenFile(L.DumpTo, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer file.Close()
		buffile := bufio.NewWriter(file)
		defer buffile.Flush()
		dumper = buffile
	}
	csvWriter := csv.NewWriter(dumper)
	defer csvWriter.Flush()

	// Provide a set of Test instances to be executed.
	tests, err := L.produceTestSet(t)
	if err != nil {
		return err
	}
	// Warump phase. Used to warmup the server side (not our code here).
	averageRT := L.warmup(tests)
	offset := averageRT / time.Duration(L.Concurrent)

	conc := L.Concurrent

	// Collect all results in an array. If we abort early the status will
	// be NotRun. Collection should be fast enough.
	resultCh := make(chan latencyResult, 3*L.Concurrent)
	data := make([]latencyResult, L.N)
	done := make(chan bool)
	started := time.Now()
	go func() {
		for i := 0; i < len(data) && time.Since(started) < 3*time.Minute; i++ {
			data[i] = <-resultCh
		}
		close(done)
	}()

	// Conc requests generation workers execute their own test instance
	// until the channel running is closed.
	wg := &sync.WaitGroup{}
	for i := 0; i < conc; i++ {
		wg.Add(1)
		go func(ex *Test, id int) {
			for running := true; running; {
				ex.Run()
				lr := latencyResult{
					status:   ex.Result.Status,
					started:  ex.Result.Started,
					duration: ex.Response.Duration,
					execBy:   id,
				}
				select {
				case <-done:
					running = false
				default:
					select {
					case resultCh <- lr:
					default:
					}
				}
			}
			wg.Done()

		}(tests[i], i)
		time.Sleep(offset)
	}
	wg.Wait()

	// Analyse data. We fail this checks if:
	//   - We stopped early (after 3 minutes)
	//   - Not all tests did Passed
	//   - Not all L.Concurrent workers did contribute a result
	//   - Some percentiles are too high
	var errs errorlist.List
	counters := make([]int, Bogus+1)
	seen := uint64(0) // Bitmap of testinstances who returned.
	for _, res := range data {
		counters[res.status]++
		seen |= 1 << uint(res.execBy)
	}
	if counters[NotRun] > 0 {
		errs = errs.Append(fmt.Errorf("Check timed out, got only %d measurements",
			L.N-counters[NotRun]))
	} else if counters[Pass] != L.N {
		errs = errs.Append(fmt.Errorf("Got %d Fail, %d Error, %d Bogus",
			counters[Fail], counters[Error], counters[Bogus]))
	}
	if seen != (uint64(1)<<uint(conc))-1 {
		errs = errs.Append(fmt.Errorf("Not all %d concurrent workers did provide a result (%x)",
			L.Concurrent, seen))
	}

	Z := 1 * time.Microsecond
	fields := make([]string, 6)
	fields[1] = fmt.Sprintf("%d", L.Concurrent)
	fields[5] = fmt.Sprintf("%t", len(data) == L.N)
	for _, r := range data {
		d := Z * ((r.duration + Z/2) / Z) // cut off nanosecond (=noise) part
		fields[0] = t.Name
		fields[2] = r.started.Format(time.RFC3339Nano)
		fields[3] = r.status.String()
		fields[4] = d.String()
		csvWriter.Write(fields)
	}

	latencies := make([]int, len(data))
	for i, r := range data {
		latencies[i] = int(r.duration) / int(time.Millisecond)
	}
	sort.Ints(latencies)

	for _, lim := range L.limits {
		lat := time.Millisecond * time.Duration(quantile(latencies, lim.q))
		t.infof("Latency quantil (conc=%d) %0.2f%% ≤ %d ms",
			conc, lim.q*100, lat/time.Millisecond)
		if lat > lim.max {
			errs = errs.Append(fmt.Errorf("%.2f%% = %s > limit %s",
				100*lim.q, lat, lim.max))
		}

	}

	return errs.AsError()
}

// produce L.Concurrent copies of t suitable for execution according to L.
func (L *Latency) produceTestSet(t *Test) ([]*Test, error) {
	tests := make([]*Test, L.Concurrent)
	for i := range tests {
		cpy, err := Merge(t)
		if err != nil {
			return nil, err
		}
		cpy.Name = fmt.Sprintf("Latency-Test %d", i+1)
		checks := []Check{}
		for _, c := range t.Checks {
			if _, lt := c.(*Latency); L.SkipChecks || lt {
				continue
			}
			checks = append(checks, c)
		}
		cpy.Checks = checks
		cpy.Execution.Verbosity = 0

		// TODO: limit cpy.Request.Timeout to let's say 10 times
		// what's the allowed value for the highes percentil.
		// Advantage: would speed things up
		// Disadvantage: might count longrunning-but-passing tests
		// as errored tests.

		if t.Jar != nil {
			if L.IndividualSessions {
				// BUG: should populate cpy.Jar
				cpy.Jar, _ = cookiejar.New(nil)
			} else {
				cpy.Jar = t.Jar
			}
		}
		tests[i] = cpy
	}
	return tests, nil
}

// warump the server by running tests. Returns the average response time.
func (L *Latency) warmup(tests []*Test) time.Duration {
	wg := &sync.WaitGroup{}
	started := time.Now()
	prewarmed := 0
	for prewarm := 0; prewarm < 2; prewarm++ {
		for _, t := range tests {
			prewarmed++
			wg.Add(1)
			go func(ex *Test) {
				ex.Run()
				wg.Done()
			}(t)
		}
		wg.Wait()
	}
	// Warump is also used to determine how long a single request takes in
	// average. This average devided by L.Concurrent determines the offset
	// to start the worker threads
	// For L.Concurent==4 and  average response time==12  ==>  offset==3
	//   1 +---------+ +---------+ +---------+ +---------+
	//   2    +---------+ +---------+ +---------+ +---------+
	//   3       +---------+ +---------+ +---------+ +---------+
	//   4          +---------+ +---------+ +---------+ +---------+
	//           |  |           |           |
	//          -+--+-         -+-----------+-
	//          offset             average
	return time.Since(started) / time.Duration(prewarmed) // average
}

type latencyResult struct {
	status   Status
	started  time.Time
	duration time.Duration
	execBy   int
}

// https://en.wikipedia.org/wiki/Quantile formula R-8
func quantile(x []int, p float64) float64 {
	if len(x) == 0 {
		return 0
	}
	N := float64(len(x))
	if p < 2.0/(3.0*(N+1.0/3.0)) {
		return float64(x[0])
	}
	if p >= (N-1.0/3.0)/(N+1.0/3.0) {
		return float64(x[len(x)-1])
	}

	h := (N+1.0/3.0)*p + 1.0/3.0
	fh := math.Floor(h)
	xl := x[int(fh)-1]
	xr := x[int(fh)]

	return float64(xl) + (h-fh)*float64(xr-xl)
}

// Prepare implements Check's Prepare method.
func (L *Latency) Prepare(*Test) error {
	if L.N == 0 {
		L.N = 50
	}
	if L.Concurrent == 0 {
		L.Concurrent = 2
	} else if L.Concurrent > 64 {
		return fmt.Errorf("concurrency %d > allowed max of 64", L.Concurrent)
	}

	if L.Limits == "" {
		L.Limits = "75% ≤ 500"
	}

	if err := L.parseLimit(); err != nil {
		return err
	}

	return nil
}

var _ Preparable = &Latency{}

func (L *Latency) parseLimit() error {
	parts := strings.Split(strings.Trim(L.Limits, "; "), ";")
	for i := range parts {
		s := strings.TrimSpace(parts[i])
		q, t, err := parseQantileLimit(s)
		if err != nil {
			return err
		}

		L.limits = append(L.limits, latLimit{q: q, max: t})
	}

	return nil
}

func parseQantileLimit(s string) (float64, time.Duration, error) {
	parts := strings.SplitN(s, "≤", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("Latency: missing '≤' in %q", s)
	}

	q := strings.TrimSpace(strings.TrimRight(parts[0], " %"))
	quantile, err := strconv.ParseFloat(q, 64)
	if err != nil {
		return 0, 0, err
	}
	if strings.Contains(parts[0], "%") {
		quantile /= 100
	}
	if quantile < 0 || quantile > 1 {
		return 0, 0, fmt.Errorf("Latency: quantile %.3f out of range [0,1]", quantile)
	}

	b := strings.TrimSpace(strings.TrimLeft(parts[1], "="))

	m, err := time.ParseDuration(b)
	if err != nil {
		return 0, 0, err
	}
	if m <= 0 || m > 300*time.Second {
		return 0, 0, fmt.Errorf("Latency: limit %s out of range (0,300s]", m)
	}

	return quantile, m, nil
}

/*


 */
