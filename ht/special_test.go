// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// ----------------------------------------------------------------------------
// file:// pseudo request.

func TestFileSchema(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	u := "file://" + wd + "/testdata/fileprotocol"

	tests := []*Test{
		&Test{
			Name: "PUT Pass",
			Request: Request{
				URL:  u,
				Body: "Tadadadaaa!",
			},
			Checks: []Check{
				StatusCode{Expect: 200},
				&Body{Prefix: "Successfully wrote " + u},
				&Header{Header: "Foo", Absent: true},
			},
		},
		&Test{
			Name: "PUT Error",
			Request: Request{
				URL:  u + "/iouer/cxxs/dlkfj",
				Body: "Tadadadaaa!",
			},
			Checks: []Check{
				StatusCode{Expect: 200},
				&Body{Prefix: "Successfully wrote " + u},
				&Header{Header: "Foo", Absent: true},
			},
		},
		&Test{
			Name: "GET Pass",
			Request: Request{
				URL: u,
			},
			Checks: []Check{
				StatusCode{Expect: 200},
				&Body{Equals: "Tadadadaaa!"},
				&Header{Header: "Foo", Absent: true},
			},
		},
		&Test{
			Name: "GET Fail",
			Request: Request{
				URL: u,
			},
			Checks: []Check{
				StatusCode{Expect: 200},
				&Body{Equals: "something else"},
				&Header{Header: "Foo", Absent: true},
			},
		},
		&Test{
			Name: "GET Error",
			Request: Request{
				URL: u + "/slkdj/cxmvn",
			},
			Checks: []Check{
				StatusCode{Expect: 200},
				&Header{Header: "Foo", Absent: true},
			},
		},
		&Test{
			Name: "GET Error",
			Request: Request{
				URL: "file://remote.host/some/path",
			},
			Checks: []Check{
				StatusCode{Expect: 200},
				&Header{Header: "Foo", Absent: true},
			},
		},
		&Test{
			Name: "DELETE Pass",
			Request: Request{
				URL: u,
			},
			Checks: []Check{
				StatusCode{Expect: 200},
				&Body{Prefix: "Successfully deleted " + u},
				&Header{Header: "Foo", Absent: true},
			},
		},
		&Test{
			Name: "DELETE Error",
			Request: Request{
				URL: u + "/sdjdfh/oieru",
			},
			Checks: []Check{
				StatusCode{Expect: 200},
				&Body{Prefix: "Successfully deleted " + u},
				&Header{Header: "Foo", Absent: true},
			},
		},
	}

	for i, test := range tests {
		p := strings.Index(test.Name, " ")
		if p == -1 {
			t.Fatalf("Ooops: no space in %d. Name: %s", i, test.Name)
		}
		method, want := test.Name[:p], test.Name[p+1:]
		test.Request.Method = method
		err = test.Run()
		if err != nil {
			t.Fatalf("%d. %s: Unexpected error: ", i, err)
		}

		got := test.Status.String()
		if got != want {
			t.Errorf("%d. %s: got %s, want %s. (Error=%v)",
				i, test.Name, got, want, test.Error)
		}
	}
}

// ----------------------------------------------------------------------------
// bash:// pseudo request

func TestBash(t *testing.T) {
	t.Run("Okay", testBashOkay)
	t.Run("Exit2", testBashNonzeroExit)
	t.Run("Timeout", testBashTimeout)
	t.Run("Error", testBashError)
}

func testBashOkay(t *testing.T) {
	test := &Test{
		Name: "Simple Bash Execution",
		Request: Request{
			URL: "bash://localhost/tmp",
			Params: url.Values{
				"FOO_VAR": []string{"wuz baz"},
			},
			Body: `
echo "Hello from your friendly bash script!"
echo "Today is $(date), we are in $(pwd)"
echo "FOO_VAR=$FOO_VAR"
`,
		},
		Checks: CheckList{
			&StatusCode{Expect: 200},
			&Body{Contains: "we are in /tmp"},
			&Body{Contains: "wuz baz"},
			&Header{Header: "Exit-Status", Condition: Condition{Equals: "exit status 0"}},
		},
	}

	if err := test.Run(); err != nil {
		t.Fatalf("Unexpected error %s <%T>", err, err)
	}
	if test.Status != Pass {
		test.PrintReport(os.Stdout)
		t.Errorf("Got test status %s (want Pass)", test.Status)
	}
}

func testBashNonzeroExit(t *testing.T) {
	test := &Test{
		Name: "Bash script with exit code 2.",
		Request: Request{
			URL:  "bash://localhost/tmp",
			Body: `echo Aaaaaarg....; exit 2; `,
		},
		Checks: CheckList{
			&StatusCode{Expect: 500},
			&Body{Contains: "Aaaaaarg"},
			&Header{Header: "Exit-Status", Condition: Condition{Equals: "exit status 2"}},
		},
	}

	if err := test.Run(); err != nil {
		t.Fatalf("Unexpected error %s <%T>", err, err)
	}
	if test.Status != Pass {
		test.PrintReport(os.Stdout)
		t.Errorf("Got test status %s (want Pass)", test.Status)
	}
}

func testBashTimeout(t *testing.T) {
	test := &Test{
		Name: "A too long running script.",
		Request: Request{
			URL:     "bash://localhost/tmp",
			Body:    `echo "Go"; sleep 1; echo "Running"; sleep 1; echo "Done"; sleep 1;`,
			Timeout: 1500 * time.Millisecond,
		},
		Checks: CheckList{
			&StatusCode{Expect: 408},
			&Body{Prefix: "Go"},
			&Body{Contains: "Running"},
			&Body{Contains: "Done", Count: -1},
		},
	}

	if err := test.Run(); err != nil {
		t.Fatalf("Unexpected error %s <%T>", err, err)
	}
	if test.Status != Pass {
		t.Errorf("Got %s, want Pass", test.Status)
		test.PrintReport(os.Stdout)
	}
}

func testBashError(t *testing.T) {
	test := &Test{
		Name: "A bogus script.",
		Request: Request{
			URL:  "bash://localhost/tmp/somehere-nonexisten",
			Body: `echo "Greeting from nowhere"`,
		},
		Checks: CheckList{
			&StatusCode{Expect: 200},
		},
	}

	if err := test.Run(); err != nil {
		t.Fatalf("Unexpected error %s <%T>", err, err)
	}
	if test.Status != Error {
		t.Errorf("Got %s, want Error", test.Status)
		e := test.Error.Error()
		if !strings.HasPrefix(e, "open /tmp/somehere-nonexisten/bashscript") ||
			!strings.HasSuffix(e, "no such file or directory") {
			t.Errorf("Got wrong error %s", e)
		}
		test.PrintReport(os.Stdout)
	}
}
