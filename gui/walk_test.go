// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gui

import (
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestWalkBool(t *testing.T) {
	form := make(url.Values)
	s := true

	// Without update.
	cpy, err := walkBool(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if got := cpy.Bool(); got != s {
		t.Fatalf("got %t, want %t", got, s)
	}

	// With update.
	form.Set("s", "")
	cpy, err = walkBool(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if cpy.Bool() != false {
		t.Fatal("bad value")
	}

	form.Set("s", "true")
	cpy, err = walkBool(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if cpy.Bool() != true {
		t.Fatal("bad value")
	}
}

func TestWalkString(t *testing.T) {
	form := make(url.Values)
	s := "Hello"

	// Without update.
	cpy, err := walkString(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if got := cpy.String(); got != s {
		t.Fatalf("got %q, want %s", got, s)
	}

	// With update.
	n := "World"
	form.Set("s", n)
	cpy, err = walkString(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if got := cpy.String(); got != n {
		t.Fatalf("got %q, want %s", got, n)
	}
}

func TestWalkFloat64(t *testing.T) {
	form := make(url.Values)
	s := 3.141

	// Without update.
	cpy, err := walkFloat64(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if got := cpy.Float(); got != s {
		t.Fatalf("got %f, want %f", got, s)
	}

	// With update.
	n := "2.718"
	form.Set("s", n)
	cpy, err = walkFloat64(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if got := cpy.Float(); got != 2.718 {
		t.Fatalf("got %f, want %s", got, n)
	}
}

func TestWalkIntSlice(t *testing.T) {
	form := make(url.Values)
	s := []int{2, 3, 5, 7}

	// Without update.
	cpy, err := walkSlice(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if cpy.Kind() != reflect.Slice {
		t.Fatal(cpy.Kind().String())
	}
	c := cpy.Interface().([]int)
	if got := fmt.Sprintf("%v", c); got != "[2 3 5 7]" {
		t.Fatal(got)
	}

	// With update.
	form.Set("s.1", "11")
	cpy, err = walkSlice(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if cpy.Kind() != reflect.Slice {
		t.Fatal(cpy.Kind().String())
	}
	c = cpy.Interface().([]int)
	if got := fmt.Sprintf("%v", c); got != "[2 11 5 7]" {
		t.Fatal(got)
	}
}

func TestWalkIntSliceAdd(t *testing.T) {
	form := make(url.Values)
	s := []int{2, 3, 5, 7}

	form.Set("s.__OP__", "Add")
	cpy, err := walkSlice(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if cpy.Kind() != reflect.Slice {
		t.Fatal(cpy.Kind().String())
	}
	c := cpy.Interface().([]int)
	if got := fmt.Sprintf("%v", c); got != "[2 3 5 7 0]" {
		t.Fatal(got)
	}
}

func TestWalkIntSliceRemove(t *testing.T) {
	s := []int{2, 3, 5, 7}

	for step, rm := range []string{"1:[2 5 7]", "0:[5 7]", "1:[5]", "0:[]"} {
		p := strings.Split(rm, ":")
		index, want := p[0], p[1]
		form := make(url.Values)
		form.Set("s."+index+".__OP__", "Remove")
		cpy, err := walkSlice(form, "s", reflect.ValueOf(s))
		if err != nil {
			t.Fatal(err)
		}
		if cpy.Kind() != reflect.Slice {
			t.Fatal(cpy.Kind().String())
		}
		c := cpy.Interface().([]int)
		if got := fmt.Sprintf("%v", c); got != want {
			t.Fatalf("Step %d: s=%v, got=%s, want=%s",
				step, s, got, want)
		}
		s = c
	}
}

func TestWalkStringSlice(t *testing.T) {
	form := make(url.Values)
	s := []string{"2", "3", "5", "7"}

	// Without update.
	cpy, err := walkSlice(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if cpy.Kind() != reflect.Slice {
		t.Fatal(cpy.Kind().String())
	}
	c := cpy.Interface().([]string)
	if got := fmt.Sprintf("%v", c); got != "[2 3 5 7]" {
		t.Fatal(got)
	}

	// With update.
	form.Set("s.1", "11")
	cpy, err = walkSlice(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if cpy.Kind() != reflect.Slice {
		t.Fatal(cpy.Kind().String())
	}
	c = cpy.Interface().([]string)
	if got := fmt.Sprintf("%v", c); got != "[2 11 5 7]" {
		t.Fatal(got)
	}
}

func TestWalkStruct(t *testing.T) {
	form := make(url.Values)
	type S struct {
		A int
		B string
	}
	s := S{123, "abc"}

	// Without update.
	cpy, err := walkStruct(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if cpy.Kind() != reflect.Struct {
		t.Fatal(cpy.Kind().String())
	}
	c := cpy.Interface().(S)
	if got := fmt.Sprintf("%v", c); got != "{123 abc}" {
		t.Fatal(got)
	}

	// With update.
	form.Set("s.A", "-12")
	form.Set("s.B", "xyz")
	cpy, err = walkStruct(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if cpy.Kind() != reflect.Struct {
		t.Fatal(cpy.Kind().String())
	}
	c = cpy.Interface().(S)
	if got := fmt.Sprintf("%v", c); got != "{-12 xyz}" {
		t.Fatal(got)
	}
}

func mapRep(m map[string]string) string {
	keys := []string{}
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	elems := []string{}
	for _, k := range keys {
		elems = append(elems, fmt.Sprintf("%s:%s", k, m[k]))
	}
	return "[" + strings.Join(elems, " ") + "]"
}

func TestWalkMap(t *testing.T) {
	form := make(url.Values)
	s := map[string]string{
		"FOO": "abc",
		"BAR": "xyz",
	}

	// Without update.
	cpy, err := walkMap(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if cpy.Kind() != reflect.Map {
		t.Fatal(cpy.Kind().String())
	}
	c := cpy.Interface().(map[string]string)
	if got := mapRep(c); got != "[BAR:xyz FOO:abc]" {
		t.Fatal(got)
	}

	// With update.
	form.Set("s.FOO", "123")
	form.Set("s.BAR.__OP__", "Remove")
	cpy, err = walkMap(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if cpy.Kind() != reflect.Map {
		t.Fatal(cpy.Kind().String())
	}
	c = cpy.Interface().(map[string]string)
	if got := mapRep(c); got != "[FOO:123]" {
		t.Fatal(got)
	}

	// New key.
	form.Set("s.__OP__", "Add")
	form.Set("s.__NEW__", "WUZ")
	form.Set("s.BAR", "+++")
	cpy, err = walkMap(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if cpy.Kind() != reflect.Map {
		t.Fatal(cpy.Kind().String())
	}
	c = cpy.Interface().(map[string]string)
	if got := mapRep(c); got != "[BAR:+++ FOO:abc WUZ:]" {
		t.Fatal(got)
	}
}

func TestWalkNonNilPtr(t *testing.T) {
	form := make(url.Values)
	x := "Hello"
	s := &x

	// Without update.
	cpy, err := walkPtr(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if cpy.Kind() != reflect.Ptr {
		t.Fatal(cpy.Kind().String())
	}
	if got := cpy.Elem().String(); got != x {
		t.Fatalf("got %q, want %s", got, x)
	}

	// With update.
	form.Set("s.__OP__", "Remove")
	cpy, err = walkPtr(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if cpy.Kind() != reflect.Ptr {
		t.Fatal(cpy.Kind().String())
	}
	if !cpy.IsNil() {
		t.Fatal(cpy)
	}
}

func TestWalkNilPtr(t *testing.T) {
	form := make(url.Values)
	var s *string

	// Without update.
	cpy, err := walkPtr(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if cpy.Kind() != reflect.Ptr {
		t.Fatal(cpy.Kind().String())
	}
	if !cpy.IsNil() {
		t.Fatal(cpy)
	}

	// With update.
	form.Set("s.__OP__", "Add")
	cpy, err = walkPtr(form, "s", reflect.ValueOf(s))
	if err != nil {
		t.Fatal(err)
	}
	if cpy.Kind() != reflect.Ptr {
		t.Fatal(cpy.Kind().String())
	}
	if cpy.IsNil() {
		t.Fatal(cpy)
	}
	if got := cpy.Elem().String(); got != "" {
		t.Fatal(cpy)
	}
}

func TestWalkNamedTypes(t *testing.T) {
	type MyBool bool
	type MyInt int
	type MyString string
	type MyFloat float64

	type MyType struct {
		A MyBool
		B MyInt
		C MyString
		D MyFloat
	}
	mt := MyType{A: MyBool(true), B: MyInt(5), C: MyString("foo"), D: MyFloat(3.141)}

	form := make(url.Values)

	// Without update.
	cpy, err := walkStruct(form, "mt", reflect.ValueOf(mt))
	if err != nil {
		t.Fatal(err)
	}
	if cpy.Kind() != reflect.Struct {
		t.Fatal(cpy.Kind().String())
	}
	c := cpy.Interface().(MyType)
	if got := fmt.Sprintf("%v", c); got != "{true 5 foo 3.141}" {
		t.Fatal(got)
	}

	// With update.
	form.Set("mt.A", "false")
	form.Set("mt.B", "-12")
	form.Set("mt.C", "xyz")
	form.Set("mt.D", "2.718")
	cpy, err = walkStruct(form, "mt", reflect.ValueOf(mt))
	if err != nil {
		t.Fatal(err)
	}
	if cpy.Kind() != reflect.Struct {
		t.Fatal(cpy.Kind().String())
	}
	c = cpy.Interface().(MyType)
	if got := fmt.Sprintf("%v", c); got != "{false -12 xyz 2.718}" {
		t.Fatal(got)
	}

}

func TestBinData(t *testing.T) {
	type MyType struct {
		S string
		B []byte
		X struct {
			T string
		}
		Y []string
		Z map[string]string
	}

	mt := MyType{
		S: "foo",
		B: []byte("bar"),
		X: struct{ T string }{T: "wuz"},
		Y: []string{"abc", "xyz"},
		Z: map[string]string{"key": "val"},
	}

	v := NewValue(mt, "Object")

	for i, tc := range []struct {
		path, want string
	}{
		{"Object.S", "foo"},
		{"Object.B", "bar"},
		{"Object.X.T", "wuz"},
		{"Object.Y.0", "abc"},
		{"Object.Y.1", "xyz"},
		{"Object.Z.key", "val"},
	} {
		data, err := v.BinaryData(tc.path)
		if err != nil {
			t.Errorf("%d. %s: unexpected error %s", i, tc.path, err)
		} else if got := string(data); got != tc.want {
			t.Errorf("%d. %s: got %q, want %q",
				i, tc.path, got, tc.want)
		}
	}
}

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

	if n := len(err); n != 2 {
		t.Fatalf("got %d errors:\n%s", n, strings.Join(err.AsStrings(), "\n"))
	}

	fmt.Println(err)
	fmt.Println(cpy)
}
