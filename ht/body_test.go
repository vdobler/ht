// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	_ "image/jpeg"
	_ "image/png"
	"testing"
)

var br = Response{BodyBytes: []byte("foo bar baz foo foo")}
var bodyTests = []TC{
	{br, &Body{Contains: "foo"}, nil},
	{br, &Body{Contains: "bar"}, nil},
	{br, &Body{Contains: "baz"}, nil},
	{br, &Body{Contains: "foo", Count: 3}, nil},
	{br, &Body{Contains: "baz", Count: 1}, nil},
	{br, &Body{Contains: "wup", Count: -1}, nil},
	{br, &Body{Contains: "foo bar", Count: 1}, nil},
	{br, &Body{Contains: "sit"}, NotFound},
	{br, &Body{Contains: "bar", Count: -1}, FoundForbidden},
	{br, &Body{Contains: "bar", Count: 2}, someError}, // TODO: real error checking
	{br, &Body{Prefix: "foo bar", Suffix: "foo foo"}, nil},
	{br, &Body{Min: 5, Max: 500}, nil},
	{br, &Body{Min: 500}, someError},
	{br, &Body{Max: 10}, someError},
	{br, &Body{Equals: "foo bar baz foo foo"}, nil},
	{br, &Body{Equals: "foo bar baZ foo foo"}, someError},
}

func TestBody(t *testing.T) {
	for i, tc := range bodyTests {
		tc.c.Prepare()
		runTest(t, i, tc)
	}
}

var utf8Tests = []TC{
	{Response{BodyBytes: []byte("All fine!")}, UTF8Encoded{}, nil},
	{Response{BodyBytes: []byte("BOMs \ufeff sucks!")}, UTF8Encoded{}, someError},
	{Response{BodyBytes: []byte("Strange \xbd\xb2\x3d\xbc")}, UTF8Encoded{}, someError},
}

func TestUTF8Encoded(t *testing.T) {
	for i, tc := range utf8Tests {
		runTest(t, i, tc)
	}
}
