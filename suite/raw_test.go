// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suite

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/vdobler/ht/ht"
)

func TestNewFile(t *testing.T) {
	raw, err := LoadFile("./testdata/../testdata/a.ht")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if got := raw.Name; got != "testdata/a.ht" {
		t.Errorf("Bad Name, got %q want %q", got, "testdata/a.ht")
	}
	if strings.Index(raw.Data, "aaa.aaa.aaa") == -1 {
		t.Errorf("Bad Data; got %q", raw.Data)
	}
}

func TestNewRawTest(t *testing.T) {
	raw, err := LoadRawTest("./testdata/b.ht")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if len(raw.Mixins) != 1 {
		t.Errorf("got %d mixins", len(raw.Mixins))
	} else if got := raw.Mixins[0].Name; got != "testdata/m.mix" {
		t.Errorf("bad mixin, got %q, want %q", got, "testdata/m.mix")
	}
}

func TestRawErrorReporting(t *testing.T) {
	_, err := LoadRawTest("./testdata/wrong.ht")
	if err == nil {
		t.Fatalf("no error")
	}
	want := "file testdata/wrong.ht not valid hjson: Found a punctuator character '}' when expecting a quoteless string (check your syntax) at line 9,15 >>>     Checks: [ } }\n"
	if got := err.Error(); got != want {
		t.Errorf("\nGot:  %q\nWant: %q", got, want)
	}
}

func TestErrorReporting(t *testing.T) {
	raw, err := LoadRawTest("./testdata/wrong2.ht")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	_, err = raw.ToTest(nil)
	if err == nil {
		t.Fatalf("no error")
	}
	want := "cannot produce Test from testdata/wrong2.ht: unknown field FollowAllRedirects in Test.Request"
	if got := err.Error(); got != want {
		t.Errorf("Got:  %q\n,Want: %q", got, want)
	}

}

func TestRawTestToTest(t *testing.T) {
	raw, err := LoadRawTest("./testdata/a.ht")
	if err != nil {
		t.Fatalf("Unexpected error %s", err)
	}

	variables := map[string]string{
		"VAR_B": "zulu",
	}
	testScope := newScope(variables, raw.Variables, false)
	test, err := raw.ToTest(testScope)
	if err != nil {
		t.Fatalf("Unexpected error %s", err)
	}

	// Basic Stuff
	if test.Name != "Test A" {
		t.Errorf("Got Name = %q", test.Name)
	}
	if got := test.Request.URL; got != "http://aaa.aaa.aaa" {
		t.Errorf("Got Request.URL = %q", got)
	}
	if got := test.Request.Header.Get("Multi"); got != "A" {
		t.Errorf("Got Request.Header[Multi] = %q", got)
	}

	// Checks and Extractions
	if len(test.Checks) != 1 || len(test.VarEx) != 1 {
		t.Errorf("Got %d checks and %d extractions", len(test.Checks), len(test.VarEx))
	} else {
		if sc, ok := test.Checks[0].(*ht.StatusCode); !ok {
			t.Errorf("Bad type %T", test.Checks[0])
		} else {
			if got := sc.Expect; got != 200 {
				t.Errorf("Got StatusCode.Expect = %d", got)
			}
		}
		if ex, ok := test.VarEx["WAZ"].(*ht.JSONExtractor); !ok {
			t.Errorf("Bad type %T", test.VarEx["WAZ"])
		} else {
			if got := ex.Path; got != "foo.bar.zip" {
				t.Errorf("Got (VarEx[WAZ].(JSONExtractor)).Path = %q", got)
			}
		}
	}

	// Proper variable substitutions
	if got := test.Description; got != "Descr: vala zulu" {
		t.Errorf("Got Description = %q", got)
	}
}

func TestNewRawSuite(t *testing.T) {
	raw, err := LoadRawSuite("./testdata/suite.suite")
	if err != nil {
		t.Fatalf("Unexpected error %s", err)
	}
	fmt.Printf("%#v\n", raw)
	pp("RawSuite", raw)
	if len(raw.RawTests()) != 5 {
		panic(len(raw.RawTests()))
	}
}

func TestFancySuite(t *testing.T) {
	raw, err := LoadRawSuite("./testdata/fancy.suite")
	if err != nil {
		t.Fatalf("Unexpected error %s", err)
	}
	if testing.Verbose() {
		pp("FancSuite", raw)
	}
}

