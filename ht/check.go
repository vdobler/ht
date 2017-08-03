// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/vdobler/ht/populate"
)

// Check is a single check performed on a Response.
type Check interface {
	// Execute executes the check.
	Execute(*Test) error
}

// Preparable is the type a Check may implement to signal that it needs some
// preparation work to be done before the HTTP request is made.
type Preparable interface {
	// Prepare is called to prepare the check, e.g. to compile
	// regular expressions or that like.
	Prepare(*Test) error
}

// NameOf returns the name of the type of inst.
func NameOf(inst interface{}) string {
	typ := reflect.TypeOf(inst)
	if typ == nil {
		return "<nil>"
	}
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	return typ.Name()
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

// ErrBadBody is returned from checks if the request body is
// not available (e.g. due to a failed request).
var ErrBadBody = errors.New("skipped due to bad body")

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

// MarshalJSON produces a JSON arry of the checks in cl.
// Each check is serialized in the form
//     { "Check": "NameOfCheckAsRegistered",
//         "Field1OfCheck": "Value1", "Field2": "Value2", ... }
func (cl CheckList) MarshalJSON() ([]byte, error) {
	buf := &bytes.Buffer{}
	buf.WriteRune('[')
	for i, check := range cl {
		raw, err := json.Marshal(check)
		if err != nil {
			return nil, err
		}
		buf.WriteString(`{"Check":"`)
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
	result := buf.String()

	return []byte(result), nil
}

// Populate implements populate.Populator.Populate.
func (cl *CheckList) Populate(src interface{}) error {
	types := []struct {
		Check string
	}{}

	err := populate.Lax(&types, src)
	if err != nil {
		return fmt.Errorf("ht: cannot determine type of check: %s", err)
	}

	raw := make([]map[string]interface{}, len(types))
	srcList, ok := src.([]interface{})
	if !ok {
		return fmt.Errorf("ht: unable to construct list of checks, cannot deserialise %T", src)
	}

	for i := range raw {
		r, ok := srcList[i].(map[string]interface{})
		if !ok {
			return fmt.Errorf("ht: unable to construct check, cannot deserialise %T", src)
		}
		delete(r, "Check")
		raw[i] = r
	}

	list := make(CheckList, len(types))
	for i, t := range types {
		checkName := t.Check
		typ, ok := CheckRegistry[checkName]
		if !ok {
			return noSuchCheckError(checkName)
		}
		if typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
		}
		rcheck := reflect.New(typ)
		err = populate.Strict(rcheck.Interface(), raw[i])
		if err != nil {
			return fmt.Errorf("ht: problems constructing check %d %s: %s",
				i+1, checkName, err)
		}
		list[i] = rcheck.Interface().(Check)
	}
	*cl = list
	return nil
}

// ----------------------------------------------------------------------------
// Handling misspelled checks

func noSuchCheckError(name string) error {
	checkNames := make([]string, 0, len(CheckRegistry))
	for cn := range CheckRegistry {
		checkNames = append(checkNames, cn)
	}
	if suggestions := possibleNames(name, checkNames); len(suggestions) > 0 {
		return fmt.Errorf("ht: no such check %s (did you mean %s?)", name,
			strings.Join(suggestions, ", "))
	}
	return fmt.Errorf("ht: no such check %s", name)
}

// possibleNames returns a list of actual existing names from valid which
// are similar to orig. "Similar" in the sense of
// https://en.wikipedia.org/wiki/Damerau–Levenshtein_distance of the
// upper cased names being at most 2.
// 3 Seems large but the check names are pretty different.
func possibleNames(orig string, valid []string) []string {
	ORIG := strings.ToUpper(orig)
	suggestions := []string{}
	for _, name := range valid {
		NAME := strings.ToUpper(name)
		if damerauLevenshtein(NAME, ORIG) <= 2 {
			suggestions = append(suggestions, name)
		}
	}
	sort.Strings(suggestions)

	return suggestions
}

// https://en.wikipedia.org/wiki/Damerau%E2%80%93Levenshtein_distance#Distance_with_adjacent_transpositions
func damerauLevenshtein(s1, s2 string) int {
	a, b := []rune(s1), []rune(s2)
	maxdist := len(a) + len(b)

	// DL("", x) == DL(x, "") == len(x)
	if len(a) == 0 {
		return len(b)
	} else if len(b) == 0 {
		return len(a)
	}

	d := make([][]int, len(a)+2)
	for i := range d {
		d[i] = make([]int, len(b)+2)
	}
	seen := make(map[rune]int)

	d[0][0] = maxdist
	for i := 0; i <= len(a); i++ {
		d[i+1][0] = maxdist
		d[i+1][1] = i
	}
	for j := 0; j <= len(b); j++ {
		d[0][j+1] = maxdist
		d[1][j+1] = j
	}

	for i := 1; i <= len(a); i++ {
		db := 0
		for j := 1; j <= len(b); j++ {
			k := seen[b[j-1]]
			ℓ := db
			cost := 0
			if a[i-1] == b[j-1] {
				db = j
			} else {
				cost = 1
			}
			d[i+1][j+1] = min4(
				d[i][j]+cost,              // substitution
				d[i+1][j]+1,               // insertion
				d[i][j+1]+1,               // deletion
				d[k][ℓ]+(i-k-1)+1+(j-ℓ-1), // transposition
			)
			seen[a[i-1]] = i
		}
	}

	return d[len(a)+1][len(b)+1]
}

func min3(a, b, c int) int {
	if a < b {
		if c < a {
			return c
		}
		return a
	}

	// b < a   (or equal)
	if c < b {
		return c
	}
	return b
}

func min4(a, b, c, d int) int {
	if a < b {
		return min3(a, c, d)
	}
	return min3(b, c, d)
}
