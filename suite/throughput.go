// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suite

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/internal/bender"
)

func makeRequest(sn int, rs *RawSuite, threads int, globals map[string]string, requests chan bender.Test, stop chan bool) {
	fmt.Printf("Starting request generation with suite %s\n", rs.Name)
	for thread := 1; thread <= threads; thread++ {
		fmt.Printf("Starting request generation with suite %s, Thread %d\n",
			rs.Name, thread)
		go func(thread int) {
			n := 1
			done := false
			for !done {
				executed := make(chan bool)

				t := 0
				executor := func(test *ht.Test) error {
					t++
					test.Name = fmt.Sprintf("%d/%d/%d/%d %s\u2237%s",
						sn, thread, n, t, rs.Name, test.Name)
					test.Reporting.SeqNo = test.Name

					if !rs.tests[t-1].IsEnabled() {
						test.Status = ht.Skipped
						return nil
					}
					select {
					case <-stop:
						done = true
						return ErrAbortExecution
					default:
					}

					requests <- bender.Test{Test: test, Done: executed}
					<-executed

					return nil
				}
				rs.Iterate(globals, nil, nil, executor)

				n += 1
			}
		}(thread)
	}
}

func Throughput(suites []*RawSuite, rate float64, duration time.Duration, variables map[string]string) []bender.TestData {
	recorder := make(chan bender.Event)
	// logger := log.New(os.Stdout, "", 0)
	// logRecorder := bender.NewLoggingRecorder(logger)
	data := make([]bender.TestData, 0, 1000)
	dataRecorder := bender.NewDataRecorder(&data)
	go bender.Record(recorder, dataRecorder)

	request := make(chan bender.Test, 10)
	stop := make(chan bool)
	for i, rs := range suites {
		go makeRequest(i+1, rs, 4, variables, request, stop)
	}
	intervals := bender.ExponentialIntervalGenerator(rate)

	bender.LoadTestThroughput(intervals, request, recorder)
	time.Sleep(duration)
	close(stop)
	close(request)
	time.Sleep(1 * time.Second)

	return data
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
		"SuiteNo",
		"ThreadNo",
		"Repetition",
		"TestNo",
		"Suite",
		"Test",
		"Error",
	}
	err := writer.Write(header)
	if err != nil {
		return err
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

		part := strings.SplitN(d.ID, " ", 2)
		r = append(r, part[0])
		P := strings.SplitN(part[0], "/", 4)
		r = append(r, P...)
		Q := strings.SplitN(part[1], "\u2237", 2)
		r = append(r, Q...)
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
