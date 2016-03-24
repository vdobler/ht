// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"reflect"

	"github.com/vdobler/ht/internal/json5"
)

// Check is a single check performed on a Response.
type Check interface {
	// Prepare is called to prepare the check, e.g. to compile
	// regular expressions or that like.
	Prepare() error

	// Execute executes the check.
	Execute(*Test) error
}

// NameOf returns the name of the type of inst.
func NameOf(inst interface{}) string {
	typ := reflect.TypeOf(inst)
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
		for i := 0; i < src.NumField(); i++ {
			deepCopy(dst.Field(i), src.Field(i), r, f)
		}
	case reflect.Slice:
		dst.Set(reflect.MakeSlice(src.Type(), src.Len(), src.Cap()))
		for i := 0; i < src.Len(); i++ {
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
// Check Registry

// CheckRegistry keeps track of all known Checks.
var CheckRegistry = make(map[string]reflect.Type)

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
	// ErrBadBody is returned from checks if the request body is
	// not available (e.g. due to a failed request).
	ErrBadBody = errors.New("skipped due to bad body")

	// ErrNotFound is returned by checks if some expected value was
	// not found.
	ErrNotFound = errors.New("not found")

	// ErrFoundForbidden is returned by checks if a forbidden value
	// is found.
	ErrFoundForbidden = errors.New("found forbidden")

	// ErrFailed is returned by a checks failing unspecificly.
	ErrFailed = errors.New("failed")
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
// parametrized, e.g. who try to check against a malformed regular expression.
type MalformedCheck struct {
	Err error
}

func (m MalformedCheck) Error() string {
	return fmt.Sprintf("malformed check: %s", m.Err.Error())
}

// ----------------------------------------------------------------------------
// CheckList

// CheckList is a slice of checks with the sole purpose of
// attaching JSON (un)marshaling methods.
type CheckList []Check

// MarshalJSON5 produces a JSON5 arry of the checks in cl.
// Each check is serialized in the form
//     { Check: "NameOfCheckAsRegistered", Field1OfCheck: Value1, Field2: Value2, ... }
func (cl CheckList) MarshalJSON5() ([]byte, error) {
	buf := &bytes.Buffer{}
	buf.WriteRune('[')
	for i, check := range cl {
		raw, err := json5.Marshal(check)
		if err != nil {
			return nil, err
		}
		buf.WriteString(`{Check:"`)
		buf.WriteString(NameOf(check))
		buf.WriteByte('"')
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

// extractSingleFieldFromJSON5 called with fieldname="Check" and data of
// "{Check: "StatusCode", Expect: 200}"  will return fieldval="StatusCode"
// and recoded="{Expect: 200}".
func extractSingleFieldFromJSON5(fieldname string, data []byte) (fieldval string, reencoded []byte, err error) {
	rawMap := map[string]*json5.RawMessage{}
	err = json5.Unmarshal(data, &rawMap)
	if err != nil {
		return "", nil, err
	}

	v, ok := rawMap[fieldname]
	if !ok {
		return "", nil, fmt.Errorf("ht: missing %s field in %q", fieldname, data)
	}

	fieldval = string(*v)
	if strings.HasPrefix(fieldval, `"`) {
		fieldval = fieldval[1 : len(fieldval)-1]
	}

	delete(rawMap, fieldname)
	reencoded, err = json5.Marshal(rawMap)
	if err != nil {
		return "", nil, fmt.Errorf("ht: re-marshaling error %q should not happen on %#v", err.Error(), rawMap)
	}

	return fieldval, reencoded, nil

}

// UnmarshalJSON unmarshals data to a slice of Checks.
func (cl *CheckList) UnmarshalJSON(data []byte) error {
	raw := []json5.RawMessage{}
	err := json5.Unmarshal(data, &raw)
	if err != nil {
		return err
	}
	for i, c := range raw {
		checkName, checkDef, err := extractSingleFieldFromJSON5("Check", c)
		if err != nil {
			return err
		}
		typ, ok := CheckRegistry[checkName]
		if !ok {
			return fmt.Errorf("ht: no such check %s", checkName)
		}
		if typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
		}
		rcheck := reflect.New(typ)
		err = json5.Unmarshal(checkDef, rcheck.Interface())
		if err != nil {
			return fmt.Errorf("%d. check: %s", i+1, err)
		}

		*cl = append(*cl, rcheck.Interface().(Check))
	}
	return nil
}
