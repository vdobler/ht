// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"database/sql"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// ----------------------------------------------------------------------------
// file:// pseudo request.

func TestFilePseudorequest(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	p := filepath.ToSlash(wd) + "/testdata/fileprotocol"
	u := "file://"
	if runtime.GOOS == "windows" {
		u += "/"
	}
	u += p

	linuxOrWinNotFound := []Check{
		&Body{Contains: "not a directory"},        // Linux
		&Body{Contains: "system cannot find the"}, // Windows
	}

	tests := []*Test{
		{
			Name: "PUT_okay",
			Request: Request{
				URL:  u,
				Body: "Tadadadaaa!",
			},
			Checks: []Check{
				StatusCode{Expect: 200},
				&Body{Prefix: "Successfully wrote " + u},
			},
		},
		{
			Name: "PUT_forbidden",
			Request: Request{
				URL:  u + "/iouer/cxxs/dlkfj",
				Body: "Tadadadaaa!",
			},
			Checks: []Check{
				StatusCode{Expect: 403},
				&Body{Contains: p},
				AnyOne{Of: linuxOrWinNotFound},
			},
		},
		{
			Name: "GET_okay",
			Request: Request{
				URL: u,
			},
			Checks: []Check{
				StatusCode{Expect: 200},
				&Body{Equals: "Tadadadaaa!"},
			},
		},
		{
			Name: "GET_wrongdir",
			Request: Request{
				URL: u + "/slfer/mxcmdk",
			},
			Checks: []Check{
				StatusCode{Expect: 404},
				&Body{Contains: p},
				AnyOne{Of: linuxOrWinNotFound},
			},
		},
		{
			Name: "GET_notfound",
			Request: Request{
				URL: u + "dfkewirxym",
			},
			Checks: []Check{
				StatusCode{Expect: 404},
				&Body{Contains: p},
				AnyOne{Of: linuxOrWinNotFound},
			},
		},
		{
			Name: "GET_fail", Description: "Fail",
			Request: Request{
				URL: u,
			},
			Checks: []Check{
				StatusCode{Expect: 200},
				&Body{Equals: "something else"},
				&Header{Header: "Foo", Absent: true},
			},
		},
		{
			Name: "GET_error", Description: "Error",
			Request: Request{
				URL: "file://remote.host/some/path",
			},
			Checks: []Check{
				StatusCode{Expect: 200},
			},
		},
		{
			Name: "DELETE_okay",
			Request: Request{
				URL: u,
			},
			Checks: []Check{
				StatusCode{Expect: 200},
				&Body{Prefix: "Successfully deleted " + u},
			},
		},
		{
			Name: "DELETE_nonexisting",
			Request: Request{
				URL: u + "/sdjdfh/oieru",
			},
			Checks: []Check{
				StatusCode{Expect: 404},
				AnyOne{Of: linuxOrWinNotFound},
			},
		},
	}
	if runtime.GOOS != "windows" {
		tests = append(tests, &Test{
			Name: "DELETE_forbidden",
			Request: Request{
				URL: "file:///etc/passwd",
			},
			Checks: []Check{
				StatusCode{Expect: 403},
				&Body{Contains: "permission denied"},
			},
		})
	}

	for i, test := range tests {
		p := strings.Index(test.Name, "_")
		if p == -1 {
			t.Fatalf("Ooops: no '_' in %d. Name: %s", i, test.Name)
		}
		expect := "Pass"
		if test.Description != "" {
			expect = test.Description
		}

		t.Run(test.Name, func(t *testing.T) {
			test.Request.Method = test.Name[:p]
			err = test.Run()
			if err != nil {
				t.Fatalf("Unexpected error: %s <%T>", err, err)
			}

			got := test.Status.String()
			if got != expect {
				sc := 0
				if test.Response.Response != nil {
					sc = test.Response.Response.StatusCode
				}
				t.Errorf("Got %s, want %s.\nError=%v\nStatusCode=%d\nBody=%q\n",
					got, expect, test.Error, sc, test.Response.BodyStr)
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
		fmt.Printf("Response-Body=%q\n", test.Response.BodyStr)
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
		fmt.Printf("Response-Body=%q\n", test.Response.BodyStr)
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
		fmt.Printf("Response-Body=%q\n", test.Response.BodyStr)
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
		t.Errorf("Got %s, want Error\nBody = %q\nHeader = %v",
			test.Status, test.Response.BodyStr,
			test.Response.Response.Header)
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
//    docker run -d -e MYSQL_USER=test -e MYSQL_PASSWORD=test -e MYSQL_DATABASE=test -e MYSQL_ALLOW_EMPTY_PASSWORD=true -p 7799:3306 mysql:5.6
var mysqlDSN = flag.String("ht.mysql",
	"test:test@tcp(127.0.0.1:7799)/test",
	"MySQL data source name")

func TestSQLPseudorequest(t *testing.T) {
	db, err := sql.Open("mysql", *mysqlDSN)
	if err != nil {
		t.Skipf("Cannot open %q", *mysqlDSN)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("Cannot connect %q", *mysqlDSN)
	}
	db.Exec("DROP TABLE orders;")

	for _, test := range sqlTests {
		t.Run(test.Name, func(t *testing.T) {
			if err := test.Run(); err != nil {
				t.Fatalf("Unexpected error %s <%T>", err, err)
			}
			if *verboseTest {
				fmt.Println("┌──────────────────────────────┐")
				fmt.Println("Test:", test.Name)
				fmt.Println("URL:", test.Request.Method, test.Request.URL)
				fmt.Println("Body:")
				fmt.Println(strings.TrimSpace(test.Request.Body))
				fmt.Println("Result:", test.Response.Response.Header.Get("Content-Type"))
				fmt.Println(test.Response.BodyStr)
				fmt.Println("└──────────────────────────────┘")
			}
			if test.Status != Pass {
				test.PrintReport(os.Stdout)
				fmt.Println(test.Response.BodyStr)
				t.Errorf("Got test status %s (want Pass)", test.Status)
			}
		})
	}

	for _, test := range sqlTestsErroring {
		t.Run(test.Name, func(t *testing.T) {
			if err := test.Run(); err != nil {
				t.Fatalf("Unexpected error %s <%T>", err, err)
			}
			if got := test.Status.String(); got != test.Description {
				test.PrintReport(os.Stdout)
				fmt.Println(test.Response.BodyStr)
				t.Errorf("Got test status %s (want %s)", got, test.Description)
			}
		})
	}
}

var sqlTests = []*Test{
	{
		Name: "Create",
		Request: Request{
			Method: "POST",
			URL:    "sql://mysql",
			Header: http.Header{
				"Data-Source-Name": []string{*mysqlDSN},
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
	},

	{
		Name: "Fill",
		Request: Request{
			Method: "POST",
			URL:    "sql://mysql",
			Header: http.Header{
				"Data-Source-Name": []string{*mysqlDSN},
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
	},
	{
		Name: "Select",
		Request: Request{
			Method: "GET",
			URL:    "sql://mysql",
			Header: http.Header{
				"Data-Source-Name": []string{*mysqlDSN},
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
	},

	{
		Name: "Insert",
		Request: Request{
			Method: "POST",
			URL:    "sql://mysql",
			Header: http.Header{
				"Data-Source-Name": []string{*mysqlDSN},
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
	},

	{
		Name: "JSON",
		Request: Request{
			Method: "GET",
			URL:    "sql://mysql",
			Header: http.Header{
				"Data-Source-Name": []string{*mysqlDSN},
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
	},

	{
		Name: "Text",
		Request: Request{
			Method: "GET",
			URL:    "sql://mysql",
			Header: http.Header{
				"Data-Source-Name": []string{*mysqlDSN},
				"Accept":           []string{"text/plain"},
			},
			Body: `
SELECT MIN(price) AS minprice, ROUND(AVG(price),2) AS avgprice FROM orders;
`,
		},
		Checks: CheckList{
			&StatusCode{Expect: 200},
			&Body{Equals: "9.70\t22.20"},
		},
	},

	{
		Name: "Text-Header",
		Request: Request{
			Method: "GET",
			URL:    "sql://mysql",
			Header: http.Header{
				"Data-Source-Name": []string{*mysqlDSN},
				"Accept":           []string{"text/plain; header=present"},
			},
			Body: `
SELECT id, price FROM orders WHERE price > 20;
`,
		},
		Checks: CheckList{
			&StatusCode{Expect: 200},
			&Body{Equals: "id\tprice\n2\t24.00\n4\t38.00"},
		},
	},

	{
		Name: "CSV",
		Request: Request{
			Method: "GET",
			URL:    "sql://mysql",
			Header: http.Header{
				"Data-Source-Name": []string{*mysqlDSN},
				"Accept":           []string{"text/csv"},
			},
			Body: `
SELECT id, price FROM orders WHERE price > 20;
`,
		},
		Checks: CheckList{
			&StatusCode{Expect: 200},
			&Body{Equals: "2,24.00\n4,38.00\n"},
		},
	},

	{
		Name: "CSV-Header",
		Request: Request{
			Method: "GET",
			URL:    "sql://mysql",
			Header: http.Header{
				"Accept":           []string{"text/csv; header=present"},
				"Data-Source-Name": []string{*mysqlDSN},
			},
			Body: `
SELECT id, price FROM orders WHERE price > 20;
`,
		},
		Checks: CheckList{
			&StatusCode{Expect: 200},
			&Body{Equals: "id,price\n2,24.00\n4,38.00\n"},
		},
	},
}

var sqlTestsErroring = []*Test{
	{
		Name:        "Bad-Query",
		Description: "Error",
		Request: Request{
			Method: "GET",
			URL:    "sql://mysql",
			Header: http.Header{
				"Data-Source-Name": []string{*mysqlDSN},
			},
			Body: `HUBBA BUBBA TRALLALA;`,
		},
	},

	{
		Name:        "Missing-DSN",
		Description: "Bogus",
		Request: Request{
			Method: "GET",
			URL:    "sql://mysql",
			Body:   `SELECT 1;`,
		},
	},

	{
		Name:        "Unknown-DBDriver",
		Description: "Bogus",
		Request: Request{
			Method: "GET",
			URL:    "sql://trallala",
			Header: http.Header{
				"Data-Source-Name": []string{*mysqlDSN},
			},
			Body: `SELECT 1;`,
		},
	},

	{
		Name:        "Missing-Query",
		Description: "Bogus",
		Request: Request{
			Method: "GET",
			URL:    "sql://mysql",
			Header: http.Header{
				"Data-Source-Name": []string{*mysqlDSN},
			},
		},
	},

	{
		Name:        "Bad-Method",
		Description: "Bogus",
		Request: Request{
			Method: "PUT",
			URL:    "sql://mysql",
			Header: http.Header{
				"Data-Source-Name": []string{*mysqlDSN},
			},
			Body: `SELECT 1;`,
		},
	},
}
