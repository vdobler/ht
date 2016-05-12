// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

var cookieResp = Response{Response: &http.Response{
	Request: &http.Request{
		URL: &url.URL{
			Path: "/some/path",
		},
	},
	Header: http.Header{
		"Set-Cookie": []string{
			"a=1; Path=/some; Max-Age=20; Domain=example.org; secure",
			"b=2; Path=/; HttpOnly",
			"c=3; Path=/other",
			"d=4; Max-Age=-1",
			"e=5; Expires=Mon, 02 Jan 2006 15:04:05 GMT",
			"e=5; Path=/other/resource; Max-Age=-999",
			"f=5",
		},
	},
}}

var setCookieTests = []TC{
	// Basic checks
	{cookieResp, &SetCookie{Name: "a"}, nil},
	{cookieResp, &SetCookie{Name: "x"}, someError},
	{cookieResp, &SetCookie{Name: "a", Value: Condition{Equals: "1"}}, nil},
	{cookieResp, &SetCookie{Name: "a", Value: Condition{Equals: "X"}}, someError},
	{cookieResp, &SetCookie{Name: "a", Path: Condition{Equals: "/some"}}, nil},
	{cookieResp, &SetCookie{Name: "a", Path: Condition{Equals: "/XXX"}}, someError},
	{cookieResp, &SetCookie{Name: "a", Domain: Condition{Suffix: ".org"}}, nil},
	{cookieResp, &SetCookie{Name: "a", Domain: Condition{Suffix: "XXX"}}, someError},
	{cookieResp, &SetCookie{Name: "a", MinLifetime: Duration(10 * time.Second)}, nil},
	{cookieResp, &SetCookie{Name: "a", MinLifetime: Duration(30 * time.Second)}, someError},

	// Different types of cookies
	{cookieResp, &SetCookie{Name: "a", Type: "persistent secure exposed"}, nil},
	{cookieResp, &SetCookie{Name: "a", Type: "session"}, someError},
	{cookieResp, &SetCookie{Name: "a", Type: "unsafe"}, someError},
	{cookieResp, &SetCookie{Name: "a", Type: "httpOnly"}, someError},
	{cookieResp, &SetCookie{Name: "b", Type: "session unsafe httpOnly"}, nil},
	{cookieResp, &SetCookie{Name: "b", Type: "exposed"}, someError},

	// Checking for absence
	{cookieResp, &SetCookie{Name: "a", Absent: true}, someError},
	{cookieResp, &SetCookie{Name: "X", Absent: true}, nil},
}

func TestSetCookie(t *testing.T) {
	for i, tc := range setCookieTests {
		runTest(t, i, tc)
	}
}

var delCookieTests = []TC{
	// Basic checks
	{cookieResp, &DeleteCookie{Name: "a"}, someError},
	{cookieResp, &DeleteCookie{Name: "b"}, someError},
	{cookieResp, &DeleteCookie{Name: "d"}, nil},
	{cookieResp, &DeleteCookie{Name: "e"}, nil},
	{cookieResp, &DeleteCookie{Name: "e", Path: "/wrong"}, someError},
}

func TestDeleteCookie(t *testing.T) {
	for i, tc := range delCookieTests {
		runTest(t, i, tc)
	}
}
