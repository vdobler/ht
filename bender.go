// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pinterest/bender"
)

// makeRequestChannel returns a channel on which the non-disabled main
// tests of the given suites are sent. The test from the suites are
// interweaved. All given suites must have at least one non-disabled
// main test.
//
// At most count tests are sent for at most the given duration.
func makeRequestChannel(suites []*Suite, count int, duration time.Duration) chan interface{} {
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

type LoadTestOptions struct {
	Type    string // one of "throughput" or "concurrency"
	Count   int
	Timeout time.Duration
	Rate    float64
	Uniform bool
}

// LoadTest will perform a load test of the main tests of suites by repeating them
// until count request have been made or the load tests takes longer than duration.
// The load test is a throughput test if rate > 0 and a concurrent load test if
// concurrent > 0.
// Errors are reported if any suite's Setup failed.
func LoadTest(suites []*Suite, opts LoadTestOptions) ([]Result, error) {
	if opts.Type != "throughput" && opts.Type != "concurrency" {
		return nil, fmt.Errorf("Unknown load tests type %q", opts.Type)
	}
	if opts.Timeout == 0 {
		opts.Timeout = 7 * 24 * time.Hour
	}

	// Setup
	for i, s := range suites {
		result := s.ExecuteSetup()
		if result.Status != Pass {
			return nil, fmt.Errorf("Setup of suite %d failed: %s", i, result.Error)
		}
	}

	log.Printf("Running load testwith %+v", opts)

	rc := makeRequestChannel(suites, opts.Count, opts.Timeout)

	executor := func(now int64, r interface{}) (interface{}, error) {
		t := r.(*Test)
		result := t.Run()
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

	allResults := make([]Result, 0, opts.Count)

	resultRec := func(msg interface{}) {
		if ere, ok := msg.(*bender.EndRequestEvent); ok {
			if result, ok := ere.Response.(Result); ok {
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

func AnalyseLoadtest(results []Result) {
	p, f, e, s, b := 0, 0, 0, 0, 0
	var max, maxp, maxf, maxe time.Duration
	for _, r := range results {
		if r.Duration > max {
			max = r.Duration
		}
		switch r.Status {
		case Pass:
			p++
			if r.Duration > maxp {
				maxp = r.Duration
			}
		case Fail:
			f++
			if r.Duration > maxf {
				maxf = r.Duration
			}
		case Error:
			e++
			if r.Duration > maxe {
				maxe = r.Duration
			}
		case Skipped:
			s++
		case Bogus:
			b++
		}
	}
	fmt.Printf("Pass=%d Fail=%d  Error=%d  Skipped=%d  Bogus=%d\n", p, f, e, s, b)

	histAll, histPass, histFail := NewLogHist(int(max/time.Millisecond)),
		NewLogHist(int(maxp/time.Millisecond)), NewLogHist(int(maxf/time.Millisecond))
	for _, r := range results {
		histAll.Add(int(r.Duration / time.Millisecond))
		switch r.Status {
		case Pass:
			histPass.Add(int(r.Duration / time.Millisecond))
		case Fail:
			histFail.Add(int(r.Duration / time.Millisecond))
		}
	}

	fmt.Printf("All Request: %s\n", histAll)
	fmt.Printf("Pass Request: %s\n", histPass)
	fmt.Printf("Fail Request: %s\n", histFail)
	histAll.PercentilPlot("")
	histAll.PercentilPlot("percentil.png")

}

type LogHist struct {
	Max      int
	Count    []int
	Last     int
	Overflow int
}

func NewLogHist(max int) LogHist {
	lh := LogHist{}
	last := lh.bucket(max) + 1
	lh.Max = max
	lh.Last = last
	lh.Count = make([]int, last)
	return lh
}

func NewHistogramRecorder(h *LogHist) bender.Recorder {
	return func(msg interface{}) {
		switch msg := msg.(type) {
		case *bender.EndRequestEvent:
			elapsed := int((msg.End - msg.Start) / int64(time.Millisecond))
			h.Add(elapsed)
		}
	}
}

var (
	// TODO: generate algoritmicaly
	defaultPlotPercentils = []float64{0,
		5, 10, 20, 25, 30, 35, 40, 50, 55, 60, 65, 70, 75, 80, 83, 85,
		87, 89, 90, 91, 92, 93, 94, 95, 95.5, 96, 96.5, 97.0, 97.5, 97.8, 98, 98.2, 98.4, 98.6,
		98.7, 98.8, 98.9, 99, 99.1, 99.2, 99.3, 99.4, 99.5, 99.6, 99.7,
		99.75, 99.78, 99.80, 99.82, 99.84, 99.86, 99.87, 99.89, 99.90, 99.91, 99.92,
		99.93, 99.94, 99.95, 99.96, 99.97, 99.98, 99.982, 99.984, 99.986, 99.987, 99.988, 99.989, 99.990,
		99.995, 99.997, 99.998, 99.999, 100}

	defaultPrintPercentils = []float64{0, 25, 50, 75, 80, 85,
		90, 95, 98, 99, 99.5, 99.8, 99.9, 100}
)

func (h LogHist) String() string {
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "Percentils:\n")
	for _, p := range defaultPrintPercentils {
		fmt.Fprintf(buf, "  %5.1f %4d\n", p, h.Percentil(p/100))
	}
	return buf.String()
}

// PercentilPlot uses gnuplot to draw latency over percentils.
// of filename is empty an ascii art is written to stdout.
func (h LogHist) PercentilPlot(filename string) {
	buf := &bytes.Buffer{}
	if filename == "" {
		fmt.Fprintln(buf, "set term dumb 85,30\n")
	} else {
		fmt.Fprintf(buf, "set term png size 800,480; set out %q\n", filename)
	}
	fmt.Fprintln(buf, `
set logscale x
set xtics ("0" 100, "50" 50, "80" 20, "90" 10, "95" 5, "98" 2, "99" 1, "99.5" 0.5, "99.8" 0.2, "99.9" 0.1, "99.95" 0.05, "99.98" 0.02, "99.99" 0.01)
set xrange [0.009:100] reverse
set yrange [0:*]
set xlab "Percentil"
set ylab "Latency [ms]"
set grid
set style data points
plot "-" using 1:2 pt 3 notit
`)
	// TODO: use only that many percentils as are reachable by the number of
	// values in h:  500 measurements will cut of the percentilplot at 99.8.
	for _, p := range defaultPlotPercentils {
		fmt.Fprintf(buf, "%.3f %d\n", 100-p, h.Percentil(p/100))
	}
	fmt.Fprintf(buf, "e\n")
	gp := &exec.Cmd{
		Path:   "/usr/bin/gnuplot",
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Stdin:  buf,
	}
	gp.Run()
}

func (h LogHist) Add(v int) {
	if v < 0 || v > h.Max {
		return
	}
	b := h.bucket(v)
	h.Count[b]++
}

func (h LogHist) MN() (maxCount int, total int) {
	for b := 0; b < h.Last; b++ {
		if h.Count[b] > maxCount {
			maxCount = h.Count[b]
		}
		total += h.Count[b]
	}
	return maxCount, total
}

func (h LogHist) dump() {
	max, n := h.MN()

	fmt.Printf("Total: %d  Max: %d\n", n, max)
	scale := 50 / float64(max)
	from, bs := -1, 0
	r := ""
	for b := 0; b < h.Last; b++ {
		from2, bs2 := h.value(b)
		if from2 != from {
			from, bs = from2, bs2
			r = fmt.Sprintf("%4d - %4d", from, from+bs-1)
		} else {
			r = "           "
		}
		bar := strings.Repeat("#", int(scale*(0.5+float64(h.Count[b]))))
		fmt.Printf("%s %s\n", r, bar)
	}
}

func (h LogHist) dumplin() {
	max, n := -1, 0
	for b := 0; b < h.Last; b++ {
		_, bs := h.value(b)
		if h.Count[b]/bs > max {
			max = h.Count[b] / bs
		}
		n += h.Count[b]
	}

	fmt.Printf("Total: %d  Max: %d\n", n, max)
	for b := 0; b < h.Last; b++ {
		from, bs := h.value(b)
		scale := 50 / float64(max)
		c := h.Count[b]
		for i := from; i < from+bs; i++ {
			bar := strings.Repeat("#", int(scale*(0.5+float64(c)/float64(bs))))
			fmt.Printf("%4d %s\n", i, bar)
		}
	}
}

// TODO: there might be a faster way...
func (h LogHist) Percentil(p float64) int {
	_, n := h.MN()
	target := p * float64(n)
	max := 0
	for b := 0; b < h.Last; b++ {
		from, bs := h.value(b)
		count := h.Count[b]
		if count == 0 {
			continue
		}
		c := float64(count) / float64(bs)
		for i := from; i < from+bs; i++ {
			target -= c
			if target < 0 {
				return i - 1
			}
			max = i
		}
	}
	return max
}

func (_ LogHist) bucketSize(v int) int {
	if v < 64 {
		return 1
	}
	bs := 2
	v /= 64
	for v > 1 {
		v >>= 1
		bs <<= 1
	}
	return bs
}

func (_ LogHist) log(b int) int {
	k := 2
	for b > 2 {
		b >>= 1
		k++
	}
	return k
}

func (h LogHist) bucket(v int) int {
	if v < 64 {
		return v
	}
	bs := h.bucketSize(v)
	v -= bs * 32
	v /= bs
	return 32*h.log(bs) + v
}

func ilog(b int) int {
	return (b - 32) / 32
}

func ibs(b int) int {
	return 1 << uint(ilog(b))
}

func (h LogHist) value(b int) (val int, bs int) {
	k := (b - 32) / 32
	bs = 1 << uint(k)
	v := b - 32 - 32*k
	v *= bs
	v += bs * 32
	if bs <= 0 {
		bs = 1
	}
	return v, bs
}
