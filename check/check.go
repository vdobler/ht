// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package check provides useful checks for ht.
package check

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"reflect"

	"github.com/vdobler/ht/response"
	"github.com/yosuke-furukawa/json5/encoding/json5"
)

// Check is a single check performed on a Response.
type Check interface {
	Okay(response *response.Response) error
}

// Compiler is the interface a check may implement to precompile
// stuff (e.g. regular expressions) during the preparation phase
// of a test.
type Compiler interface {
	Compile() error
}

// NameOf returns the name of the check.
func NameOf(check Check) string {
	typ := reflect.TypeOf(check)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	return typ.Name()
}

// SubstituteVariables returns a deep copy of check with all exported string
// fields in check modified by applying r and all int and int64 fields modified
// by applying f.
// TODO: applying r is not "variable replacing"
func SubstituteVariables(check Check, r *strings.Replacer, f map[int64]int64) Check {
	src := reflect.ValueOf(check)
	dst := reflect.New(src.Type()).Elem()
	deepCopy(dst, src, r, f)
	return dst.Interface().(Check)
}

// deepCopy copes src recursively to dst while transforming all string fields
// by applying r and f
func deepCopy(dst, src reflect.Value, r *strings.Replacer, f map[int64]int64) {
	if !dst.CanSet() {
		return
	}
	switch src.Kind() {
	case reflect.String:
		// TODO: maybe skip certain fields based on their struct tag?
		dst.SetString(r.Replace(src.String()))
	case reflect.Int, reflect.Int64:
		if n, ok := f[src.Int()]; ok {
			dst.SetInt(n)
		} else {
			dst.Set(src)
		}
	case reflect.Struct:
		for i := 0; i < src.NumField(); i += 1 {
			deepCopy(dst.Field(i), src.Field(i), r, f)
		}
	case reflect.Slice:
		dst.Set(reflect.MakeSlice(src.Type(), src.Len(), src.Cap()))
		for i := 0; i < src.Len(); i += 1 {
			deepCopy(dst.Index(i), src.Index(i), r, f)
		}
	case reflect.Map:
		dst.Set(reflect.MakeMap(src.Type()))
		for _, key := range src.MapKeys() {
			srcValue := src.MapIndex(key)
			dstValue := reflect.New(srcValue.Type()).Elem()
			deepCopy(dstValue, srcValue, r, f)
			dst.SetMapIndex(key, dstValue)
		}
	case reflect.Ptr:
		src = src.Elem()
		if !src.IsValid() {
			return
		}
		dst.Set(reflect.New(src.Type()))
		deepCopy(dst.Elem(), src, r, f)
	case reflect.Interface:
		// Like Pointer but with one more call to Elem.
		src = src.Elem()
		dstIface := reflect.New(src.Type()).Elem()
		deepCopy(dstIface, src, r, f)
		dst.Set(dstIface)
	default:
		dst.Set(src)
	}
}

// ----------------------------------------------------------------------------
// Registry

// CheckRegistry keeps track of all known Checks.
var CheckRegistry map[string]reflect.Type

func init() {
	CheckRegistry = make(map[string]reflect.Type)
	RegisterCheck(StatusCode{})
}

// RegisterCheck registers the check. Once a check is registered it may be
// unmarshaled from its name and marshaled data.
func RegisterCheck(check Check) {
	name := NameOf(check)
	typ := reflect.TypeOf(check)
	if _, ok := CheckRegistry[name]; ok {
		panic(fmt.Sprintf("Check with name %q already registered.", name))
	}
	CheckRegistry[name] = typ
}

// ----------------------------------------------------------------------------
// Errors

var (
	BadBody        = errors.New("skipped due to bad body")
	Failed         = errors.New("failed")
	NotFound       = errors.New("not found")
	FoundForbidden = errors.New("found forbidden")
)

// CantCheck is the error type returned by checks whose preconditions
// are not fulfilled, e.g. malformed HTML or XML.
type CantCheck struct {
	err error
}

func (m CantCheck) Error() string {
	return fmt.Sprintf("cannot do check: %s", m.err.Error())
}

// WrongCount is the error type returned by checks which require a certain
// number of matches.
type WrongCount struct {
	Got, Want int
}

func (m WrongCount) Error() string {
	return fmt.Sprintf("found %d, want %d", m.Got, m.Want)
}

// MalformedCheck is the error type returned by checks who are badly
// parametrized, e.g. whoc try to check against a malformed regualr expression.
type MalformedCheck struct {
	Err error
}

func (m MalformedCheck) Error() string {
	return fmt.Sprintf("malformed check: %s", m.Err.Error())
}

// CheckList is a slice of checks with the sole purpose of
// attaching JSON (un)marshaling methods.
type CheckList []Check

// TODO: handle errors
func (cl CheckList) MarshalJSON() ([]byte, error) {
	buf := &bytes.Buffer{}
	buf.WriteRune('[')
	for i, check := range cl {
		raw, err := json5.Marshal(check)
		if err != nil {
			return nil, err
		}
		buf.WriteString(`{"Check":"` + NameOf(check) + `"`)
		if string(raw) != "{}" {
			buf.WriteRune(',')
			buf.Write(raw[1 : len(raw)-1])
		}
		buf.WriteRune('}')
		if i < len(cl)-1 {
			buf.WriteString(", ")
		}
	}
	buf.WriteRune(']')
	result := string(buf.Bytes())

	return []byte(result), nil
}

func (cl *CheckList) UnmarshalJSON(data []byte) error {
	raw := []json5.RawMessage{}
	err := json5.Unmarshal(data, &raw)
	if err != nil {
		return err
	}
	for _, c := range raw {
		u := struct{ Check string }{}
		err = json5.Unmarshal(c, &u)
		if err != nil {
			return err
		}
		typ, ok := CheckRegistry[u.Check]
		if !ok {
			return fmt.Errorf("no such check %s", u.Check)
		}
		check := reflect.New(typ)
		err = json5.Unmarshal(c, check.Interface())
		if err != nil {
			return err
		}
		*cl = append(*cl, check.Interface().(Check))
	}
	return nil
}

// ----------------------------------------------------------------------------
// Condition

type Condition int

const (
	Equal Condition = iota
	HasPrefix
	HasSuffix
	Match
	LessThan
	GreaterThan
	Present
	Absent
)

var conditionNames = []string{"Equal", "HasPrefix", "HasSuffix", "Match", "Less", "Greater",
	"Present", "Absent"}

func (c Condition) String() string {
	return conditionNames[c]
}
func (c Condition) MarshalText() ([]byte, error) {
	if c < 0 || c > Absent {
		return []byte(""), fmt.Errorf("No such Condition %d.", c)
	}
	return []byte(c.String()), nil
}
func (c *Condition) UnmarshalText(text []byte) error {
	cond := string(text)
	for i, s := range conditionNames {
		if s == cond {
			*c = Condition(i)
			return nil
		}
	}
	return fmt.Errorf("ht: unknow condition %q", cond)
}
