// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"encoding/json"
	"testing"
	"time"
)

var DurationTests = []struct {
	ns int64
	s  string
}{
	{0, "0ns"},
	{1, "1ns"},
	{9, "9ns"},
	{10, "10ns"},
	{99, "99ns"},
	{100, "100ns"},
	{999, "999ns"},
	{1000, "1.00µs"},
	{1001, "1.00µs"},
	{1004, "1.00µs"},
	{1006, "1.01µs"},
	{1010, "1.01µs"},
	{9990, "9.99µs"},
	{10000, "10.0µs"},
	{100000, "100µs"},
	{999000, "999µs"},
	{999400, "999µs"},
	// {999600, "1.00ms"}, // TODO
	{1000000, "1.00ms"},
	{10000000, "10.0ms"},
	{99900000, "99.9ms"},
	{99940000, "99.9ms"},
	// {99960000, "100ms"}, // TODO
	{100000000, "100ms"},
	{999000000, "999ms"},
	{999400000, "999ms"},
	// {999600000, "1.00s"}, // TODO
	{1000000000, "1.00s"},
	{10000000000, "10.0s"},
	{100000000000, "100s"},
	{150000000000, "150s"},
	{200000000000, "3m20s"},
}

func TestDurationString(t *testing.T) {
	for i, tc := range DurationTests {
		got := Duration(tc.ns).String()
		if got != tc.s {
			t.Errorf("%d String(%d)=%s, want %s", i, tc.ns, got, tc.s)
		}
	}
}

var MarshalJSONTests = []struct {
	d Duration
	s string
}{
	{0, `"0ns"`},
	{30, `"30ns"`},
	{400, `"400ns"`},
	{1000, `"1.00µs"`},
	{123000, `"123µs"`},
	{1000000, `"1.00ms"`},
	{456000000, `"456ms"`},
	{7456000000, `"7.46s"`},
	{9999000000, `"10.00s"`},
	{10000000000, `"10.0s"`},
	{60000000000, `"60.0s"`},
	{61000000000, `"61.0s"`},
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
			delta := float64(got - tc.d)
			if delta/float64(tc.d) > 0.001 {
				t.Errorf("%d: got %s, want %s", i,
					time.Duration(got), time.Duration(tc.d))
			}
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
