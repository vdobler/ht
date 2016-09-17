// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/vdobler/ht/cookiejar"
)

func TestQuantile(t *testing.T) {
	x := []int{1, 2, 3, 4, 5}
	want := []float64{1.000000, 1.000000, 1.400000, 1.933333, 2.466667,
		3.000000, 3.533333, 4.066667, 4.600000, 5.000000, 5.000000}
	for i, p := range []float64{0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1} {
		if got := quantile(x, p); math.Abs(got-want[i]) > 0.00001 {
			t.Errorf("quantile(1:5, %.2f, type=8): got %.ff, want %.f",
				p, got, want[i])
		}
	}

	x = []int{3, 3, 5, 6, 7, 10, 10, 12, 12, 18, 22}
	want = []float64{3.000000, 3.000000, 4.200000, 5.733333, 6.866667,
		10.000000, 10.266667, 12.000000, 14.400000, 20.133333, 22.000000}
	for i, p := range []float64{0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1} {
		if got := quantile(x, p); math.Abs(got-want[i]) > 0.00001 {
			t.Errorf("quantile(x, %.2f, type=8): got %.ff, want %.f",
				p, got, want[i])
		}
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

var (
	sessionMu      sync.Mutex
	nextSessionID  = 3414
	activeSessions = make(map[string]string)
)

// sessionHandler sleeps for a random time. The sleep times are drawn from
// a normal distribution with mean/mode and stddev linearily increasing
// with the amount of active sessions.
func sessionHandler(w http.ResponseWriter, r *http.Request) {
	sessionMu.Lock()
	var session string
	cookie, err := r.Cookie("session")
	if err != nil {
		nextSessionID++
		session = fmt.Sprintf("%d", nextSessionID)
		activeSessions[session] = session
	} else if session, ok := activeSessions[session]; ok {
		nextSessionID++
		session = fmt.Sprintf("%d", nextSessionID)
		activeSessions[session] = session
	} else {
		session = cookie.Value
	}
	noSessions := float64(len(activeSessions))
	sessionMu.Unlock()

	http.SetCookie(w, &http.Cookie{Name: "session", Value: session})
	stddev := noSessions / 1.5
	mean := noSessions * 1.5
	x := rand.NormFloat64()*stddev + mean
	s := time.Microsecond * time.Duration(2000*x)
	if s <= 0 {
		s = 0
	} else if s > 150*time.Millisecond {
		s = 150 * time.Millisecond
	}
	time.Sleep(s)
	info := fmt.Sprintf("%d Sessions. Slept %s", int(noSessions), s)
	// fmt.Println(info)
	http.Error(w, info, http.StatusOK)
}

func TestLatency(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(primeHandler))
	defer ts.Close()

	concLevels := []int{1, 4, 16}
	if !testing.Short() {
		concLevels = append(concLevels, 64)
	}

	for _, conc := range concLevels {
		test := Test{
			Name: "Prime-Handler",
			Request: Request{
				Method: "GET",
				URL:    ts.URL + "/",
				Params: url.Values{
					"n": []string{"100000"},
				},
				Timeout: Duration(100 * time.Millisecond),
			},
			Checks: []Check{
				StatusCode{200},
				&Latency{
					N:          200 * conc,
					Concurrent: conc,
					Limits:     "50% ≤ 35ms; 75% ≤ 45ms; 0.995 ≤ 55ms",
					// DumpTo:     "foo.xxx",
				},
			},
			Execution: Execution{Verbosity: 0},
		}
		if *verboseTest {
			test.Execution.Verbosity = 1
		}
		test.Run()

		if *verboseTest {
			test.PrintReport(os.Stdout)
		}
	}
}

func TestSessionLatency(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(sessionHandler))
	defer ts.Close()

	concLevels := []int{1, 4}
	if !testing.Short() {
		concLevels = append(concLevels, 16)
	}

	for _, kind := range []string{"indiv", "shared"} {
		for _, conc := range concLevels {
			sessionMu.Lock()
			activeSessions = make(map[string]string)
			sessionMu.Unlock()
			jar, _ := cookiejar.New(nil)

			medianLimit := "5ms"
			if kind == "indiv" {
				medianLimit = fmt.Sprintf("%dms", (conc-1)*2+5)
			}

			test := Test{
				Name: kind,
				Request: Request{
					URL:     ts.URL + "/",
					Timeout: Duration(500 * time.Millisecond),
				},
				Checks: []Check{
					StatusCode{200},
					&Latency{
						N:          200 * conc,
						Concurrent: conc,
						Limits:     "50% ≤ " + medianLimit,
						// DumpTo:             "sessionlatency",
						IndividualSessions: kind == "indiv",
					},
				},
				Execution: Execution{Verbosity: 0},
				Jar:       jar,
			}
			if *verboseTest {
				test.Execution.Verbosity = 1
			}

			test.Run()

			// shared tests pass, indiv fail
			if kind == "shared" && test.Status != Pass {
				test.PrintReport(os.Stdout)
				t.Errorf("Unexpected failure for shared sessions")
			} else if kind == "indiv" && test.Status != Fail {
				test.PrintReport(os.Stdout)
				t.Errorf("Missing failure for indiv sessions")
			}
		}
	}
}
