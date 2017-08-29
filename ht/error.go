// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// error.go contains stuff to combine multiple errors

package ht

import (
	"fmt"
	"strings"
)

// PosError is an error with optinaly attached position information.
type PosError struct {
	Err  error  // Err is the actual error.
	Line int    // Line is the line number counting from 1.
	Col  int    // Col is the column or byte position, also counting from 1!
	File string // Filename is the optional filename.
}

func (e PosError) Error() string {
	s := ""
	if e.File != "" {
		s = e.File + ":"
	}
	if e.Line > 0 {
		s += fmt.Sprintf("Line %d:", e.Line)
	}
	if e.Col > 0 {
		s += fmt.Sprintf("col %d:", e.Col)
	}
	if s != "" {
		s += " "
	}
	s += e.Err.Error()
	return s
}

// ErrorList is a collection of errors.
type ErrorList []error

// Append err to el.
func (el ErrorList) Append(err error) ErrorList {
	if err == nil {
		return el
	}
	if list, ok := err.(ErrorList); ok {
		return append(el, list...)
	}
	return append(el, err)
}

// Error implements the Error method of error.
func (el ErrorList) Error() string {
	return strings.Join(el.AsStrings(), "; \u2029")
}

// AsError returns el properly returning nil for a empty el.
func (el ErrorList) AsError() error {
	if len(el) == 0 {
		return nil
	}
	return el
}

// AsStrings returns the error list as as string slice.
func (el ErrorList) AsStrings() []string {
	s := []string{}
	for _, e := range el {
		if e == nil {
			continue // TODO: this should never happen
		}
		if nel, ok := e.(ErrorList); ok {
			s = append(s, nel.AsStrings()...)
		} else {
			s = append(s, e.Error())
		}
	}
	return s
}
