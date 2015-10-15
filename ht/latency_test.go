// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestQuantile(t *testing.T) {
	x := []int{1, 2, 3, 4, 5}

	for _, p := range []float64{0.5, 0.25, 0.75, 0, 1} {
		fmt.Printf("p=%.02f q=%d\n", p, quantile(x, p))
	}
}

func TestLatency(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(echoHandler))
	defer ts.Close()

	for _, conc := range []int{1, 2, 4, 8, 16, 32, 64, 128} {
		fmt.Printf("Conccurency = %d\n", conc)
		test := Test{
			Name: "Echo-Handler",
			Request: Request{
				Method: "GET",
				URL:    ts.URL + "/",
				Params: URLValues{
					"smin": []string{"10"},
					"smax": []string{"50"},
					"fail": {"1"},
				},
			},
			Timeout: Duration(100 * time.Millisecond),
			Checks: []Check{
				StatusCode{200},
				&Latency{
					N:          100 * conc,
					Concurrent: conc,
					Limits:     "50% < 35ms; 75% < 45ms; 90% < 55ms",
					DumpTo:     "foo.xxx",
				},
			},
		}

		test.Run(nil)
		if test.Status != Pass {
			t.Errorf("Unexpected error %s", test.Error)
		}
		if testing.Verbose() {
			test.PrintReport(os.Stdout)
		}

	}
}
