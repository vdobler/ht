// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
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

func TestChecklistMarshalJSON(t *testing.T) {
	cl := CheckList{
		&StatusCode{Expect: 404},
		None{Of: CheckList{ResponseTime{Lower: 1234}}},
		None{Of: CheckList{UTF8Encoded{}}},
		AnyOne{
			Of: CheckList{
				&StatusCode{Expect: 303},
				&StatusCode{Expect: 404},
			},
		},
	}

	j, err := json.MarshalIndent(cl, "", "    ")
	if err != nil {
		t.Fatalf("Unexpected error %v\n%s", err, j)
	}

	want := `[
    {
        "Check": "StatusCode",
        "Expect": 404
    },
    {
        "Check": "None",
        "Of": [
            {
                "Check": "ResponseTime",
                "Lower": 1234
            }
        ]
    },
    {
        "Check": "None",
        "Of": [
            {
                "Check": "UTF8Encoded"
            }
        ]
    },
    {
        "Check": "AnyOne",
        "Of": [
            {
                "Check": "StatusCode",
                "Expect": 303
            },
            {
                "Check": "StatusCode",
                "Expect": 404
            }
        ]
    }
]`
	got := string(j)
	if got != want {
		t.Errorf("Got: %s", got)
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
