// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suite

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/vdobler/ht/ht"
)

func logger() *log.Logger {
	if testing.Verbose() {
		return log.New(os.Stdout, "", 0)
	}

	return log.New(ioutil.Discard, "", 0)
}

// Variables in outer scopes dominate those in inner scopes.
func TestVariableDominance(t *testing.T) {
	txt := `
# dominace.suite
{
    Name: Testsuite to check variable dominance
    Variables: {
        C:  suite
        D:  suite
    }
    Main: [
        { File: "dominance.ht"
          Variables: {
              B: call
              C: call
              D: call
          }
        }
    ]
}

# dominance.ht
{
    Name: Test of variable dominance
    Request: { URL: "file:///etc/passwd" }
    Variables: {
        A: local
        B: local
        C: local
        D: local
    }   
}`

	globals := map[string]string{"D": "global"}

	rs, err := parseRawSuite("dominace.suite", txt)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	s := rs.Execute(globals, nil, logger())

	if s.Tests[0].Variables["A"] != "local" ||
		s.Tests[0].Variables["B"] != "call" ||
		s.Tests[0].Variables["C"] != "suite" ||
		s.Tests[0].Variables["D"] != "global" {
		s.PrintReport(os.Stdout)
		t.Errorf("Bad variable dominance. Got %v", s.Tests[0].Variables)
	}
}

// Variables are handed down from scope to scope. Replacement works.
func TestVariableHanddown(t *testing.T) {
	txt := `
# handdown.suite
{
    Name: Testsuite to check variable handdown
    Variables: {
        C:  "test-c",
        D:  "{{E}}"
    }
    Main: [
        { File: "handdown.ht"
          Variables: {
              A: "call-a"
              B: "{{C}}",
          }
        }
    ]
}

# handdown.ht
{
    Name: Test of variable handdown
    Request: { URL: "file:///etc/passwd" }
    Variables: {
        Va: "{{A}}"
        Vb: "{{B}}"
    }   
}`

	globals := map[string]string{"E": "global-e"}

	rs, err := parseRawSuite("handdown.suite", txt)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	s := rs.Execute(globals, nil, logger())

	if s.Tests[0].Variables["A"] != "call-a" ||
		s.Tests[0].Variables["B"] != "test-c" ||
		s.Tests[0].Variables["C"] != "test-c" ||
		s.Tests[0].Variables["D"] != "global-e" ||
		s.Tests[0].Variables["E"] != "global-e" ||
		s.Tests[0].Variables["Va"] != "call-a" ||
		s.Tests[0].Variables["Vb"] != "test-c" {
		s.PrintReport(os.Stdout)
		t.Errorf("Bad variable handdown. Got %v", s.Tests[0].Variables)
	}
}

// The automatic variables COUNTER and RANDOM are injected into global
// and call scope so that call- and test-scope have just one value of COUNTER
// and RANDOM.
func TestAutomaticVariables(t *testing.T) {
	ht.Random.Seed(1234)
	counter = 1

	txt := `
# automatic.suite
{
    Name: Testsuite for automatic variables
    Variables: {
        SuiteCount:  "{{COUNTER}}",
        SuiteRand:  "{{RANDOM}}"
    }
    Main: [
        { File: "test.ht"
          Variables: {
              CallCount: "{{COUNTER}}"
              CallRand: "{{RANDOM}}",
          }
        },
        { File: "test.ht"
          Variables: {
              CallCount: "{{COUNTER}}"
              CallRand: "{{RANDOM}}",
          }
        }
    ]
}

# test.ht
{
    Name: Test of automatic varibles
    Request: { URL: "file:///etc/passwd" }
    Variables: {
        TestCount: "{{COUNTER}}"
        TestRand: "{{RANDOM}}"
    }
}`

	globals := map[string]string{"E": "global-e"}

	rs, err := parseRawSuite("automatic.suite", txt)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	s := rs.Execute(globals, nil, logger())

	want1 := "SuiteCount=1 SuiteRand=131682 "
	want1 += "CallCount=2 CallRand=858315 "
	want1 += "TestCount=2 TestRand=858315 " // Same as Call{Count,Rand}
	want1 += "COUNTER=2 RANDOM=858315"      // Same as Call{Count,Rand}
	if wrong := matchVars(s.Tests[0].Variables, want1); wrong != "" {
		s.PrintReport(os.Stdout)
		t.Errorf("First invocation. Got %s", wrong)
	}

	want2 := "SuiteCount=1 SuiteRand=131682 " // Same as in first invocation
	want2 += "CallCount=3 CallRand=817389 "
	want2 += "TestCount=3 TestRand=817389 " // Same as Call{Count,Rand}
	want2 += "COUNTER=3 RANDOM=817389"      // Same as Call{Count,Rand}
	if wrong := matchVars(s.Tests[1].Variables, want2); wrong != "" {
		s.PrintReport(os.Stdout)
		t.Errorf("Second invocation. Got %s", wrong)
	}
}

// Variable extraction works upwards: From test-scope into suite-scope.
// RANDOM (and counter are not special).
func TestVariableExtraction(t *testing.T) {
	rand.Seed(1234)
	counter = 1

	txt := `
# extraction.suite
{
    Name: Testsuite for variable extraction
    Variables: {
        A:  "initial-A",
        B:  "B",
    }
    Main: [
        { File: "test.ht"
          Variables: {
              C: "{{A}}"
          }
        },
        { File: "test.ht"
          Variables: {
              C: "{{B}}"
          }
        }
    ]
}

# test.ht
{
    Name: Test of variable extraction
    Request: { URL: "file:///etc/passwd" }
    VarEx: {
        A: {Extractor: "SetVariable", To: "fixed-A" }
        B: {Extractor: "SetVariable", To: "{{B}} {{B}}" }
        D: {Extractor: "SetVariable", To: "D={{C}}" }
        RANDOM: {Extractor: "SetVariable", To: "abcdef" }
    }
}`

	rs, err := parseRawSuite("extraction.suite", txt)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	s := rs.Execute(nil, nil, logger())

	if s.FinalVariables["A"] != "fixed-A" ||
		s.FinalVariables["B"] != "B B B B" ||
		s.FinalVariables["D"] != "D=B B" ||
		s.FinalVariables["RANDOM"] != "abcdef" {
		s.PrintReport(os.Stdout)
		t.Errorf("Bad variable extraction. Got %v", s.FinalVariables)
	}
}

func matchVars(got map[string]string, want string) string {
	for _, elem := range strings.Split(want, " ") {
		p := strings.Split(elem, "=")
		name, val := p[0], p[1]
		if g := got[name]; g != val {
			return fmt.Sprintf("%s=%s, want %s", name, g, elem)
		}
	}

	return ""
}
