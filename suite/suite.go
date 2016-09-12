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

func shouldRun(t int, rs *RawSuite, s *Suite) bool {
	// Stop execution on errors during setup
	for i := 0; i < len(rs.Setup) && i < len(s.Tests); i++ {
		if s.Tests[i].Status > ht.Pass {
			return false
		}
	}
	return true
}

// Execute the raw suite rs and capture the outcome in a Suite.
func (rs *RawSuite) Execute(variables map[string]string, jar *cookiejar.Jar) *Suite {
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

		Tests: make([]*ht.Test, 0, len(rs.tests)),

		Variables:      make(map[string]string),
		FinalVariables: make(map[string]string),
		Jar:            jar,
	}
	for n, v := range varset {
		suite.Variables[n] = v
	}

	setup, main := len(rs.Setup), len(rs.Main)

	for i, rt := range rs.tests {
		fmt.Printf("Executing Test %q (%s)\n", rt.Name, rt.File.Name)
		testvarset := mergeVariables(varset, rt.contextVars)
		test, err := rt.ToTest(testvarset)

		if i < setup {
			test.Reporting.SeqNo = fmt.Sprintf("Setup-%02d", i+1)
		} else if i < setup+main {
			test.Reporting.SeqNo = fmt.Sprintf("Main-%02d", i-setup+1)
		} else {
			test.Reporting.SeqNo = fmt.Sprintf("Teardown-%02d", i-setup-main+1)
		}

		if err != nil {
			test.Status = ht.Bogus
			test.Error = err
		} else if shouldRun(i, rs, suite) {
			test.Jar = jar
			test.Run()
			if i < setup+main {
				updateSuite(test, suite)
			}
			updateVariables(test, varset)
		} else {
			test.Status = ht.Skipped
		}

		fmt.Printf("%s test %q (%s)",
			strings.ToUpper(test.Status.String()), test.Name, rt.File.Name)
		if test.Status > ht.Pass {
			fmt.Println(" ==> ", test.Error)
		} else {
			fmt.Println()
		}
		suite.Tests = append(suite.Tests, test)
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
