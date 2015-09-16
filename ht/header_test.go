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
