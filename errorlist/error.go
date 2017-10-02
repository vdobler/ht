// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package error contains a type to collect errors.
package errorlist

import (
	"fmt"
	"os"
	"strings"
)

// List is a collection of errors.
type List []error

// Append err to el.
func (el List) Append(err error) List {
	if err == nil {
		return el
	}
	if list, ok := err.(List); ok {
		return append(el, list...)
	}
	return append(el, err)
}

// Error implements the Error method of error.
func (el List) Error() string {
	return strings.Join(el.AsStrings(), "; \u2029")
}

// AsError returns el properly returning nil for a empty el.
func (el List) AsError() error {
	if len(el) == 0 {
		return nil
	}
	return el
}

// AsStrings returns the error list as as string slice.
func (el List) AsStrings() []string {
	s := []string{}
	for _, e := range el {
		if nel, ok := e.(List); ok {
			s = append(s, nel.AsStrings()...)
		} else {
			s = append(s, e.Error())
		}
	}
	return s
}

// PrintlnStderr prints err to stderr. If err is a List it prints
// several lines.
func PrintlnStderr(err error) {
	if err == nil {
		return
	}
	if el, ok := err.(List); ok {
		for _, msg := range el.AsStrings() {
			fmt.Fprintln(os.Stderr, msg)
		}
	} else {
		fmt.Fprintln(os.Stderr, err.Error())
	}
}
