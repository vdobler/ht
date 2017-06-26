// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

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

var errCheck = fmt.Errorf("any error during Execute of a check")
var errDuringPrepare = fmt.Errorf("prepare error")

const ms = 1e6

func runTest(t *testing.T, i int, tc TC) {
	fakeTest := Test{Response: tc.r}
	if prep, ok := tc.c.(Preparable); ok {
		if err := prep.Prepare(&fakeTest); err != nil {
			if tc.e != errDuringPrepare {
				t.Errorf("%d. %s %+v: unexpected error during Prepare %v",
					i, NameOf(tc.c), tc.c, err)
			}
			return // expected error during prepare
		}
	}
	got := tc.c.Execute(&fakeTest)
	switch {
	case got == nil && tc.e == nil:
		return
	case got != nil && tc.e == nil:
		t.Errorf("%d. %s %+v: unexpected error %v",
			i, NameOf(tc.c), tc.c, got)
	case got == nil && tc.e != nil:
		t.Errorf("%d. %s %+v: missing error, want %v",
			i, NameOf(tc.c), tc.c, tc.e)
	case got != nil && tc.e != nil:
		if tc.e == errCheck {
			return // fine, any error
		}
		if tc.e.Error() != got.Error() {
			t.Errorf("%d. %s %+v:\n\tgot  %q  (of type %T)\n\twant %q  (of tyoe %T)",
				i, NameOf(tc.c), tc.c, got, got, tc.e, tc.e)
		}
	}
}

func TestPossibleCheckNames(t *testing.T) {
	valid := strings.Split("AnyOne Body Cache ContentType CustomJS "+
		"DeleteCookie ETag FinalURL HTMLContains HTMLTag Header "+
		"Identity Image JSON JSONExpr Latency Links Logfile "+
		"NoServerError None Redirect RedirectChain RenderedHTML "+
		"RenderingTime Resilience ResponseTime Screenshot SetCookie "+
		"Sorted StatusCode UTF8Encoded ValidHTML W3CValidHTML XML", " ")

	for _, tc := range []struct {
		name, want string
	}{
		{"JSONEx", "JSON, JSONExpr"},
		{"StatusCod", "StatusCode"},
		{"Statuscode", "StatusCode"},
		{"statscode", "StatusCode"},
		{"HTML", "XML"},
		{"Grblfmpf", ""},
	} {
		got := strings.Join(possibleNames(tc.name, valid), ", ")
		if got != tc.want {
			t.Errorf("possibleCheckNames(%q) = %q, want %q",
				tc.name, got, tc.want)
		}
	}
}
