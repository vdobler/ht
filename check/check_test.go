// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package check

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vdobler/ht/response"
)

type sampleCheck struct {
	A string
	B *string
	C int
	D *int
	E int64
	F time.Duration
	G []string
	H []int

	N nested
	M []nested
	P interface{}

	X float32
	Y int
	Z int

	privateInt int
	privateStr string
}

// let sampleCheck satisfy Check interface.
func (_ sampleCheck) Execute(response *response.Response) error { return nil }
func (_ sampleCheck) Prepare() error                            { return nil }

type nested struct {
	X string
	Y int
}

func BenchmarkSubstituteVariables(b *testing.B) {
	r := strings.NewReplacer("a", "X", "e", "Y", "o", "Z")
	f := map[int64]int64{99: 77}
	var ck Check
	ck = &Body{Contains: "Hallo", Count: 99}
	for i := 0; i < b.N; i++ {
		f := SubstituteVariables(ck, r, f)
		if _, ok := f.(*Body); !ok {
			b.Fatalf("Bad type %T", f)
		}
	}
}

func TestSubstituteVariables(t *testing.T) {
	r := strings.NewReplacer("a", "X", "e", "Y", "o", "Z")
	var ck Check
	ck = &Body{Contains: "Hallo"}
	f := SubstituteVariables(ck, r, nil)
	if bc, ok := f.(*Body); !ok {
		t.Errorf("Bad type %T", f)
	} else if bc.Contains != "HXllZ" {
		t.Errorf("Got %s", bc.Contains)
	}

	bar := "bar"
	baz := 34
	sample := sampleCheck{
		A: "foo",
		B: &bar,
		C: 56,
		D: &baz,
		E: 12321,
		F: time.Duration(999),
		G: []string{"hallo", "gut", "xyz"},
		H: []int{34, 999, 12321, 31415},
		N: nested{
			X: "zoo",
			Y: 56,
		},
		M: []nested{
			{X: "aa", Y: 34},
			{X: "bb", Y: 33},
		},
		P:          "foo",
		X:          56,
		Y:          731,
		Z:          9348,
		privateInt: 56,
		privateStr: "foo",
	}

	r = strings.NewReplacer("a", "X", "o", "Y")
	g := map[int64]int64{34: 44, 56: 66, 12321: 11, 999: 888}
	s := SubstituteVariables(sample, r, g)
	sc, ok := s.(sampleCheck)
	if !ok {
		t.Fatalf("Bad type %T", s)
	}
	if sc.A != "fYY" || *sc.B != "bXr" || sc.C != 66 || *sc.D != 44 ||
		sc.E != 11 || sc.F != time.Duration(888) {
		t.Fatalf("Got %+v", sc)
	}
	if len(sc.G) != 3 || sc.G[0] != "hXllY" || sc.G[1] != "gut" || sc.G[2] != "xyz" {
		t.Fatalf("Got %+v", sc)
	}

	if len(sc.H) != 4 || sc.H[0] != 44 || sc.H[1] != 888 ||
		sc.H[2] != 11 || sc.H[3] != 31415 {
		t.Fatalf("Got %+v", sc)
	}
	if sc.N.X != "zYY" || sc.N.Y != 66 {
		t.Fatalf("Got %+v", sc)
	}
	if len(sc.M) != 2 || sc.M[0].X != "XX" || sc.M[0].Y != 44 ||
		sc.M[1].X != "bb" || sc.M[1].Y != 33 {
		t.Fatalf("Got %+v", sc)
	}
	if sc.P.(string) != "fYY" {
		t.Fatalf("Got %+v", sc)
	}

	// Unexported stuff gets zeroed.
	if sc.X != 56 || sc.Y != 731 || sc.Z != 9348 || sc.privateInt != 0 || sc.privateStr != "" {
		t.Fatalf("Got %+v", sc)
	}

}

func TestUnmarshalJSON(t *testing.T) {
	j := []byte(`[
{Check: "ResponseTime", Lower: 3450},
{Check: "Body", Prefix: "BEGIN", Regexp: "foo", Count: 3},
]`)

	cl := CheckList{}
	err := (&cl).UnmarshalJSON(j)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	if len(cl) != 2 {
		t.Fatalf("Wrong len, got %d", len(cl))
	}

	if rt, ok := cl[0].(*ResponseTime); !ok {
		t.Errorf("Check 0, got %T, %#v", cl[0], cl[0])
	} else {
		if rt.Lower != 3450 {
			t.Errorf("Got Lower=%d", rt.Lower)
		}
	}

	if rt, ok := cl[1].(*Body); !ok {
		t.Errorf("Check 1, got %T, %#v", cl[1], cl[1])
	} else {
		if rt.Regexp != "foo" {
			t.Errorf("Got Reqexp=%q", rt.Regexp)
		}
		if rt.Prefix != "BEGIN" {
			t.Errorf("Got Prefix=%q", rt.Prefix)
		}
		ce := rt.Prepare()
		if ce != nil {
			t.Errorf("Unexpected error: %#v", ce)
		}
		if len(rt.re.FindAllString("The foo made foomuh", -1)) != 2 {
			t.Errorf("Got %v", rt.re.FindAllString("The foo made foomuh", -1))
		}
	}
}

// ----------------------------------------------------------------------------
// type TC and runTest: helpers for testing the different checks

type TC struct {
	r response.Response
	c Check
	e error
}

var someError = fmt.Errorf("any error")

const ms = 1e6

func runTest(t *testing.T, i int, tc TC) {
	tc.r.BodyReader()
	got := tc.c.Execute(&tc.r)
	switch {
	case got == nil && tc.e == nil:
		return
	case got != nil && tc.e == nil:
		t.Errorf("%d. %s %v: unexpected error %v",
			i, NameOf(tc.c), tc.c, got)
	case got == nil && tc.e != nil:
		t.Errorf("%d. %s %v: missing error, want %v",
			i, NameOf(tc.c), tc.c, tc.e)
	case got != nil && tc.e != nil:
		_, malformed := got.(MalformedCheck)
		if (tc.e == someError && !malformed) ||
			(tc.e == NotFound && got == NotFound) ||
			(tc.e == FoundForbidden && got == FoundForbidden) {
			return
		}
		switch tc.e.(type) {
		case MalformedCheck:
			if !malformed {
				t.Errorf("%d. %s %v:got \"%v\" of type %T, want MalformedCheck",
					i, NameOf(tc.c), tc.c, got, got)
			}
		default:
			t.Errorf("%d. %s %v: got %T of type \"%v\", want %T",
				i, NameOf(tc.c), tc.c, got, got, tc.e)
		}
	}
}
