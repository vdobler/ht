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
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func init() {
	RegisterCheck(&Latency{})
}

// ----------------------------------------------------------------------------
// Latency

type Latency struct {
	// N is the number if request to measure. It should be much larger
	// than Concurrent. Default is 50.
	N int `json:",omitempty"`

	// Concurrent is the number of concurrent requests in flight.
	// Defaults to 2.
	Concurrent int `json:",omitempty"`

	// Limits is a string of the following form:
	//    "50% < 150; 80% < 200; 95% < 250; 0.9995 < 900"
	// The limits above would require the median of the response
	// times to be < 150 ms and would allow only 1 request in 2000 to
	// exced 900ms.
	Limits string `json:",omitempty"`

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
	offset := time.Duration(t.Response.Duration) / time.Duration(conc)
	// offset -= 10 * time.Millisecond

	// Provide a set of Test instances to be executed.
	tests := make(chan *Test, conc) // buffer of unused tests
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
		tests <- cpy
	}

	warmup := conc * 2
	type result struct {
		status   Status
		started  time.Time
		duration Duration
	}
	results := make(chan result, L.N+warmup)

	done := make(chan bool)

	// Collect results into data and signal end via done.
	data := make([]result, 0, L.N)
	npass, nerr, nfail := 0, 0, 0
	// TODO: clean t.Name from ',' and other stuff illegal in csv files.
	checkid := fmt.Sprintf("%s,%d", t.Name, L.Concurrent)
	go func() {
		for {
			r := <-results
			switch r.status {
			case Pass:
				npass++
				data = append(data, r)
			case Error:
				nerr++
			case Fail:
				nfail++
			default:
				panic(r.status.String())
			}
			fmt.Fprintf(dumper, "%s,%s,%s,%d\n",
				checkid,
				r.started.Format(time.RFC3339Nano),
				r.status,
				r.duration/Duration(time.Millisecond))
			// TODO: limit total running time, etc.
			if npass == L.N || nfail > L.N/5 || nerr > L.N/20 {
				close(done)
				break
			}
		}
	}()

	// Main loop generating requests.
	i := 0
mainLoop:
	for {
		select {
		case <-done:
			break mainLoop
		default:
		}

		if i < conc {
			time.Sleep(offset)
		}

		go func(ex *Test, measure bool) {
			ex.Run(nil) // TODO: get copy of variables from somewhere
			if measure {
				results <- result{
					status:   ex.Status,
					started:  ex.Started,
					duration: ex.Response.Duration,
				}
			}
			tests <- ex // return to unused buffer
		}(<-tests, i > warmup) // grap next unused test from buffer

		i++
	}

	// Drain rest; wait till requests currently in flight die.
	for len(results) > 0 {
		<-results
	}

	latencies := make([]int, len(data))
	for i, r := range data {
		latencies[i] = int(r.duration) / int(time.Millisecond)
	}
	sort.Ints(latencies)

	if len(latencies) < L.N {
		return fmt.Errorf("Got only %d PASS but %d FAIL and %d ERR",
			npass, nfail, nerr)
	}

	errs := ""
	for _, lim := range L.limits {
		lat := time.Millisecond * time.Duration(quantile(latencies, lim.q))
		if lat > lim.max {
			if errs != "" {
				errs += "; "
			}
			errs += fmt.Sprintf("%.2f%% = %s > limit %s",
				100*lim.q, lat, lim.max)
		}
	}
	if errs != "" {
		return fmt.Errorf("%s", errs)
	}

	return nil
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
	}

	if L.Limits == "" {
		L.Limits = "75% < 500"
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
	parts := strings.SplitN(s, "<", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("missing '<'")
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
