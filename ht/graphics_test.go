// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestRTHistogram(t *testing.T) {
	fast := []int{10, 12, 13, 14, 10, 20, 15, 17, 37, 12, 13, 15, 16, 19, 11, 14, 17, 28}
	slow := []int{34, 35, 30, 37, 38, 19, 42, 43, 41, 34, 36,
		38, 33, 32, 32, 44, 43, 45, 45, 49}

	data := map[string][]int{
		"Fast": fast,
		"Slow": slow,
	}

	err := RTHistogram("Commit deadbeef", data, true, "comparison.png")
	if err != nil {
		t.Fatalf("Unexpected error %s", err)
	}
}

func TestResponseTimeHistogram(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping response time histogram in short mode.")
	}

	ts := httptest.NewServer(http.HandlerFunc(echoHandler))
	defer ts.Close()

	test := Test{
		Name: "Page X",
		Request: Request{
			Method: "GET",
			URL:    ts.URL + "/",
			Params: url.Values{
				"text": {"Hello World!"},
				"smin": {"20"},
				"smax": {"75"},
				"fail": {"5"},
				"bad":  {"10"},
			},
			FollowRedirects: false,
		},
		Checks: []Check{
			StatusCode{200},
		},
	}

	err := test.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	// TODO: maybe some code here?
}
