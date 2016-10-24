// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suite

import (
	"testing"
	"time"
)

var (
	min = time.Minute
	sec = time.Second
	ms  = time.Millisecond
	mu  = time.Microsecond
	ns  = time.Nanosecond
)

var rdTests = []struct {
	in   time.Duration
	want string
}{
	{3*min + 231*ms, "3m0s"},
	{3*min + 789*ms, "3m1s"},
	{65*sec + 789*ms, "1m6s"},
	{59*sec + 789*ms, "59.8s"},
	{10*sec + 789*ms, "10.8s"},
	{9*sec + 789*ms, "9.79s"},
	{9*sec + 123*ms, "9.12s"},
	{512*ms + 345*mu, "512ms"},
	{512*ms + 945*mu, "513ms"},
	{51*ms + 345*mu, "51.3ms"},
	{51*ms + 945*mu, "51.9ms"},
	{5*ms + 345*mu, "5.35ms"},
	{5*ms + 945*mu, "5.95ms"},
	{234*mu + 444*ns, "234µs"},
	{23*mu + 444*ns, "23.4µs"},
	{2*mu + 444*ns, "2.4µs"},
	{2*mu + 444*ns, "2.4µs"},
	{444 * ns, "440ns"},
}

func TestRoundDuration(t *testing.T) {
	for i, tc := range rdTests {
		if got := roundDuration(tc.in).String(); got != tc.want {
			t.Errorf("%d. roundDuration(%s) = %s, want %s",
				i, tc.in, got, tc.want)
		}
	}
}
