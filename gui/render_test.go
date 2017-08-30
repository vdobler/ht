// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gui

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestWalkErrors(t *testing.T) {
	Typedata = make(map[reflect.Type]Typeinfo)
	registerTestTypes()

	execution := Execution{
		Method:     "POST",
		Tries:      3,
		Wait:       456 * time.Millisecond,
		Hash:       "deadbeef",
		Env:        nil,
		unexported: -17,
	}

	form := make(url.Values)
	form.Set("Execution.Method", "FISH")
	form.Set("Execution.Tries", "none")
	form.Set("Execution.Wait", "a bit")
	form.Set("Execution.Hash", "plopp")
	form.Set("Execution.unexported", "19")

	cpy, err := walk(form, "Execution", reflect.ValueOf(execution))
	if err == nil {
		t.Fatal("nil error")
	}

	if n := len(err); n != 4 {
		t.Fatalf("got %d errors:\n%s", n, strings.Join(err.AsStrings(), "\n"))
	}

	fmt.Println(err)
	fmt.Println(cpy)
}

func TestBinaryString(t *testing.T) {
	for i, tc := range []struct {
		in  string
		bin bool
	}{
		{"", false},
		{"Hello World", false},
		{`{"some":"JSON"}`, false},
		{"\xEF\xBB\xBFHello World", true},
		{"\x00\x00", true},
		{"Hello\x04blob", true},
		{"\xbd\xb2\x3d\xbc\x20\xe2\x8c\x98", true},
	} {
		if got := binaryString(tc.in); (got != nil) != tc.bin {
			t.Errorf("%d: binaryString(%q)=%t, want %t",
				i, tc.in, (got == nil), tc.bin)
		}
	}

}
