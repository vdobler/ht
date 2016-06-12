// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vdobler/ht/internal/json5"
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
func (sampleCheck) Execute(t *Test) error { return nil }
func (sampleCheck) Prepare() error        { return nil }

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

func TestSubstituteCheckVariables(t *testing.T) {
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

func TestChecklistMarshalJSON(t *testing.T) {
	cl := CheckList{
		&StatusCode{Expect: 404},
		None{Of: CheckList{ResponseTime{Lower: Duration(1234)}}},
		None{Of: CheckList{UTF8Encoded{}}},
		AnyOne{
			Of: CheckList{
				&StatusCode{Expect: 303},
				&StatusCode{Expect: 404},
			},
		},
	}

	j, err := json5.MarshalIndent(cl, "", "    ")
	if err != nil {
		t.Fatalf("Unexpected error %v\n%s", err, j)
	}

	want := `[
    {
        Check: "StatusCode",
        Expect: 404
    },
    {
        Check: "None",
        Of: [
            {
                Check: "ResponseTime",
                Lower: "1.23µs"
            }
        ]
    },
    {
        Check: "None",
        Of: [
            {
                Check: "UTF8Encoded"
            }
        ]
    },
    {
        Check: "AnyOne",
        Of: [
            {
                Check: "StatusCode",
                Expect: 303
            },
            {
                Check: "StatusCode",
                Expect: 404
            }
        ]
    }
]`
	got := string(j)
	if got != want {
		t.Errorf("Got: %s", got)
	}
}

func TestChecklistUnmarshalJSON(t *testing.T) {
	j := []byte(`[
{Check: "ResponseTime", Lower: 1.23},
{Check: "Body", Prefix: "BEGIN", Regexp: "foo", Count: 3},
{Check: "None", Of: [
   {Check: "StatusCode", Expect: 500},
   {Check: "UTF8Encoded"},
]},
{Check: "AnyOne", Of: [
   {Check: "StatusCode", Expect: 303},
   {Check: "Body", Contains: "all good"},
   {Check: "ValidHTML"},
]},
]`)

	cl := CheckList{}
	err := (&cl).UnmarshalJSON(j)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	want := CheckList{
		ResponseTime{Lower: Duration(1230 * time.Millisecond)},
		&Body{Prefix: "BEGIN", Regexp: "foo", Count: 3},
		None{Of: CheckList{
			StatusCode{Expect: 500},
			UTF8Encoded{},
		}},
		AnyOne{Of: CheckList{
			StatusCode{Expect: 303},
			&Body{Contains: "all good"},
			ValidHTML{},
		}},
	}

	gots, err := json5.MarshalIndent(cl, "", "    ")
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	wants, err := json5.MarshalIndent(want, "", "    ")
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	if string(gots) != string(wants) {
		t.Errorf("Got:\n%s\nWant:\n%s", gots, wants)
	}

}

// ----------------------------------------------------------------------------
// type TC and runTest: helpers for testing the different checks

type TC struct {
	r Response
	c Check
	e error
}

var someError = fmt.Errorf("any error")
var prepareError = fmt.Errorf("prepare error")

const ms = 1e6

func runTest(t *testing.T, i int, tc TC) {
	tc.r.Body()
	fakeTest := Test{Response: tc.r}
	if err := tc.c.Prepare(); err != nil {
		if tc.e != prepareError {
			t.Errorf("%d. %s %v: unexpected error during Prepare %v",
				i, NameOf(tc.c), tc.c, err)
		}
		return // expected error during prepare
	}
	got := tc.c.Execute(&fakeTest)
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
			(tc.e == ErrNotFound && got == ErrNotFound) ||
			(tc.e == ErrFoundForbidden && got == ErrFoundForbidden) {
			return
		}
		switch tc.e.(type) {
		case MalformedCheck:
			if !malformed {
				t.Errorf("%d. %s %v:got \"%v\" of type %T, want MalformedCheck",
					i, NameOf(tc.c), tc.c, got, got)
			}
		default:
			if tc.e.Error() != got.Error() {
				t.Errorf("%d. %s %v: got \"%v\" of type %T , want \"%v\" %T",
					i, NameOf(tc.c), tc.c, got, got, tc.e, tc.e)
			}
		}
	}
}
