// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"math/rand"
	"testing"
)

func TestSetRandomVariable(t *testing.T) {
	for i, tc := range []struct {
		r, want string
		err     error
	}{
		{r: "RANDOM NUMBER 80", want: "46", err: nil},
		{r: "RANDOM NUMBER 80 %d", want: "46", err: nil},
		{r: "RANDOM NUMBER 80 %04d", want: "0046", err: nil},
		{r: "RANDOM NUMBER 80 %x", want: "2e", err: nil},
		{r: "RANDOM NUMBER 80 %04X", want: "002E", err: nil},
		{r: "RANDOM NUMBER 70-80", want: "71", err: nil},
		{r: "RANDOM NUMBER 1000-9999", want: "6374", err: nil},
		{r: "RANDOM NUMBER 40-30", want: "46", err: nil},
		{r: "RANDOM TEXT 8", want: "", err: nil},
		{r: "RANDOM TEXT de 8", want: "", err: nil},
		{r: "RANDOM TEXT de 10-20", want: "", err: nil},
		{r: "RANDOM TEXT de 1", want: "", err: nil},
	} {
		vars := map[string]string{}
		Random = rand.New(rand.NewSource(1))
		setRandomVariable(vars, "{{"+tc.r+"}}")
		if got, ok := vars[tc.r]; !ok {
			t.Errorf("%d: %q missing value; vars=%v", i, tc.r, vars)
		} else if got != tc.want {
			t.Errorf("%d: %q got %q, want %q", i, tc.r, got, tc.want)
		}
	}
}
