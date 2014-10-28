// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package check

import (
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

	X          float32
	privateInt int
	privateStr string
}

// let sampleCheck satisfy Check interface.
func (s sampleCheck) Okay(response *response.Response) error { return nil }

type nested struct {
	X string
	Y int
}

func TestSubstituteVariables(t *testing.T) {
	r := strings.NewReplacer("a", "X", "e", "Y", "o", "Z")
	var ck Check
	ck = BodyContains{Text: "Hallo"}
	f := SubstituteVariables(ck, r)
	if bc, ok := f.(BodyContains); !ok {
		t.Errorf("Bad type %T", f)
	} else if bc.Text != "HXllZ" {
		t.Errorf("Got %s", bc.Text)
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
		P: "foo",
		X: 56,

		privateInt: 56,
		privateStr: "foo",
	}

	r = strings.NewReplacer("a", "X", "o", "Y",
		"34", "44", "56", "66", "12321", "11", "999", "888")
	s := SubstituteVariables(sample, r)
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
	if sc.X != 56 || sc.privateInt != 0 || sc.privateStr != "" {
		t.Fatalf("Got %+v", sc)
	}
}
