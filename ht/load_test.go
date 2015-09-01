// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/kr/pretty"
)

func TestFindRawTest(t *testing.T) {
	sample := []byte(`
{
    Name: "TestName",
    Description: "TestDescription",
    BasedOn: [
        "../foo/base1",
        "base2"
    ],
    Request: {
        Method: "POST",
        URL: "http://www.the.url",
        Params: {
            single: "singleVal",
            multi: [ "multiVal1", "multiVal2", "multiVal3" ],
        },
        ParamsAs: "ParamsAs",
        Header: {
            HeaderA: [ "header A" ],
            HeaderB: [ "header B 1", "header B 2" ],
            HeaderC: [ "header C 1", "header C 2", "header C 3" ],
        },
        Cookies: [
            {Name: "Cookie1", Value: "CookieVal1"},
        ],
        Body: "RequestBody",
        FollowRedirects: true,
    },
    Checks: [
        {Check: "StatusCode", Expect: 200},
        {Check: "Header", Header: "X-Clacks-Overhead", Equals: "GNU Terry Pratchett"},
	{Check: "HTMLTag", Selector: "div#logo_bereich div.logo", Count: 1},
    ],
    VarEx: {
        VariableA: {Extractor: "BodyExtractor", Regexp: "[A-Z]+[0-9]+"},
    },
    Poll: {
        Max: 3,
        Sleep: "5432ms",
    },
    Verbosity: 1,
    Criticality: "Warn",
    PreSleep: "11ms",
    InterSleep: "22ms",
    PostSleep: "33ms",
}
`)

	pool := make(map[string]*rawTest)

	rt, basedir, err := findRawTest("/the/current/dir", "../qux/sample.ht", pool, sample)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	if basedir != "/the/current/qux" {
		t.Errorf("Got basedir=%q, want /the/current/qux")
	}

	// Check some simple values
	if rt.Name != "TestName" || rt.Description != "TestDescription" {
		t.Errorf("Got Test == %#v", *rt)
	}
	if rt.BasedOn[0] != "../foo/base1" || rt.BasedOn[1] != "base2" {
		t.Errorf("Got Test == %#v", *rt)
	}
	if rt.Request.Method != "POST" || rt.Request.URL != "http://www.the.url" {
		t.Errorf("Got Test == %#v", *rt)
	}
	if rt.Request.ParamsAs != "ParamsAs" || rt.Request.Body != "RequestBody" {
		t.Errorf("Got Test == %#v", *rt)
	}
	if len(rt.Request.Params) != 2 || len(rt.Request.Header) != 3 {
		t.Errorf("Got Test == %#v", *rt)
	}
	if len(rt.Request.Cookies) != 1 || rt.Request.FollowRedirects != true {
		t.Errorf("Got Test == %#v", *rt)
	}
	if len(rt.Checks) != 3 || len(rt.VarEx) != 1 {
		t.Errorf("Got Test == %#v", *rt)
	}
	if rt.Verbosity != 1 || rt.Criticality != CritWarn {
		t.Errorf("Got Test == %#v", *rt)
	}
	if rt.Poll.Max != 3 || rt.Poll.Sleep != Duration(5432*time.Millisecond) {
		t.Errorf("Got Test == %#v", *rt)
	}
	if rt.PreSleep != Duration(11*time.Millisecond) {
		t.Errorf("Got Test == %#v", *rt)
	}
	if rt.InterSleep != Duration(22*time.Millisecond) {
		t.Errorf("Got Test == %#v", *rt)
	}
	if rt.PostSleep != Duration(33*time.Millisecond) {
		t.Errorf("Got Test == %#v", *rt)
	}

	if rt.Request.Params["single"][0] != "singleVal" {
		t.Errorf("Got Test == %#v", *rt)
	}
	if rt.Request.Params["multi"][2] != "multiVal3" {
		t.Errorf("Got Test == %#v", *rt)
	}
	if rt.Request.Header["HeaderA"][0] != "header A" {
		t.Errorf("Got Test == %#v", *rt)
	}
	if rt.Request.Header["HeaderB"][1] != "header B 2" {
		t.Errorf("Got Test == %#v", *rt)
	}
	if rt.Request.Header["HeaderC"][2] != "header C 3" {
		t.Errorf("Got Test == %#v", *rt)
	}
	if rt.Request.Cookies[0].Name != "Cookie1" || rt.Request.Cookies[0].Value != "CookieVal1" {
		t.Errorf("Got Test == %#v", *rt)
	}

}

func differences(t1, t2 *Test) (d []string) {
	if t1.Name != t2.Name {
		d = append(d, fmt.Sprintf("Name: %q != %q", t1.Name, t2.Name))
	}
	if t1.Description != t2.Description {
		d = append(d, fmt.Sprintf("Description: %q != %q", t1.Description, t2.Description))
	}
	return d
}

func TestLoadSuite(t *testing.T) {
	suite, err := LoadSuite("../testdata/sample.suite")
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}

	if testing.Verbose() {
		suite.Log = log.New(os.Stdout, "", log.LstdFlags)
	} else {
		for _, test := range suite.AllTests() {
			test.Verbosity = 0
		}
	}

	err = suite.Prepare()
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
	if testing.Short() {
		t.Skip("Skipping execution without network in short mode.")
	}

	if testing.Short() {
		return
	}
	suite.Execute()
	if suite.Status != Pass {
		for _, tr := range suite.AllTests() {
			if tr.Status == Pass || !testing.Verbose() {
				continue
			}
			fmt.Println("Test", tr.Name)
			if tr.Error != nil {
				fmt.Println("  Error: ", tr.Error)
			} else {
				for _, cr := range tr.CheckResults {
					if cr.Status == Pass {
						continue
					}
					fmt.Println("  Fail: ", cr.Name, cr.JSON, cr.Status, cr.Error)
				}
			}
			if tr.Response.Response != nil &&
				tr.Response.Response.Request != nil {
				tr.Response.Response.Request.TLS = nil
				req := pretty.Sprintf("% #v", tr.Response.Response.Request)
				fmt.Printf("  Request\n%s\n", req)
				tr.Response.Response.Request = nil
				tr.Response.Response.TLS = nil
				resp := pretty.Sprintf("% #v", tr.Response.Response)
				fmt.Printf("  Response\n%s\n", resp)
			}
		}
	}

	if testing.Verbose() {
		fmt.Printf("\nDefault Text Output:\n")
		suite.PrintReport(os.Stdout)
		junit, err := suite.JUnit4XML()
		if err != nil {
			t.Fatalf("Unexpected error: %+v", err)
		}
		fmt.Printf("\nJUnit 4 XML Output:\n%s", junit)
		sr := NewSuiteResult()
		sr.Account(suite, true, true)
		fmt.Println(sr.Matrix())
		fmt.Printf("Default KPI: %.3f   JustBad KPI: %.3f    KPI: %.3f\n",
			sr.KPI(DefaultPenaltyFunc), sr.KPI(JustBadPenaltyFunc),
			sr.KPI(AllWrongPenaltyFunc))
	}
}

func TestLoadSuiteComplicated(t *testing.T) {
	suite, err := LoadSuite("testdata/suite.suite")
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}

	if n := len(suite.Setup); n != 1 {
		t.Errorf("Got %d setup tests, want 1", n)
	}

	if n := len(suite.Tests); n != 3 {
		t.Errorf("Got %d setup tests, want 1. Got %+v", n, suite.Tests)
	}

	if n := len(suite.Teardown); n != 1 {
		t.Errorf("Got %d teardown tests, want 1", n)
	}

}
