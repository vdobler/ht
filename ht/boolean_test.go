// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"strings"
	"testing"
)

// bCheck implements Check: It looks for want in the body and records
// if it's Execute method was called.
type bCheck struct {
	want     string
	executed bool
}

func (b *bCheck) Prepare() error { return nil }
func (b *bCheck) Execute(t *Test) error {
	b.executed = true
	if strings.Contains(t.Response.BodyStr, b.want) {
		return nil
	}
	return fmt.Errorf("%s missing", b.want)
}

func TestAnyOne(t *testing.T) {
	first := &bCheck{want: "foo"}
	second := &bCheck{want: "bar"}

	anytcs := []struct {
		body string
		err  error
		both bool
	}{
		{"foo", nil, false},
		{"bar", nil, true},
		{"qux", errCheck, true},
	}

	for i, at := range anytcs {
		first.executed, second.executed = false, false
		tc := TC{Response{BodyStr: at.body},
			AnyOne{Of: CheckList{first, second}},
			at.err}
		runTest(t, i, tc)
		if !first.executed {
			t.Errorf("%d: first check in AnyOne not executed", i)
		}
		if at.both != second.executed {
			t.Errorf("%d: second check executed=%t, want=%t", i, second.executed, at.both)
		}
	}
}

func TestNone(t *testing.T) {
	first := &bCheck{want: "foo"}
	second := &bCheck{want: "bar"}

	anytcs := []struct {
		body string
		err  error
		both bool
	}{
		{"foo", errCheck, false}, // foo fulfilled -> error after first, second skipped
		{"bar", errCheck, true},  // bar fulfilled -> error after executing both
		{"qux", nil, true},       // both false -> pass, both executed
	}

	for i, at := range anytcs {
		first.executed, second.executed = false, false
		tc := TC{Response{BodyStr: at.body},
			None{Of: CheckList{first, second}},
			at.err}
		runTest(t, i, tc)
		if !first.executed {
			t.Errorf("%d: first check in None not executed", i)
		}
		if at.both != second.executed {
			t.Errorf("%d: second check executed=%t, want=%t", i, second.executed, at.both)
		}
	}
}
