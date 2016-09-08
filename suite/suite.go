// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suite

import (
	"fmt"
	"strings"
	"time"

	"github.com/vdobler/ht/cookiejar"
	"github.com/vdobler/ht/ht"
)

// A Suite is a collection of Tests after execution.
type Suite struct {
	Name        string
	Description string
	KeepCookies bool

	Status   ht.Status
	Error    error
	Started  time.Time
	Duration time.Duration

	Tests []*ht.Test

	Variables      map[string]string
	FinalVariables map[string]string
	Jar            *cookiejar.Jar
}

// Execute the raw suite rs and capture the outcome in a Suite.
func (rs *RawSuite) Execute(variables map[string]string, jar *cookiejar.Jar) *Suite {
	fmt.Println("Execute suite", rs.Name)
	ppvars("Variables to Execute", variables)

	// Create cookie jar if needed.
	if rs.KeepCookies {
		if jar == nil {
			// Make own, private-use jar.
			jar, _ = cookiejar.New(nil)
		}
	} else {
		jar = nil
	}

	varset := mergeVariables(variables, rs.Variables)
	replacer := VarReplacer(varset)

	suite := &Suite{
		Name:        replacer.Replace(rs.Name),
		Description: replacer.Replace(rs.Description),
		KeepCookies: rs.KeepCookies,

		Status:   ht.NotRun,
		Error:    nil,
		Started:  time.Now(),
		Duration: 0,

		Tests: make([]*ht.Test, len(rs.Tests)),

		Variables:      make(map[string]string),
		FinalVariables: make(map[string]string),
		Jar:            jar,
	}
	for n, v := range varset {
		suite.Variables[n] = v
	}

	setup, main := len(rs.Setup), len(rs.Main)
	execute := true

	for i, rt := range rs.Tests {
		fmt.Printf("Executing Test %q (%s)\n", rt.Name, rt.File.Name)
		test, err := rt.ToTest(varset)

		// Sorry...
		if i < setup {
			test.Reporting.SeqNo = fmt.Sprintf("Setup %d", i+1)
		} else if i < setup+main {
			test.Reporting.SeqNo = fmt.Sprintf("Main %d", i-setup+1)
		} else {
			test.Reporting.SeqNo = fmt.Sprintf("Teardown %d", i-setup-main+1)
		}

		if err != nil {
			test.Status = ht.Bogus
			test.Error = err
		} else {
			if execute {
				test.Jar = jar
				test.Verbosity++
				test.Run(varset)
				// TODO: copy variables for reference
				if i < setup+main {
					updateSuite(test, suite)
				}
				updateVariables(test, varset)
			} else {
				test.Status = ht.Skipped
			}
		}
		fmt.Printf("%s test %q (%s)",
			strings.ToUpper(test.Status.String()), test.Name, rt.File.Name)
		if test.Status > ht.Pass {
			fmt.Println(" ==> ", test.Error)
		} else {
			fmt.Println()
		}
		suite.Tests[i] = test

		if i < setup && test.Status > ht.Pass {
			execute = false
		}
	}
	suite.Duration = time.Since(suite.Started)
	for n, v := range varset {
		suite.FinalVariables[n] = v
	}

	return suite
}

// Stats counts the test results of s.
func (s *Suite) Stats() (notRun int, skipped int, passed int, failed int, errored int, bogus int) {
	for _, tr := range s.Tests {
		switch tr.Status {
		case ht.NotRun:
			notRun++
		case ht.Skipped:
			skipped++
		case ht.Pass:
			passed++
		case ht.Fail:
			failed++
		case ht.Error:
			errored++
		case ht.Bogus:
			bogus++
		default:
			panic(fmt.Sprintf("No such Status %d in suite %q test %q",
				tr.Status, s.Name, tr.Name))
		}
	}
	return
}
