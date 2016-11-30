// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// ----------------------------------------------------------------------------
// file:// pseudo request.

func TestFilePseudorequest(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	u := "file://" + wd + "/testdata/fileprotocol"

	tests := []*Test{
		&Test{
			Name: "PUT-Pass",
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
			Name: "PUT-Error",
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
			Name: "GET-Pass",
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
			Name: "GET-Fail",
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
			Name: "GET-Error",
			Request: Request{
				URL: u + "/slkdj/cxmvn",
			},
			Checks: []Check{
				StatusCode{Expect: 200},
				&Header{Header: "Foo", Absent: true},
			},
		},
		&Test{
			Name: "GET-Error",
			Request: Request{
				URL: "file://remote.host/some/path",
			},
			Checks: []Check{
				StatusCode{Expect: 200},
				&Header{Header: "Foo", Absent: true},
			},
		},
		&Test{
			Name: "DELETE-Pass",
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
			Name: "DELETE-Error",
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
		p := strings.Index(test.Name, "-")
		if p == -1 {
			t.Fatalf("Ooops: no '-' in %d. Name: %s", i, test.Name)
		}
		t.Run(test.Name, func(t *testing.T) {
			method, want := test.Name[:p], test.Name[p+1:]
			test.Request.Method = method
			err = test.Run()
			if err != nil {
				t.Fatalf("Unexpected error: %s <%T>", err, err)
			}

			got := test.Status.String()
			if got != want {
				t.Errorf("Fot %s, want %s. (Error=%v)", got, want, test.Error)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// bash:// pseudo request

func TestBashPseudorequest(t *testing.T) {
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

// ----------------------------------------------------------------------------
// sql:// pseudo request

// To test against a MySQL database:
//    $ docker run -d -e MYSQL_USER=test -e MYSQL_PASSWORD=test -e MYSQL_DATABASE=test \
//          -e MYSQL_ALLOW_EMPTY_PASSWORD=true -p 7799:3306 mysql:5.6
var mysqlDSN = flag.String("ht.mysql",
	"test:test@tcp(127.0.0.1:7799)/test",
	"MySQL data source name")

func TestSQLPseudorequest(t *testing.T) {
	t.Run("Create", testSQLCreate)
	t.Run("Fill", testSQLFill)
	t.Run("Select", testSQLSelect)
	t.Run("Insert", testSQLInsert)
	t.Run("BadQuery", testSQLBadQuery)
	t.Run("Single", testSQLSingle)
	t.Run("PlaintextSingle", testSQLSinglePlaintext)
	t.Run("PlaintextMulti", testSQLMultiPlaintext)
}

func testSQLCreate(t *testing.T) {
	test := &Test{
		Name: "SQL Create",
		Request: Request{
			Method: "POST",
			URL:    "sql://mysql",
			Params: url.Values{
				"DSN": []string{*mysqlDSN},
			},
			Body: `
CREATE TABLE orders (
  id INT NOT NULL AUTO_INCREMENT UNIQUE PRIMARY KEY,
  product VARCHAR(30),
  price DECIMAL(4,2)
);
`,
		},
		Checks: CheckList{
			&StatusCode{Expect: 200},
			&JSON{Element: "LastInsertId.Value",
				Condition: Condition{Equals: `0`}},
			&JSON{Element: "RowsAffected.Value",
				Condition: Condition{Equals: `0`}},
		},
	}

	if err := test.Run(); err != nil {
		t.Fatalf("Unexpected error %s <%T>", err, err)
	}
	if test.Status != Pass {
		test.PrintReport(os.Stdout)
		fmt.Println(test.Response.BodyStr)
		t.Errorf("Got test status %s (want Pass)", test.Status)
	}
}

func testSQLFill(t *testing.T) {
	test := &Test{
		Name: "SQL Insert",
		Request: Request{
			Method: "POST",
			URL:    "sql://mysql",
			Params: url.Values{
				"DSN": []string{*mysqlDSN},
			},
			Body: `
INSERT INTO orders
  (product,price)
VALUES
  ("Badetuch", 17.10),
  ("Taschenmesser", 24.00),
  ("Puzzle", 9.70)
;
`,
		},
		Checks: CheckList{
			&StatusCode{Expect: 200},
			&JSON{Element: "LastInsertId.Value",
				Condition: Condition{Equals: `1`}},
			&JSON{Element: "RowsAffected.Value",
				Condition: Condition{Equals: `3`}},
		},
	}

	if err := test.Run(); err != nil {
		t.Fatalf("Unexpected error %s <%T>", err, err)
	}
	if test.Status != Pass {
		test.PrintReport(os.Stdout)
		fmt.Println(test.Response.BodyStr)
		t.Errorf("Got test status %s (want Pass)", test.Status)
	}
}

func testSQLSelect(t *testing.T) {
	test := &Test{
		Name: "SQL Select",
		Request: Request{
			Method: "GET",
			URL:    "sql://mysql",
			Params: url.Values{
				"DSN": []string{*mysqlDSN},
			},
			Body: `
SELECT id AS orderID, product, price
FROM orders
ORDER BY price DESC;
`,
		},
		Checks: CheckList{
			&StatusCode{Expect: 200},
			&JSON{Element: "0.price",
				Condition: Condition{Equals: `"24.00"`}},
			&JSON{Element: "1.product",
				Condition: Condition{Equals: `"Badetuch"`}},
			&JSON{Element: "2.price",
				Condition: Condition{Equals: `"9.70"`}},
			&JSON{Element: "2.product",
				Condition: Condition{Equals: `"Puzzle"`}},
		},
	}

	if err := test.Run(); err != nil {
		t.Fatalf("Unexpected error %s <%T>", err, err)
	}
	if test.Status != Pass {
		test.PrintReport(os.Stdout)
		fmt.Println(test.Response.BodyStr)
		t.Errorf("Got test status %s (want Pass)", test.Status)
	}
}

func testSQLInsert(t *testing.T) {
	test := &Test{
		Name: "SQL Insert",
		Request: Request{
			Method: "POST",
			URL:    "sql://mysql",
			Params: url.Values{
				"DSN": []string{*mysqlDSN},
			},
			Body: `
INSERT INTO orders (product,price)
VALUES ("Buch", 38.00);
`,
		},
		Checks: CheckList{
			&StatusCode{Expect: 200},
			&JSON{Element: "LastInsertId.Value",
				Condition: Condition{Equals: `4`}},
			&JSON{Element: "RowsAffected.Value",
				Condition: Condition{Equals: `1`}},
		},
	}

	if err := test.Run(); err != nil {
		t.Fatalf("Unexpected error %s <%T>", err, err)
	}
	if test.Status != Pass {
		test.PrintReport(os.Stdout)
		fmt.Println(test.Response.BodyStr)
		t.Errorf("Got test status %s (want Pass)", test.Status)
	}
}

func testSQLBadQuery(t *testing.T) {
	test := &Test{
		Name: "SQL Bad Query",
		Request: Request{
			Method: "GET",
			URL:    "sql://mysql",
			Params: url.Values{
				"DSN": []string{*mysqlDSN},
			},
			Body: `
HUBBA BUBBA TRALLALA;
`,
		},
		Checks: CheckList{
			&StatusCode{Expect: 200},
		},
	}

	if err := test.Run(); err != nil {
		t.Fatalf("Unexpected error %s <%T>", err, err)
	}
	if test.Status != Error {
		test.PrintReport(os.Stdout)
		fmt.Println(test.Response.BodyStr)
		t.Errorf("Got test status %s (want Pass)", test.Status)
	}
}

func testSQLSingle(t *testing.T) {
	test := &Test{
		Name: "SQL Select",
		Request: Request{
			Method: "GET",
			URL:    "sql://mysql",
			Params: url.Values{
				"DSN": []string{*mysqlDSN},
			},
			Body: `
SELECT ROUND(AVG(price),2) AS avgprice FROM orders;
`,
		},
		Checks: CheckList{
			&StatusCode{Expect: 200},
			&JSON{Element: "0.avgprice",
				Condition: Condition{Equals: `"22.20"`}},
		},
	}

	if err := test.Run(); err != nil {
		t.Fatalf("Unexpected error %s <%T>", err, err)
	}
	if test.Status != Pass {
		test.PrintReport(os.Stdout)
		fmt.Println(test.Response.BodyStr)
		t.Errorf("Got test status %s (want Pass)", test.Status)
	}
}

func testSQLSinglePlaintext(t *testing.T) {
	test := &Test{
		Name: "SQL Single Plaintext",
		Request: Request{
			Method: "GET",
			URL:    "sql://mysql",
			Params: url.Values{
				"DSN": []string{*mysqlDSN},
			},
			Header: http.Header{
				"Accept": []string{"text/plain"},
			},
			Body: `
SELECT MIN(price) AS minprice, ROUND(AVG(price),2) AS avgprice FROM orders;
`,
		},
		Checks: CheckList{
			&StatusCode{Expect: 200},
			&Body{Equals: `9.70 22.20`},
		},
	}

	if err := test.Run(); err != nil {
		t.Fatalf("Unexpected error %s <%T>", err, err)
	}
	if test.Status != Pass {
		test.PrintReport(os.Stdout)
		fmt.Println(test.Response.BodyStr)
		t.Errorf("Got test status %s (want Pass)", test.Status)
	}
}

func testSQLMultiPlaintext(t *testing.T) {
	test := &Test{
		Name: "SQL Multiple Plaintext",
		Request: Request{
			Method: "GET",
			URL:    "sql://mysql",
			Params: url.Values{
				"DSN": []string{*mysqlDSN},
			},
			Header: http.Header{
				"Accept": []string{"text/plain"},
			},
			Body: `
SELECT id, price FROM orders WHERE price > 20;
`,
		},
		Checks: CheckList{
			&StatusCode{Expect: 200},
			&Body{Equals: "2 24.00\n4 38.00"},
		},
	}

	if err := test.Run(); err != nil {
		t.Fatalf("Unexpected error %s <%T>", err, err)
	}
	if test.Status != Pass {
		test.PrintReport(os.Stdout)
		fmt.Println(test.Response.BodyStr)
		t.Errorf("Got test status %s (want Pass)", test.Status)
	}
}
