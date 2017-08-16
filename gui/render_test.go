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
