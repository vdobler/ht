// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"sync"
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

var (
	sessionMu      sync.Mutex
	nextSessionID  int               = 3414
	activeSessions map[string]string = make(map[string]string)
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

	for _, conc := range []int{1, 4, 16, 64} {
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
					// DumpTo:     "foo.xxx",
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

func TestSessionLatency(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(sessionHandler))
	defer ts.Close()

	for _, kind := range []string{"indiv", "shared"} {
		for _, conc := range []int{1, 4, 16} {
			sessionMu.Lock()
			activeSessions = make(map[string]string)
			sessionMu.Unlock()
			jar, _ := cookiejar.New(nil)

			medianLimit := "5ms"
			if kind == "indiv" {
				medianLimit = fmt.Sprintf("%dms", (conc-1)*2+5)
			}

			test := Test{
				Name:    kind,
				Request: Request{URL: ts.URL + "/"},
				Timeout: Duration(500 * time.Millisecond),
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
				Verbosity: 1,
				Jar:       jar,
			}

			test.Run(nil)

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
