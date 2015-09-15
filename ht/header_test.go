// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"net/http"
	"testing"
)

var jsonct = Response{Response: &http.Response{
	Header: http.Header{"Content-Type": []string{"application/json"}},
}}

var htmlct = Response{Response: &http.Response{
	Header: http.Header{"Content-Type": []string{"text/html; charset=UTF-8"}},
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
