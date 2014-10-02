// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package check provides useful checks for ht.
package check

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"reflect"

	"github.com/vdobler/ht/response"
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

func NameOf(check Check) string {
	typ := reflect.TypeOf(check)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	return typ.Name()
}

// CheckRegistry keeps track of all known Checks.
var CheckRegistry map[string]reflect.Type

func init() {
	CheckRegistry = make(map[string]reflect.Type)
	RegisterCheck(StatusCode{})
}

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
		raw, err := json.Marshal(check)
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
	raw := []json.RawMessage{}
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return err
	}
	for _, c := range raw {
		u := struct{ Check string }{}
		err = json.Unmarshal(c, &u)
		if err != nil {
			return err
		}
		typ, ok := CheckRegistry[u.Check]
		if !ok {
			return fmt.Errorf("no such check %s", u.Check)
		}
		check := reflect.New(typ)
		err = json.Unmarshal(c, check.Interface())
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
