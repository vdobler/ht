// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

var sampleTestJSON = `{
    Name: "TestName",
    Description: "TestDescription",
    BasedOn: [
        "../foo/base.ht",
        "std.ht"
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
        {Check: "None", Of: [
            {Check: "Identity", SHA1: "bc86149a4f735e882f2d922eb6b778751161ac9b"},
        ]},
    ],
    VarEx: {
        VariableA: {Extractor: "BodyExtractor", Regexp: "[A-Z]+[0-9]+"},
    },
    Poll: {
        Max: 3,
        Sleep: "5432ms",
    },
    Timeout: "3.5s",
    Verbosity: 1,
    Criticality: "Warn",
    PreSleep: "11ms",
    InterSleep: "22ms",
    PostSleep: "33ms",
}`

func TestFindRawTest(t *testing.T) {
	sample := []byte(sampleTestJSON)

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
	if rt.BasedOn[0] != "../foo/base.ht" || rt.BasedOn[1] != "std.ht" {
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
	if len(rt.Checks) != 4 || len(rt.VarEx) != 1 {
		t.Errorf("Got Test == %#v", *rt)
	}
	if rt.Verbosity != 1 || rt.Criticality != CritWarn {
		t.Errorf("Got Test == %#v", *rt)
	}
	if rt.Poll.Max != 3 || rt.Poll.Sleep != Duration(5432*time.Millisecond) {
		t.Errorf("Got Test == %#v", *rt)
	}
	if rt.Timeout != Duration(3500*time.Millisecond) {
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

var baseTestJSON = `{
    Name: "Base Test",
    Request: {
        Params: {
            base: "base",
        },
        Header: {
            Base: [ "Base" ],
        },
        Cookies: [
            {Name: "basecookie", Value: "basevalue"},
        ],
    },
    Checks: [
        {Check: "StatusCode", Expect: 400},
    ],
    VarEx: {
        BaseVar: {Extractor: "BodyExtractor", Regexp: "Base"},
    },
}`

var stdTestJSON = `{
    Name: "Std Test",
    Request: {
        Header: {
            Std: [ "std" ],
        },
    },
}`

func TestRawTestToTest(t *testing.T) {
	// Populate the raw test pool with the 'base' and 'std' tests which
	// are referenced from the sample test.
	pool := make(map[string]*rawTest)
	_, _, err := findRawTest("/the/current/dir", "std.ht", pool, []byte(stdTestJSON))
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	_, _, err = findRawTest("/the/current/foo", "base.ht", pool, []byte(baseTestJSON))
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	sample := []byte(sampleTestJSON)
	rt, _, err := findRawTest("/the/current/dir", "../qux/sample.ht", pool, sample)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	tests, err := rawTestToTests("/the/current/dir", rt, pool)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	want := &Test{
		Name:        "TestName",
		Description: "Merge of TestName, Base Test, Std Test\nTestDescription",
		Request: Request{
			Method: "POST",
			URL:    "http://www.the.url",
			Params: URLValues{
				"single": []string{"singleVal"},
				"multi":  []string{"multiVal1", "multiVal2", "multiVal3"},
				"base":   []string{"base"},
			},
			ParamsAs: "ParamsAs",
			Header: http.Header{
				"Base":    []string{"Base"},
				"Std":     []string{"std"},
				"HeaderB": []string{"header B 1", "header B 2"},
				"HeaderC": []string{"header C 1", "header C 2", "header C 3"},
				"HeaderA": []string{"header A"},
			},
			Cookies: []Cookie{
				{Name: "Cookie1", Value: "CookieVal1"},
				{Name: "basecookie", Value: "basevalue"},
			},
			Body:            "RequestBody",
			FollowRedirects: true,
		},
		Checks: CheckList{
			&StatusCode{Expect: 200},
			&Header{Header: "X-Clacks-Overhead", Condition: Condition{Equals: "GNU Terry Pratchett"}},
			&HTMLTag{Selector: "div#logo_bereich div.logo", Count: 1},
			None{Of: CheckList{Identity{SHA1: "bc86149a4f735e882f2d922eb6b778751161ac9b"}}},
			&StatusCode{Expect: 400},
		},
		VarEx: map[string]Extractor{
			"BaseVar":   &BodyExtractor{Regexp: "[A-Z]+[0-9]+"},
			"VariableA": &BodyExtractor{Regexp: "Base"},
		},
		Poll: Poll{
			Max:   3,
			Sleep: 5432000000,
		},
		Timeout:     3500000000,
		Verbosity:   1,
		Criticality: 3,
		PreSleep:    11000000,
		InterSleep:  22000000,
		PostSleep:   33000000,
	}

	diff := differences(tests[0], want)

	if len(diff) != 0 {
		t.Error("Differences:\n" + strings.Join(diff, "\n"))
	}
}

func differences(t1, t2 *Test) (d []string) {
	if t1.Name != t2.Name {
		d = append(d, fmt.Sprintf("Name: %q != %q", t1.Name, t2.Name))
	}
	if t1.Description != t2.Description {
		d = append(d, fmt.Sprintf("Description: %q != %q", t1.Description, t2.Description))
	}

	if t1.Request.Method != t2.Request.Method {
		d = append(d, fmt.Sprintf("Method: %s != %s", t1.Request.Method, t2.Request.Method))
	}
	if t1.Request.URL != t2.Request.URL {
		d = append(d, fmt.Sprintf("URL: %s != %s", t1.Request.URL, t2.Request.URL))
	}
	if t1.Request.ParamsAs != t2.Request.ParamsAs {
		d = append(d, fmt.Sprintf("ParamsAs: %s != %s", t1.Request.ParamsAs, t2.Request.ParamsAs))
	}
	if t1.Request.Body != t2.Request.Body {
		d = append(d, fmt.Sprintf("Body: %s != %s", t1.Request.Body, t2.Request.Body))
	}
	if t1.Request.FollowRedirects != t2.Request.FollowRedirects {
		d = append(d, fmt.Sprintf("FollowRedirects: %t != %t",
			t1.Request.FollowRedirects, t2.Request.FollowRedirects))
	}

	d = append(d, mapToSliceDifference("Params", t1.Request.Params, t2.Request.Params)...)
	d = append(d, mapToSliceDifference("Header", t1.Request.Header, t2.Request.Header)...)
	if len(t1.Request.Cookies) != len(t2.Request.Cookies) {
		d = append(d, fmt.Sprintf("Cookies: %v != %v", t1.Request.Cookies, t2.Request.Cookies))
	} else {
		for i, c1 := range t1.Request.Cookies {
			if c2 := t2.Request.Cookies[i]; c1 != c2 {
				d = append(d, fmt.Sprintf("Cookie %d: %v != %v", i, c1, c2))
			}
		}
	}
	d = append(d, checklistDifference(t1.Checks, t2.Checks)...)

	if t1.Poll != t2.Poll {
		d = append(d, fmt.Sprintf("Poll: %v != %v", t1.Poll, t2.Poll))
	}
	if t1.Timeout != t2.Timeout {
		d = append(d, fmt.Sprintf("Timeout: %s != %s", t1.Timeout, t2.Timeout))
	}
	if t1.Verbosity != t2.Verbosity {
		d = append(d, fmt.Sprintf("Verbosity: %d != %d", t1.Verbosity, t2.Verbosity))
	}
	if t1.Criticality != t2.Criticality {
		d = append(d, fmt.Sprintf("Criticality: %d != %d", t1.Criticality, t2.Criticality))
	}
	if t1.PreSleep != t2.PreSleep {
		d = append(d, fmt.Sprintf("PreSleep: %s != %s", t1.PreSleep, t2.PreSleep))
	}
	if t1.InterSleep != t2.InterSleep {
		d = append(d, fmt.Sprintf("InterSleep: %s != %s", t1.InterSleep, t2.InterSleep))
	}
	if t1.PostSleep != t2.PostSleep {
		d = append(d, fmt.Sprintf("PostSleep: %s != %s", t1.PostSleep, t2.PostSleep))
	}

	return d
}

func mapToSliceDifference(what string, a, b map[string][]string) (d []string) {
	for an, av := range a {
		if bv, ok := b[an]; !ok {
			d = append(d, fmt.Sprintf("%s: %q-->%v missing in second", what, an, av))
		} else {
			d = append(d, sliceDifference(what+"["+an+"]", av, bv)...)
		}
	}
	return d
}

func sliceDifference(what string, a, b []string) []string {
	if len(a) == len(b) {
		for i := range a {
			if a[i] != b[i] {
				goto diffFound
			}
		}
		return nil
	}

diffFound:
	as, bs := strings.Join(a, ", "), strings.Join(b, ", ")
	return []string{fmt.Sprintf("%s: [%s] != [%s]", what, as, bs)}
}

func checklistDifference(a, b CheckList) []string {
	names := func(l CheckList) (n []string) {
		for _, check := range l {
			n = append(n, NameOf(check))
		}
		return n
	}
	aNames := names(a)
	bNames := names(b)
	diff := sliceDifference("Checks:", aNames, bNames)
	if diff != nil {
		return diff
	}

	// Selectively check StatusCodes. TODO: one more (e.g. HTMLTag)
	for i, c1 := range a {
		sc1, ok := c1.(*StatusCode)
		if !ok {
			continue
		}
		sc2, _ := b[i].(*StatusCode) // Must be StatusCode by Names.
		if sc1.Expect == sc2.Expect {
			continue
		}
		return []string{fmt.Sprintf("Check %d (StatusCode) %d != %d", i, sc1.Expect, sc2.Expect)}
	}
	return nil
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
				req := fmt.Sprintf("%#v", tr.Response.Response.Request)
				fmt.Printf("  Request\n%s\n", req)
				tr.Response.Response.Request = nil
				tr.Response.Response.TLS = nil
				resp := fmt.Sprintf("%#v", tr.Response.Response)
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