func TestRawSuiteExecute(t *testing.T) {
	which := "./testdata/suite.suite"
	which = "../showcase/showcase.suite"
	// which = "./testdata/fancy.suite"
	raw, err := LoadRawSuite(which)
	if err != nil {
		t.Fatalf("Unexpected error %s", err)
	}
	// pp("Raw", raw)
	for i, test := range raw.RawTests() {
		fmt.Printf("%d. %q\n", i, test.Name)
	}

	vars := map[string]string{
		"HOST":   "localhost:8080",
		"DOMAIN": "localhost:9090",
	}

	s := raw.Execute(vars, nil, logger())
	fmt.Println("STATUS ==", s.Status, s.Error)
	// pp("Suite", s)

	err = PrintSuiteReport(os.Stdout, s)
	if err != nil {
		t.Fatalf("Unexpected error %s", err)
	}

	for i, test := range s.Tests {
		fmt.Printf("%d. %q ==> %s (%v)\n", i, test.Name, test.Status, test.Error)
	}

	err = HTMLReport(".", s)
	if err != nil {
		t.Fatalf("Unexpected error %s", err)
	}

}

// ----------------------------------------------------------------------------
// CheckLists and ExtractorMap

func TestChecklist(t *testing.T) {
	rt := &RawTest{
		File: &File{
			Name: "<internal>",
			Data: `{
    Name: CheckList Test
    Checks: [
        {Check: "StatusCode", Expect: 404}
        {Check: "Body", Contains: "foobar" }
        {Check: "UTF8Encoded"}
        {Check: "None",
            Of: [
                {Check: "StatusCode", Expect: 303}
                {Check: "Body", Contains: "helloworld" }
                {Check: "UTF8Encoded"}
            ]
        }
    ]
    VarEx: {
        NAME: {Extractor: "JSONExtractor", Path: "foo.1"}
        SESSION: {Extractor: "CookieExtractor", Name: "JSESSIONID"} 
    }
}`,
		},
	}

	test, err := rt.ToTest(nil)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if len(test.Checks) != 4 {
		t.Fatalf("Got %d checks, want 4", len(test.Checks))
	}

	// Check 0
	statuscode, ok := test.Checks[0].(*ht.StatusCode)
	if !ok {
		t.Errorf("Wrong type, got %T", test.Checks[0])
	} else if statuscode.Expect != 404 {
		t.Errorf("Got %d, want 404", statuscode.Expect)
	}

	// Check 1
	body, ok := test.Checks[1].(*ht.Body)
	if !ok {
		t.Errorf("Wrong type, got %T", test.Checks[1])
	} else if body.Contains != "foobar" {
		t.Errorf("Got %q, want foobar", body.Contains)
	}

	// Check 2
	utf8, ok := test.Checks[2].(*ht.UTF8Encoded)
	if !ok {
		t.Errorf("Wrong type, got %T", test.Checks[2])
	} else if utf8 == nil {
		t.Error("Got nil")
	}

	// Check 3
	none, ok := test.Checks[3].(*ht.None)
	if !ok {
		t.Errorf("Wrong type, got %T", test.Checks[3])
	} else {
		if len(none.Of) != 3 {
			t.Error("Got %d, want 3", len(none.Of))
		}

		// NoneOf 0
		statuscode, ok := none.Of[0].(*ht.StatusCode)
		if !ok {
			t.Errorf("Wrong type, got %T", none.Of[0])
		} else if statuscode.Expect != 303 {
			t.Errorf("Got %d, want 303", statuscode.Expect)
		}

		// NoneOf 1
		body, ok := none.Of[1].(*ht.Body)
		if !ok {
			t.Errorf("Wrong type, got %T", none.Of[1])
		} else if body.Contains != "helloworld" {
			t.Errorf("Got %q, want helloworld", body.Contains)
		}

		// NoneOf 2
		utf8, ok := none.Of[2].(*ht.UTF8Encoded)
		if !ok {
			t.Errorf("Wrong type, got %T", none.Of[2])
		} else if utf8 == nil {
			t.Error("Got nil")
		}
	}

	enc, err := test.Checks.MarshalJSON()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	want := `[{"Check":"StatusCode","Expect":404}, {"Check":"Body","Contains":"foobar"}, {"Check":"UTF8Encoded"}, {"Check":"None","Of":[{"Check":"StatusCode","Expect":303},{"Check":"Body","Contains":"helloworld"},{"Check":"UTF8Encoded"}]}]`
	if string(enc) != want {
		t.Errorf("got  %s\nwat %s", enc, want)
	}

	//
	// Extractions
	//

	if len(test.VarEx) != 2 {
		t.Fatalf("Got %d extractions, want 2", len(test.VarEx))
	}

	// NAME
	json, ok := test.VarEx["NAME"].(*ht.JSONExtractor)
	if !ok {
		t.Errorf("Wrong type, got %T", test.VarEx["NAME"])
	} else if json.Path != "foo.1" {
		t.Errorf("Got %s, want foo.1", json.Path)
	}

	// SESSION
	cookie, ok := test.VarEx["SESSION"].(*ht.CookieExtractor)
	if !ok {
		t.Errorf("Wrong type, got %T", test.VarEx["SESSION"])
	} else if cookie.Name != "JSESSIONID" {
		t.Errorf("Got %s, want JSESSIONID", cookie.Name)
	}
}
