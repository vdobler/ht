// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// latency.go contains checks against response time latency.

package ht

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http/cookiejar"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
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
	// The special values "stdout" and "stderr" are recognised.
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

	conc := L.Concurrent

	// Provide a set of Test instances to be executed.
	tests := []*Test{}
	for i := 0; i < conc; i++ {
		cpy, err := Merge(t)
		if err != nil {
			return err
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
		cpy.Verbosity = 0

		if t.Jar != nil {
			if L.IndividualSessions {
				cpy.Jar, _ = cookiejar.New(nil)
			} else {
				cpy.Jar = t.Jar
			}
		}
		tests = append(tests, cpy)
	}

	results := make(chan latencyResult, 2*conc)

	// Synchronous warmup phase is much simpler.
	wg := &sync.WaitGroup{}
	started := time.Now()
	prewarmed := 0
	for prewarm := 0; prewarm < 2; prewarm++ {
		for i := 0; i < conc; i++ {
			prewarmed++
			wg.Add(1)
			go func(ex *Test) {
				ex.Run(t.Variables)
				wg.Done()
			}(tests[i])
		}
		wg.Wait()
	}
	offset := time.Since(started) / time.Duration(prewarmed*conc)

	// Conc requests generation workers execute their own test instance
	// until the channel running is closed.
	done := make(chan bool)
	wg2 := &sync.WaitGroup{}
	wg2.Add(1) // Add one sentinel value.
	for i := 0; i < conc; i++ {
		go func(ex *Test, id int) {
			for {
				wg2.Add(1)
				ex.Run(t.Variables)
				results <- latencyResult{
					status:   ex.Status,
					started:  ex.Started,
					duration: ex.Response.Duration,
					execBy:   id,
				}
				wg2.Done()
				select {
				case <-done:
					return
				default:
				}
			}
		}(tests[i], i)
		time.Sleep(offset)
	}

	// Collect results into data and signal end via done.
	data := make([]latencyResult, 2*L.N)
	counters := make([]int, Bogus)
	// TODO: clean t.Name from ',' and other stuff illegal in csv files.
	checkid := fmt.Sprintf("%s,%d", t.Name, L.Concurrent)
	seen := uint64(0) // Bitmap of testinstances who returned.
	all := (uint64(1) << uint(conc)) - 1
	measureFrom := 0
	n := 0
	started = time.Now()
	for n < len(data) {
		data[n] = <-results
		if seen == all {
			if measureFrom == 0 {
				measureFrom = n
			}
			counters[data[n].status]++
		} else {
			seen |= 1 << uint(data[n].execBy)
		}
		n++

		// TODO: Make limits configurable?
		if counters[Pass] == L.N ||
			counters[Fail] > L.N/5 ||
			counters[Error] > L.N/20 ||
			time.Since(started) > 3*time.Minute {
			break
		}
	}
	close(done)
	data = data[:n]

	// Drain rest; wait till requests currently in flight die.
	wg2.Done() // Done with sentinel value.
	wg2.Wait()

	completed := false
	if counters[Pass] == L.N {
		completed = true
	}
	for _, r := range data {
		fmt.Fprintf(dumper, "%s,%s,%s,%d,%t\n",
			checkid,
			r.started.Format(time.RFC3339Nano),
			r.status,
			r.duration/Duration(time.Millisecond),
			completed,
		)
	}

	latencies := []int{}
	for i, r := range data {
		if i < measureFrom || r.status != Pass {
			continue
		}
		latencies = append(latencies, int(r.duration)/int(time.Millisecond))
	}
	sort.Ints(latencies)

	errs := ErrorList{}
	for _, lim := range L.limits {
		lat := time.Millisecond * time.Duration(quantile(latencies, lim.q))
		t.infof("Latency quantil (conc=%d) %0.2f%% ≤ %d ms",
			conc, lim.q*100, lat/time.Millisecond)
		if lat > lim.max {
			errs = append(errs, fmt.Errorf("%.2f%% = %s > limit %s",
				100*lim.q, lat, lim.max))
		}
	}

	if !completed {
		errs = append(errs, fmt.Errorf("Got only %d PASS but %d FAIL and %d ERROR",
			counters[Pass], counters[Fail], counters[Error]))
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

type latencyResult struct {
	status   Status
	started  time.Time
	duration Duration
	execBy   int
}

// https://en.wikipedia.org/wiki/Quantile formula R-8
func quantile(x []int, p float64) int {
	N := float64(len(x))
	if p < 2.0/(3.0*(N+1.0/3.0)) {
		return x[0]
	}
	if p >= (N-1.0/3.0)/(N+1.0/3.0) {
		return x[len(x)-1]
	}

	h := (N+1.0/3.0)*p + 1.0/3.0
	fh := math.Floor(h)
	xl := x[int(fh)-1]
	xr := x[int(fh)]

	return xl + int((h-fh)*float64(xr-xl)+0.5)
}

// Prepare implements Check's Prepare method.
func (L *Latency) Prepare() error {
	if L.N == 0 {
		L.N = 50
	}
	if L.Concurrent == 0 {
		L.Concurrent = 2
	} else if L.Concurrent > 64 {
		return MalformedCheck{
			Err: fmt.Errorf("concurrency %d > allowed max of 64",
				L.Concurrent),
		}
	}

	if L.Limits == "" {
		L.Limits = "75% ≤ 500"
	}

	if err := L.parseLimit(); err != nil {
		fmt.Printf("err = %v\n", err)
		return MalformedCheck{Err: err}
	}

	return nil
}

func (L *Latency) parseLimit() error {
	parts := strings.Split(L.Limits, ";")
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
		return 0, 0, fmt.Errorf("missing '≤'")
	}

	q := strings.TrimSpace(strings.TrimRight(parts[0], " %"))
	quantile, err := strconv.ParseFloat(q, 64)
	if err != nil {
		return 0, 0, err
	}
	if strings.Index(parts[0], "%") != -1 {
		quantile /= 100
	}
	if quantile < 0 || quantile > 1 {
		return 0, 0, fmt.Errorf("quantile %.3f out of range [0,1]", quantile)
	}

	b := strings.TrimSpace(strings.TrimLeft(parts[1], "="))

	m, err := time.ParseDuration(b)
	if err != nil {
		return 0, 0, err
	}
	if m <= 0 || m > 300*time.Second {
		return 0, 0, fmt.Errorf("limit %s out of range (0,300s]", m)
	}

	return quantile, m, nil
}

/*

For Conc==4 and RT_orig == 12  ==>  offset==3

 1 +---------+ +---------+ +---------+ +---------+ +---------+
 2    +---------+ +---------+ +---------+ +---------+ +---------+
 3       +---------+ +---------+ +---------+ +---------+ +---------+
 4          +---------+ +---------+ +---------+ +---------+ +---------+
         |  |           |           |
        -+--+-         -+-----------+-
        offset             RT_orig

*/
