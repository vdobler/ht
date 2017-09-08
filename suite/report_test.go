// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suite

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/vdobler/ht/ht"
)

var (
	min = time.Minute
	sec = time.Second
	ms  = time.Millisecond
	mu  = time.Microsecond
	ns  = time.Nanosecond
)

var rdTests = []struct {
	in   time.Duration
	want string
}{
	{3*min + 231*ms, "3m0s"},
	{3*min + 789*ms, "3m1s"},
	{65*sec + 789*ms, "1m6s"},
	{59*sec + 789*ms, "59.8s"},
	{10*sec + 789*ms, "10.8s"},
	{9*sec + 789*ms, "9.79s"},
	{9*sec + 123*ms, "9.12s"},
	{512*ms + 345*mu, "512ms"},
	{512*ms + 945*mu, "513ms"},
	{51*ms + 345*mu, "51.3ms"},
	{51*ms + 945*mu, "51.9ms"},
	{5*ms + 345*mu, "5.35ms"},
	{5*ms + 945*mu, "5.95ms"},
	{234*mu + 444*ns, "234µs"},
	{23*mu + 444*ns, "23.4µs"},
	{2*mu + 444*ns, "2.4µs"},
	{2*mu + 444*ns, "2.4µs"},
	{444 * ns, "440ns"},
}

func TestRoundDuration(t *testing.T) {
	for i, tc := range rdTests {
		if got := roundDuration(tc.in).String(); got != tc.want {
			t.Errorf("%d. roundDuration(%s) = %s, want %s",
				i, tc.in, got, tc.want)
		}
	}
}

var updateGolden = flag.Bool("update-golden", false, "update golden records")

func TestHTMLReport(t *testing.T) {
	// --------------------------------------------------------------------
	// Test 1
	request1, _ := http.NewRequest("GET", "http://www.example.org/foo/bar?baz=wuz", nil)
	request1.Header["Accept"] = []string{"text/html"}
	request1.Header["X-Custom"] = []string{"Sonne", "Mond"}
	test1 := &ht.Test{
		Name:        "Test 1",
		Description: "The first test",
		Request: ht.Request{
			Method:  "GET",
			URL:     "http://www.example.org/foo/bar?baz=wuz",
			Timeout: 250 * time.Millisecond,
			Request: request1,
		},
		Response: ht.Response{
			Response: &http.Response{
				Status:     "200 OK",
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header: http.Header{
					"Content-Type": []string{"text/xhtml"},
				},
			},
			Duration: 200 * time.Millisecond,
			BodyStr:  "Hello World!",
			BodyErr:  nil,
			Redirections: []string{
				"http://www.example.org/login",
				"http://www.example.org/auth",
			},
		},
		Result: ht.Result{
			Status:       ht.Pass,
			Started:      time.Date(2017, 9, 8, 9, 48, 1, 0, time.UTC),
			Duration:     210 * time.Millisecond,
			FullDuration: 220 * time.Millisecond,
			Tries:        1,
			CheckResults: []ht.CheckResult{
				{
					Name:     "StatusCode",
					JSON:     "{Expect: 200}",
					Status:   ht.Pass,
					Duration: 20 * time.Millisecond,
					Error:    nil,
				},
			},
		},
	}
	test1.SetMetadata("Filename", "test1.ht")

	// --------------------------------------------------------------------
	// Test 2
	request2, _ := http.NewRequest("POST", "http://www.example.org/api/user", nil)
	request2.Header["Accept"] = []string{"application/json"}
	test2 := &ht.Test{
		Name:        "Test 2",
		Description: "The second test",
		Request: ht.Request{
			Method:   "POST",
			URL:      "http://www.example.org/api/user",
			Timeout:  350 * time.Millisecond,
			Request:  request2,
			Body:     `{"command": "doWork"}`,
			SentBody: `{"command": "doWork"}`,
		},
		Response: ht.Response{
			Response: &http.Response{
				Status:     "200 OK",
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
			},
			Duration: 300 * time.Millisecond,
			BodyStr:  "true",
			BodyErr:  nil,
		},
		Result: ht.Result{
			Status:       ht.Fail,
			Started:      time.Date(2017, 9, 8, 9, 48, 1, 3, time.UTC),
			Duration:     310 * time.Millisecond,
			FullDuration: 320 * time.Millisecond,
			Tries:        1,
			CheckResults: []ht.CheckResult{
				{
					Name:     "StatusCode",
					JSON:     "{Expect: 200}",
					Status:   ht.Pass,
					Duration: 30 * time.Millisecond,
					Error:    nil,
				},
				{
					Name:     "Body",
					JSON:     `{Prefix: "super", Contains: "okay"}`,
					Status:   ht.Fail,
					Duration: 31 * time.Millisecond,
					Error: ht.ErrorList{
						fmt.Errorf("bad Prefix"),
						fmt.Errorf("missing okay"),
					},
				},
			},
		},
	}
	test2.SetMetadata("Filename", "test2.ht")

	// --------------------------------------------------------------------
	// Test 3
	test3 := &ht.Test{
		Name:        "Test 3",
		Description: "The third test",
		Result:      ht.Result{Status: ht.NotRun},
	}
	test3.SetMetadata("Filename", "test3.ht")

	// --------------------------------------------------------------------
	// Test 4
	test4 := &ht.Test{
		Name:        "Test 4",
		Description: "The fourth test",
		Result:      ht.Result{Status: ht.Skipped},
	}
	test4.SetMetadata("Filename", "test4.ht")

	suite := Suite{
		Name: "HTML Report Suite",
		Description: "Test generation of _Report_.html file\n" +
			"and the appropriate bodyfiles",
		Status: ht.Fail,
		Error: ht.ErrorList{
			fmt.Errorf("First Error"),
			fmt.Errorf("Second Error"),
		},
		Started:  time.Date(2017, 9, 8, 9, 48, 0, 123456789, time.UTC),
		Duration: 2345 * time.Millisecond,
		Tests:    []*ht.Test{test1, test2, test3, test4},
	}

	os.RemoveAll("testdata/testreport")
	err := os.Mkdir("testdata/testreport", 0766)
	if err != nil {
		panic(err)
	}

	err = HTMLReport("testdata/testreport", &suite)
	if err != nil {
		panic(err)
	}

	written, err := ioutil.ReadFile("testdata/testreport/_Report_.html")
	if err != nil {
		panic(err)
	}

	if *updateGolden {
		t.Log("Updating golden record testdata/goldenreport.html")
		err = ioutil.WriteFile("testdata/goldenreport.html", written, 0640)
		if err != nil {
			panic(err)
		}
	}

	golden, err := ioutil.ReadFile("testdata/goldenreport.html")
	if err != nil {
		panic(err)
	}

	writtenlines := bytes.Split(written, []byte("\n"))
	goldenlines := bytes.Split(golden, []byte("\n"))
	diff := 0
	for i := 0; i < len(goldenlines) && i < len(writtenlines) && diff < 10; i++ {
		a, b := string(goldenlines[i]), string(writtenlines[i])
		if a != b {
			t.Errorf("Line %d:\n  Got:  %q\n  Want: %q", i, b, a)
			diff++
		}
	}

}
