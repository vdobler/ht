// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"math/rand"
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

func primeHandler(w http.ResponseWriter, r *http.Request) {
	n := intFormValue(r, "n")
	if n < 2 {
		n = rand.Intn(10000)
	}

	// Brute force check of primality of n.
	text := fmt.Sprintf("Number %d is prime.", n)
	for d := 2; d < n; d++ {
		if n%d == 0 {
			text = fmt.Sprintf("Number %d is NOT prime (divisor %d).", n, d)
		}
	}

	http.Error(w, text, http.StatusOK)
}

func TestLatency(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(primeHandler))
	defer ts.Close()

	for _, conc := range []int{1, 2, 4, 8, 16, 32, 64} {
		test := Test{
			Name: "Prime-Handler",
			Request: Request{
				Method: "GET",
				URL:    ts.URL + "/",
				Params: URLValues{
					"n": []string{"100000"},
				},
			},
			Timeout: Duration(100 * time.Millisecond),
			Checks: []Check{
				StatusCode{200},
				&Latency{
					N:          200 * conc,
					Concurrent: conc,
					Limits:     "50% ≤ 35ms; 75% ≤ 45ms; 0.995 ≤ 55ms",
					DumpTo:     "foo.xxx",
				},
			},
			Verbosity: 1,
		}

		test.Run(nil)

		if testing.Verbose() {
			test.PrintReport(os.Stdout)
		}
	}
}
