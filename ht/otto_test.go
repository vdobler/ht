// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"errors"
	"net/http"
	"testing"
)

var customJSTests = []struct {
	script string
	error  string // Result from Execute
}{
	// true, 0 and "" indicate PASS
	{`true;`, ""},
	{`0;`, ""},
	{`"";`, ""},
	// Anything else indicates FAIL
	{`false;`, "false"},
	{`9;`, "9"},
	{`-7.8;`, "-7.8"},
	{`"problem X";`, "problem X"},
	// TODO other JavaScript values like undefined or whatever there might be...

	// Accessing the current Test object and it's fields
	{`Test.Name;`, "Dummy test for CustomJS check"},
	{`Test.Request.Method[0];`, "P"},
	{`Test.Request.Header["X-Special"];`, "tada"},
	{`Test.Response.BodyStr.length;`, "235"},
	{`Test.Response.Response.Trailer["Sponsored-By"];`, "Someone"},

	// Simple but real checks
	{`Test.Request.Method === "POST";`, ""},
	{`Test.Request.Method === "HEAD";`, "false"},
	{`
          var m = Test.Request.Method;
          if(m==="HEAD") {
              true;
          } else {
              "Wrong method "+m;
          }`,
		"Wrong method POST",
	},
	{`
	  var body = JSON.parse(Test.Response.BodyStr);
          if (body[1].name=="Bern") true; else "Expecting Bern, got "+body[1].name;
         `,
		"",
	},
	{`
	  var body = JSON.parse(Test.Response.BodyStr);
          if (body[2].name=="Bern") true; else "Expecting Bern, got "+body[2].name;
         `,
		"Expecting Bern, got Zürich",
	},

	// Use of underscore:
	{`
	  var body = JSON.parse(Test.Response.BodyStr);
          var all = _.reduce(body, function(memo, s){ return memo+" "+s.code; }, "");
          all;
         `,
		" AG BE ZH ZG GE",
	},

	// Script from disk.
	{`@file:./testdata/custom.js`, "Lalelu"},
}

func TestCustomJS(t *testing.T) {
	body := `[
  { "id": 12, "code": "AG", "name": "Aargau" },
  { "id": 34, "code": "BE", "name": "Bern" },
  { "id": 56, "code": "ZH", "name": "Zürich" },
  { "id": 78, "code": "ZG", "name": "Zug" },
  { "id": 90, "code": "GE", "name": "Genf" }
]`

	test := &Test{
		Name: "Dummy test for CustomJS check",
		Request: Request{
			Method: "POST",
			URL:    "http://example.org",
			Header: http.Header{
				"X-Special": []string{"tada"},
			},
		},

		Response: Response{
			Response: &http.Response{
				Status: "200 OK",
				Trailer: http.Header{
					"Sponsored-By": []string{"Someone"},
				},
			},
			BodyStr: body,
			BodyErr: errors.New("client timeout"),
		},
	}

	for i, tc := range customJSTests {
		custom := &CustomJS{Script: tc.script}
		err := custom.Prepare()
		if err != nil {
			t.Errorf("%d. Unexpected error during Prepare: %s", i, err)
			continue
		}

		err = custom.Execute(test)
		if err != nil {
			if tc.error == "" {
				t.Errorf("%d. Unexpected error during Execute: '%s'", i, err)
			} else if got := err.Error(); tc.error != got {
				t.Errorf("%d. Wrong error '%s', want '%s'", i, got, tc.error)
			}
			continue
		} else if tc.error != "" {
			t.Errorf("%d. Unexpected passing, want error '%s'", i, tc.error)

		}
		continue
	}
}
