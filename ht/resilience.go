// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// resilience.go contains resilience checking.

package ht

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func init() {
	RegisterCheck(Resilience{})
}

// ----------------------------------------------------------------------------
// Resilience

// Resilience checks the resilience of an URL against unexpected requests:
//  - Different HTTP methods (GET, POST and HEAD)
//  - Dropped, garbled and doubled HTTP header fields
//  - Dropped, garbled and doubled parameters
//  - Different parameter transmissions (query parameters,
//    application/x-www-form-urlencoded and multipart/form-data)
// This check will make a wast amount of request to the given URL including
// the modifying and non-idempotent methods POST, PUT, and DELETE. Some
// care using this check is advisable.
type Resilience struct {
	AllMethods bool // Test PUT, DELETE, OPTIONS and TRACE too.
}

// Execute implements Check's Execute method.
func (r Resilience) Execute(t *Test) error {
	suite := &Suite{
		Name: "Autogenerated Resilience Checking Suite",
	}

	methods := []string{"GET", "POST", "HEAD"}
	if r.AllMethods {
		methods = append(methods, []string{"PUT", "DELETE", "OPTIONS", "TRACE"}...)
	}

	for _, method := range methods {
		// Just an other method.
		if method != t.Request.Method {
			suite.Tests = append(suite.Tests, r.makeTest(t, method, "", "", modNone, modNone))
		}

		// Fiddle with each header field.
		for h := range t.Request.Header {
			suite.Tests = append(suite.Tests, r.makeTest(t, method, h, "", modDrop, modNone))
			suite.Tests = append(suite.Tests, r.makeTest(t, method, h, "", modGarble, modNone))
			suite.Tests = append(suite.Tests, r.makeTest(t, method, h, "", modDouble, modNone))
		}

		// Fiddle with each parameter.
		for p := range t.Request.Params {
			suite.Tests = append(suite.Tests, r.makeTest(t, method, "", p, modNone, modDrop))
			suite.Tests = append(suite.Tests, r.makeTest(t, method, "", p, modNone, modGarble))
			suite.Tests = append(suite.Tests, r.makeTest(t, method, "", p, modNone, modDouble))
		}

		// Drop all headers and all parameters.
		test := resilienceTest(t, method)
		test.Name = fmt.Sprintf("%s without header or parameters", method)
		test.Request.Header, test.Request.Params = nil, nil
		suite.Tests = append(suite.Tests, test)

		// Fiddle with type of parameter transmission.
		if len(t.Request.Params) > 0 && (method == "POST" || method == "PUT" || method == "DELETE") {
			for _, pa := range []string{"URL", "body", "multipart"} {
				test := resilienceTest(t, method)
				test.Request.ParamsAs = pa
				test.Name = fmt.Sprintf("%s with parameters as %s", method, pa)
				suite.Tests = append(suite.Tests, test)
			}
		}
	}

	t.infof("Start of resilience suite")
	suite.Execute()
	t.infof("End of resilience suite")
	if suite.Status != Pass {
		failures := []string{}
		for _, t := range suite.Tests {
			if t.Status != Pass {
				failures = append(failures, t.Name)
			}
		}
		collected := strings.Join(failures, "; ")
		return errors.New(collected)
	}

	return nil
}

type modification uint32

const (
	modNone modification = iota
	modDrop
	modGarble
	modDouble
)

func modify(m map[string][]string, key string, mod modification) string {
	descr := ""

	switch mod {
	case modDrop:
		descr += fmt.Sprintf(" dropped %s", key)
		delete(m, key)
	case modGarble:
		descr += fmt.Sprintf(" garbled %s", key)
		m[key] = []string{"p,f1u;p5c:h*"} // arbitary garbage
	case modDouble:
		descr += fmt.Sprintf(" doubled %s", key)
		m[key] = append(m[key], "p,f1u;p5c:h*") // arbitary garbage
	}

	return descr
}

// resilienceTest makes a copy of orig. The copy uses the given method
// and has just one check, a No ServerError. Header fields and parameters
// are deep copied. The actual set of cookies is copied from the jar.
func resilienceTest(orig *Test, method string) *Test {
	cpy := &Test{
		Name: method,
		Request: Request{
			Method:          method,
			URL:             orig.Request.URL,
			FollowRedirects: false,
		},
		Verbosity: orig.Verbosity - 1,
		PreSleep:  Duration(10 * time.Millisecond),
	}

	cpy.Request.Header = make(http.Header)
	for h, v := range orig.Request.Header {
		vc := make([]string, len(v))
		copy(vc, v)
		cpy.Request.Header[h] = vc
	}

	cpy.Request.Params = make(URLValues)
	for p, v := range orig.Request.Params {
		vc := make([]string, len(v))
		copy(vc, v)
		cpy.Request.Params[p] = vc
	}
	cpy.Request.ParamsAs = orig.Request.ParamsAs
	if cpy.Request.ParamsAs == "" {
		cpy.Request.ParamsAs = "URL" // the default if empty
	}

	cpy.Checks = CheckList{
		NoServerError{},
	}

	cpy.PopulateCookies(orig.Jar, orig.Request.Request.URL)

	return cpy
}

// makeTests produces a copy of t and applies the modifications hmod and pmod
// to the header field h and the parameter p.
func (r Resilience) makeTest(t *Test, method string, h, p string, hmod, pmod modification) *Test {
	test := resilienceTest(t, method)

	hmd := modify(map[string][]string(test.Request.Header), h, hmod)
	pmd := modify(map[string][]string(test.Request.Params), p, pmod)

	if hmd != "" {
		test.Name += " Header:" + hmd
	}
	if pmd != "" {
		test.Name += " Parameter:" + pmd
	}

	return test
}

// Prepare implements Check's Prepare method.
func (r Resilience) Prepare() error { return nil }