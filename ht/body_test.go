// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"testing"
)

var br = Response{BodyStr: "foo bar baz foo foo 15\""}
var bodyTests = []TC{
	{br, &Body{Contains: "foo"}, nil},
	{br, &Body{Contains: "bar"}, nil},
	{br, &Body{Contains: "baz"}, nil},
	{br, &Body{Contains: "foo", Count: 3}, nil},
	{br, &Body{Contains: "baz", Count: 1}, nil},
	{br, &Body{Contains: "wup", Count: -1}, nil},
	{br, &Body{Contains: "foo bar", Count: 1}, nil},
	{br, &Body{Contains: "sit"}, ErrNotFound},
	{br, &Body{Contains: "bar", Count: -1}, ErrFoundForbidden},
	{br, &Body{Contains: "bar", Count: 2}, errCheck}, // TODO: real error checking
	{br, &Body{Prefix: "foo bar", Suffix: "foo foo 15\""}, nil},
	{br, &Body{Min: 5, Max: 500}, nil},
	{br, &Body{Min: 500}, errCheck},
	{br, &Body{Max: 10}, errCheck},
	{br, &Body{Equals: "foo bar baz foo foo 15\""}, nil},
	{br, &Body{Equals: "foo bar baZ foo foo 15\""}, errCheck},
}

func TestBody(t *testing.T) {
	for i, tc := range bodyTests {
		tc.c.Prepare()
		runTest(t, i, tc)
	}
}

var utf8Tests = []TC{
	{Response{BodyStr: "All fine!"}, UTF8Encoded{}, nil},
	{Response{BodyStr: "BOMs \ufeff sucks!"}, UTF8Encoded{}, errCheck},
	{Response{BodyStr: "Strange \xbd\xb2\x3d\xbc"}, UTF8Encoded{}, errCheck},
}

func TestUTF8Encoded(t *testing.T) {
	for i, tc := range utf8Tests {
		runTest(t, i, tc)
	}
}

var srb = Response{BodyStr: "foo 1 bar 2 waz 3 tir 4 kap 5"}
var srbh = Response{
	BodyStr: sampleHTML,
	Response: &http.Response{
		Header: http.Header{"Content-Type": []string{"text/html; charset=UTF-8"}},
	},
}

var sortedTests = []TC{
	{srb, &Sorted{Text: []string{"foo", "waz", "4"}}, nil},
	{srb, &Sorted{Text: []string{"foo 1 b", "ar 2 w", "az"}}, nil},
	{srb, &Sorted{Text: []string{"1", "2", "??", "3", "4"}}, errCheck},
	{srb, &Sorted{Text: []string{"1", "2", "??", "3", "4"}, AllowedMisses: 1}, nil},
	{srb, &Sorted{Text: []string{"1", "2", "??", "3", "XXX"}, AllowedMisses: 2}, nil},
	{srb, &Sorted{Text: []string{"YYY", "2", "??", "3", "XXX"}, AllowedMisses: 2}, errCheck},
	{srb, &Sorted{Text: []string{"xxx", "yyy", "2", "??"}, AllowedMisses: 2}, errCheck},
	{srb, &Sorted{Text: []string{"xxx", "yyy", "zzz", "??"}, AllowedMisses: 4}, errDuringPrepare},
	{srb, &Sorted{Text: []string{"1"}}, errDuringPrepare},
	{srbh, &Sorted{Text: []string{"Foo", "Bar", "Waz"}}, nil},
	{srbh, &Sorted{Text: []string{"Interwordemphasis", "Some important things", "Waz", "Three"}}, nil},
}

func TestSorted(t *testing.T) {
	for i, tc := range sortedTests {
		runTest(t, i, tc)
	}
}
