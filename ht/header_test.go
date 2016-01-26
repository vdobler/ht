// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"net/http"
	"testing"
)

var jsonct = Response{Response: &http.Response{
	StatusCode: 302,
	Header: http.Header{
		"Content-Type": []string{"application/json"},
		"Location":     []string{"http://example.org/foo/bar"},
	},
}}

var htmlct = Response{Response: &http.Response{
	StatusCode: 200,
	Header: http.Header{
		"Content-Type": []string{"text/html; charset=UTF-8"},
		"Location":     []string{"http://example.org/foo/bar"},
	},
}}

var xmlct = Response{Response: &http.Response{
	StatusCode: 301,
	Header: http.Header{
		"Content-Type": []string{"application/xml; charset=UTF-8"},
	},
}}

var contentTypeTests = []TC{
	{jsonct, &ContentType{Is: "application/json"}, nil},
	{jsonct, &ContentType{Is: "json"}, nil},
	{jsonct, &ContentType{Is: "text/html"}, someError},
	{jsonct, &ContentType{Is: "application/json", Charset: "any"}, someError},
	{htmlct, &ContentType{Is: "text/html"}, nil},
	{htmlct, &ContentType{Is: "text/html", Charset: "UTF-8"}, nil},
	{htmlct, &ContentType{Is: "text/html", Charset: "iso-latin-1"}, someError},

	{Response{}, &ContentType{Is: "application/json"}, someError},
}

func TestContentType(t *testing.T) {
	for i, tc := range contentTypeTests {
		runTest(t, i, tc)
	}
}

var redirectTests = []TC{
	{jsonct, &Redirect{To: "http://example.org/foo/bar"}, nil},
	{jsonct, &Redirect{To: "http://example.org/..."}, nil},
	{jsonct, &Redirect{To: ".../foo/bar"}, nil},
	{jsonct, &Redirect{To: "http://example.../bar"}, nil},
	{jsonct, &Redirect{To: "http://example.org/foo/bar", StatusCode: 302}, nil},
	{jsonct, &Redirect{To: "http://other.domain/waz"}, someError},
	{jsonct, &Redirect{To: "http://example.org/foo/bar", StatusCode: 307}, someError},
	{jsonct, &Redirect{To: "", StatusCode: 302}, prepareError},

	{htmlct, &Redirect{To: "http://example.org/foo/bar"}, someError},
	{htmlct, &Redirect{To: "http://example.org/"}, someError},
	{Response{}, &Redirect{To: "http://example.org/"}, someError},
	// Missing Location
	{htmlct, &Redirect{To: "http://example.org"}, someError},
}

func TestRedirect(t *testing.T) {
	for i, tc := range redirectTests {
		runTest(t, i, tc)
	}
}

func TestDotMatch(t *testing.T) {
	for i, tc := range []struct {
		g, w  string
		match bool
	}{
		{"foo", "foo", true},
		{"foo 123 bar", "foo...", true},
		{"foo 123 bar", "...bar", true},
		{"foo 123 bar", "f...r", true},
		{"wuz", "wuz...", true},
		{"wuz", "...wuz", true},
		{"wuz", "wu...z", true},
		{"foo", "qux", false},
		{"foo", "qux...", false},
		{"foo", "...qux", false},
		{"foo", "q...ux", false},
	} {
		if got := dotMatch(tc.g, tc.w); got != tc.match {
			t.Errorf("%d: dotMatch(%q, %q)=%t, want %t",
				i, tc.g, tc.w, got, tc.match)
		}
	}

}

func TestRedirectChain(t *testing.T) {
	resp := Response{Redirections: []string{
		"http://www.example.org/foo/bar",
		"http://www.example.org/foo",
		"http://www.example.org/foo/qux",
		"http://www.example.org/foo/qux/wiz",
	}}

	rdChainTests := []TC{
		{resp, &RedirectChain{
			Via: []string{".../bar", ".../foo", ".../qux", ".../wiz"}},
			nil},
		{resp, &RedirectChain{Via: []string{".../bar"}}, nil},
		{resp, &RedirectChain{Via: []string{".../foo"}}, nil},
		{resp, &RedirectChain{Via: []string{".../wiz"}}, nil},
		{resp, &RedirectChain{Via: []string{".../foo", ".../wiz"}}, nil},

		{resp, &RedirectChain{Via: []string{".../XXX"}}, someError},
		{resp, &RedirectChain{Via: []string{".../foo", ".../bar"}}, someError},
	}

	for i, tc := range rdChainTests {
		runTest(t, i, tc)
	}
}
