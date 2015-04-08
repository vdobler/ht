// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"testing"
)

var responseTimeTests = []TC{
	{Response{Duration: 10 * ms}, ResponseTime{Lower: 20 * ms}, nil},
	{Response{Duration: 10 * ms}, ResponseTime{Lower: 2 * ms}, someError},
	{Response{Duration: 10 * ms}, ResponseTime{Higher: 2 * ms}, nil},
	{Response{Duration: 10 * ms}, ResponseTime{Higher: 20 * ms}, someError},
	{Response{Duration: 10 * ms}, ResponseTime{Higher: 5 * ms, Lower: 20 * ms}, nil},
	{Response{Duration: 10 * ms}, ResponseTime{Higher: 15 * ms, Lower: 20 * ms}, someError},
	{Response{Duration: 10 * ms}, ResponseTime{Higher: 5 * ms, Lower: 8 * ms}, someError},
	{Response{Duration: 10 * ms}, ResponseTime{Higher: 20 * ms, Lower: 5 * ms},
		MalformedCheck{Err: someError}},
}

func TestResponseTime(t *testing.T) {
	for i, tc := range responseTimeTests {
		runTest(t, i, tc)
	}
}
