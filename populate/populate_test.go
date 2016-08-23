// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package populate

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	hjson "github.com/hjson/hjson-go"
)

type S struct {
	A int
	B float64
	C string
}

type T struct {
	String string
	Int    int
	Slice  []string
	Map    map[string]string

	Int32s   []int32
	Float64s []float64
	Dict     map[string]interface{}

	S  S
	PS *S

	Duration time.Duration
}

var obj = make(map[string]interface{})

func init() {
	obj["String"] = "foo"
	obj["Int"] = 123
	obj["Slice"] = []string{"Hello", "World"}
	obj["Map"] = map[string]string{
		"up":   "top",
		"down": "bottom",
	}
	obj["Int32s"] = []interface{}{567, int8(100), "2345"}
	obj["Float64s"] = []interface{}{88.77, float32(0.01), "-23.456"}
	obj["Dict"] = map[string]interface{}{
		"pi":   3.141,
		"e":    "2.67",
		"prim": 57,
	}
	obj["S"] = map[string]interface{}{
		"A": 99,
		"B": 88.88,
		"C": "ccc",
	}
	obj["PS"] = map[string]interface{}{
		"A": -777,
		"B": -66.66,
		"C": "xXxXx",
	}
}

func TestStrict(t *testing.T) {
	data := `{
    String: foo
    "Int": 123,
    Slice: [
        Hello
        "World"
    ],
    "Map": {
        "down": "bottom",
        up: "top",
    },
    "Int32s": [
        567,
        100,
        2345
    ],
    "Float64s": [
        88.77,
        0.009999999776482582,
        -23.456
    ],
    "Dict": {
        "e": "2.67",
        "pi": 3.141,
        "prim": 57
    },
    "S": {
        "A": 99,
        "B": 88.88,
        "C": "ccc"
    },
    "PS": {
        "A": -777,
        "B": -66.66,
        "C": "xXxXx"
    }
    Duration: "2.34s"
}`
	var raw interface{}
	err := hjson.Unmarshal([]byte(data), &raw)
	if err != nil {
		t.Errorf("Error: %s", err)
	}

	v := T{}

	err = Strict(&v, raw)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}

	result, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		t.Errorf("Error: %s", err)
	}
	fmt.Println(string(result))
}

func TestStrictError(t *testing.T) {
	data := `{
    "S": {
        "A": 99,
        "B": 88.88,
        "C": "ccc",
        "XXX": "unknown"
    },
}`
	var raw interface{}
	err := hjson.Unmarshal([]byte(data), &raw)
	if err != nil {
		t.Errorf("Error: %s", err)
	}

	v := T{}

	err = Strict(&v, raw)
	if err == nil {
		t.Errorf("Missing error")
	}
}

func TestLax(t *testing.T) {
	data := `{
    "S": {
        "A": 99,
        "B": 88.88,
        "C": "ccc",
        "XXX": "unknown"
    },
}`
	var raw interface{}
	err := hjson.Unmarshal([]byte(data), &raw)
	if err != nil {
		t.Errorf("Error: %s", err)
	}

	v := T{}

	err = Lax(&v, raw)
	if err != nil {
		t.Errorf("Error: %s", err)
	}
}

// ----------------------------------------------------------------------------
// Populator

type DynamicSlice []string

func (d *DynamicSlice) Populate(src interface{}) error {
	*d = []string{"Foo", "bAr", "wuZ"}
	return nil
}

type DynamicMap map[string]string

func (d *DynamicMap) Populate(src interface{}) error {
	*d = map[string]string{
		"foo": "bar",
		"fiz": "waz",
	}
	return nil
}

var _ Populator = &DynamicSlice{}
var _ Populator = &DynamicMap{}

func TestPopulator(t *testing.T) {
	data := `{
        "S": 99,
        "D": [ 1, 2, 3 ],
        "I": 88,
        "M": { "a": 7, "x": 9 }
}`
	var raw interface{}
	err := hjson.Unmarshal([]byte(data), &raw)
	if err != nil {
		t.Fatalf("Error: %s", err)
	}

	type TD struct {
		S string
		D DynamicSlice
		I int
		M DynamicMap
	}

	v := TD{}
	err = Strict(&v, raw)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}

	fmt.Printf("%#v\n", v)
}
