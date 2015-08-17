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

func loggingHandler(w http.ResponseWriter, r *http.Request) {
	file, err := os.OpenFile("../testdata/logfile", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		fmt.Printf("%+v\n", err)
		http.Error(w, fmt.Sprintf("%#v", err), 500)
		return
	}
	defer file.Close()

	fmt.Fprintln(file, "Important log message")
	data := r.FormValue("data")
	fmt.Fprintf(file, "Data: %s\n", data)
	file.Sync()
	http.Error(w, "Everything logged", 200)
}

func TestLogfile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(loggingHandler))
	defer ts.Close()

	rand.Seed(time.Now().Unix())
	data := fmt.Sprintf("Hello %d", 1000+rand.Intn(9000))

	test := Test{
		Name: "Testing the Logfile check",
		Request: Request{
			Method: "GET",
			URL:    ts.URL + "/",
			Params: URLValues{"data": []string{data}},
		},
		Checks: []Check{
			&StatusCode{Expect: 200},                                                            // pass
			&Logfile{Path: "../testdata/logfile", Condition: Condition{Min: 10000}},             // fail
			&Logfile{Path: "../testdata/logfile", Condition: Condition{Min: 10}},                // pass
			&Logfile{Path: "../testdata/logfile", Condition: Condition{Max: 5}},                 // fail
			&Logfile{Path: "../testdata/logfile", Condition: Condition{Max: 500}},               // pass
			&Logfile{Path: "../testdata/logfile", Condition: Condition{Contains: "Stacktrace"}}, // fail
			&Logfile{Path: "../testdata/logfile", Condition: Condition{Contains: data}},         // pass
			&Logfile{Path: "../testdata/logfile", Disallow: []string{"Important"}},              // fail
			&Logfile{Path: "../testdata/logfile", Disallow: []string{"Hubba bubba"}},            // pass
		},
	}
	err := test.Run(nil)
	if err != nil {
		t.Fatalf("Unexpected error: %+v", err)
	}

	if test.Status != Fail || len(test.CheckResults) != len(test.Checks) {
		t.Fatalf("Unexpected test status %s or %d != %d", test.Status,
			len(test.CheckResults), len(test.Checks))
	}

	for i, cr := range test.CheckResults {
		if (i%2 == 0 && cr.Status != Pass) || (i%2 == 1 && cr.Status != Fail) {
			t.Errorf("%d: %s -> %s", i, cr.JSON, cr.Status)
		}
	}
}
