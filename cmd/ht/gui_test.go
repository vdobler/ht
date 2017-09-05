// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/vdobler/ht/internal/hjson"
)

func TestFixDuration(t *testing.T) {
	soup := map[string]interface{}{
		"Name":  "Hello",
		"Sleep": int64(25 * time.Millisecond),
		"Exec": map[string]interface{}{
			"Wait": int64(45 * time.Second),
			"Frac": 4.321,
			"ID":   int64(12345678),
		},
		"Count": int64(45678),
	}

	fixDuration(soup)
	want := `{"Count":45678,"Exec":{"Frac":4.321,"ID":12345678,"Wait":"45s"},"Name":"Hello","Sleep":"25ms"}`
	raw, err := json.Marshal(soup)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(raw); got != want {
		t.Errorf("got :%s\nwant: %s\n", got, want)
	}
}

func TestInvertVars(t *testing.T) {
	variables := map[string]string{
		"FOO":         "foo",
		"CURRENT_DIR": ".",      // single character
		"COUNTER":     "34",     // smal number
		"FOOBAR":      "foobar", // longer than foo
	}

	type S struct {
		A         string
		B         string
		C         int
		Variables map[string]string
	}
	orig := S{
		A:         "foobarbaz34foowuz.com",
		B:         "foo",
		C:         34,
		Variables: variables,
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}
	var s interface{}
	err = hjson.Unmarshal(data, &s)
	if err != nil {
		t.Fatal(err)
	}
	soup := s.(map[string]interface{})

	isoup, err := invertVars(soup, variables)
	if err != nil {
		t.Fatal(err)
	}
	vardata, err := hjson.Marshal(isoup)
	if err != nil {
		t.Fatal(err)
	}
	got := string(vardata)
	want := `{
    A: "{{FOOBAR}}baz34{{FOO}}wuz.com"
    B: "{{FOO}}"
    C: 34
    Variables: {
        COUNTER: "34"
        CURRENT_DIR: .
        FOO: foo
        FOOBAR: foobar
    }
}`
	if got != want {
		t.Error("got", got)
	}
}
