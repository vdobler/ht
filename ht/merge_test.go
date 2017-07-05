// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

func TestMerge(t *testing.T) {
	a := &Test{}
	b := &Test{}
	_, err := Merge(a, b)
	if err != nil {
		t.Fatalf("Unexpected error %#v", err)
	}

	a = &Test{
		Name:        "A",
		Description: "A does a in a very a-ish way.",
		Request: Request{
			Method: "POST",
			URL:    "http://demo.test",
			Header: http.Header{
				"User-Agent": []string{"A User Agent"},
				"Special-A":  []string{"Special A Value"},
			},
			Params: url.Values{
				"q": []string{"foo-A"},
				"a": []string{"aa", "AA"},
			},
			Cookies: []Cookie{
				{Name: "a", Value: "vaaaaalue"},
				{Name: "session", Value: "deadbeef"},
			},
			FollowRedirects: true,
			Chunked:         false,
			Authorization: Authorization{
				Basic: BasicAuth{Username: "foo.bar", Password: "secret"},
			},
		},
		Execution: Execution{
			PreSleep:   100,
			InterSleep: 120,
			PostSleep:  140,
		},
	}

	b = &Test{
		Name:        "B",
		Description: "B does b in a very b-ish way.",
		Request: Request{
			Method: "POST",
			Header: http.Header{
				"User-Agent": []string{"B User Agent"},
				"Special-B":  []string{"Special B Value"},
			},
			Params: url.Values{
				"q": []string{"foo-B"},
				"b": []string{"bb", "BB"},
			},
			Cookies: []Cookie{
				{Name: "b", Value: "vbbbbblue"},
				{Name: "session", Value: "othersession"},
			},
			FollowRedirects: false,
			Chunked:         true,
		},
		Execution: Execution{
			InterSleep: 300,
		},
	}

	c, err := Merge(a, b)
	if err != nil {
		t.Fatalf("Unexpected error %#v", err)
	}
	if *verboseTest {
		jr, err := json.MarshalIndent(c, "", "    ")
		if err != nil {
			t.Fatal(err.Error())
		}
		fmt.Println(string(jr))
	}
	if len(c.Request.Params) != 3 ||
		c.Request.Params["a"][0] != "aa" ||
		c.Request.Params["b"][0] != "bb" ||
		c.Request.Params["q"][0] != "foo-A" ||
		c.Request.Params["q"][1] != "foo-B" {
		t.Errorf("Bad Params. Got %#v", c.Request.Params)
	}
	if len(c.Request.Header) != 3 ||
		c.Request.Header["Special-A"][0] != "Special A Value" ||
		c.Request.Header["Special-B"][0] != "Special B Value" ||
		c.Request.Header["User-Agent"][0] != "A User Agent" ||
		c.Request.Header["User-Agent"][1] != "B User Agent" {
		t.Errorf("Bad Header. Got %#v", c.Request.Header)
	}
	if len(c.Request.Cookies) != 3 ||
		c.Request.Cookies[0].Value != "vaaaaalue" ||
		c.Request.Cookies[1].Value != "othersession" ||
		c.Request.Cookies[2].Value != "vbbbbblue" {
		t.Errorf("Bad cookies. Got %#v", c.Request.Cookies)
	}

	if c.Request.Authorization.Basic.Username != "foo.bar" || c.Request.Authorization.Basic.Password != "secret" {
		t.Errorf("Bad BasicAuth. Got %q : %q", c.Request.Authorization.Basic.Username,
			c.Request.Authorization.Basic.Password)
	}

	if c.Request.FollowRedirects || !c.Request.Chunked {
		t.Errorf("FollowRedirect=%t Chunked=%t",
			c.Request.FollowRedirects, c.Request.Chunked)
	}

	if c.Execution.PreSleep != 100 || c.Execution.InterSleep != 420 || c.Execution.PostSleep != 140 {
		t.Errorf("Bad sleep times. Got pre=%s, inter=%s, post=%s",
			c.Execution.PreSleep, c.Execution.InterSleep, c.Execution.PostSleep)
	}

}
