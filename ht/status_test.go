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
)

func TestStatusCode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(echoHandler))
	defer ts.Close()

	for _, code := range []int{200, 201, 204, 300, 400, 404, 500} {
		s := fmt.Sprintf("%d", code)
		test := Test{
			Name: "A very basic test.",
			Request: Request{
				Method:          "GET",
				URL:             ts.URL + "/",
				Params:          URLValues{"status": []string{s}},
				FollowRedirects: false,
			},
			Checks: []Check{
				StatusCode{Expect: code},
			},
		}

		test.Run(nil)
		if test.Status != Pass {
			t.Errorf("Unexpected error for %d: %s", code, test.Error)
		}
		if testing.Verbose() {
			test.PrintReport(os.Stdout)
		}
	}
}

func TestNoServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(echoHandler))
	defer ts.Close()

	for _, code := range []int{200, 201, 204, 300, 400, 404, 500, 501, 503, 520, 599} {
		s := fmt.Sprintf("%d", code)
		test := Test{
			Name: "A very basic test.",
			Request: Request{
				Method:          "GET",
				URL:             ts.URL + "/",
				Params:          URLValues{"status": []string{s}},
				FollowRedirects: false,
			},
			Checks: []Check{NoServerError{}},
		}

		test.Run(nil)

		if code >= 500 {
			if test.Status != Fail {
				t.Errorf("Missing failure for %d", code)
			}

		} else {
			if test.Status != Pass {
				t.Errorf("Unexpected error for %d: %s", code, test.Error)
			}
		}
		if testing.Verbose() {
			test.PrintReport(os.Stdout)
		}
	}
}
