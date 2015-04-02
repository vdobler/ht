// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package response

import (
	"encoding/json"
	"testing"
	"time"
)

var MarshalJSONTests = []struct {
	d Duration
	s string
}{
	{0, `"0µs"`},
	{1000, `"1µs"`},
	{123000, `"123µs"`},
	{1000000, `"1ms"`},
	{456000000, `"456ms"`},
	{7456000000, `"7456ms"`},
	{9999000000, `"9999ms"`},
	{10000000000, `"10.0s"`},
	{60000000000, `"60.0s"`},
	{61000000000, `"61s"`},
	{100000000000, `"100s"`},
	{180000000000, `"180s"`},
	{181000000000, `"3m01s"`},
	{(12*60 + 34) * 1e9, `"12m34s"`},
}

func TestMarshalJSONDuration(t *testing.T) {
	for i, tc := range MarshalJSONTests {
		got, err := tc.d.MarshalJSON()
		if err != nil {
			t.Errorf("%d: Unexpected error %#v", i, err)
		}
		if string(got) != tc.s {
			t.Errorf("%d: got %s, want %s", i, string(got), tc.s)
		}

	}

}

var UnmarshalJSONTests = []struct {
	d Duration
	s string
}{
	{0, "0"},
	{1e9, "1"},
	{1e9, "1.0"},
	{1e6, "0.001"},
}

func TestUnmarshalJSONDuration(t *testing.T) {
	for i, tc := range MarshalJSONTests {
		var got Duration
		err := json.Unmarshal([]byte(tc.s), &got)
		if err != nil {
			t.Errorf("%d: Unexpected error %#v", i, err)
		}
		if got != tc.d {
			t.Errorf("%d: got %s, want %s", i, time.Duration(got), time.Duration(tc.d))
		}

	}

	for i, tc := range UnmarshalJSONTests {
		var got Duration
		err := json.Unmarshal([]byte(tc.s), &got)
		if err != nil {
			t.Errorf("%d: Unexpected error %#v", i, err)
		}
		if got != tc.d {
			t.Errorf("%d: got %s, want %s", i, time.Duration(got), time.Duration(tc.d))
		}

	}
}
